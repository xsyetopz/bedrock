package jvm

import (
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// unread is a build tool adopt finds but cannot read.
type unread struct {
	file string
	tool string
}

func unreadTools() []unread {
	return []unread{
		{file: "build.sc", tool: "Mill"},
		{file: "build.mill", tool: "Mill"},
		{file: "MODULE.bazel", tool: "Bazel"},
		{file: "WORKSPACE", tool: "Bazel"},
	}
}

// migrationOf converts the build tool in root; pom.xml beats build.sbt, which
// beats Ant, which beats Gradle. It returns nil when there is none.
func migrationOf(root string) (*migration, error) {
	switch {
	case fsutil.Exists(filepath.Join(root, "pom.xml")):
		return maven(root)
	case fsutil.Exists(filepath.Join(root, "build.sbt")):
		return sbt(root)
	case fsutil.Exists(filepath.Join(root, "build.xml")), fsutil.Exists(filepath.Join(root, "ivy.xml")):
		return ant(root)
	}

	gradleBuild, err := isGradle(root)
	if err != nil || !gradleBuild {
		return nil, err
	}

	return gradle(root)
}

// migrationFor returns the migration for the build tool in root, or nil when
// it has none. Scala and Groovy sources and builds adopt cannot read become
// notes, since the rest of the repo still adopts.
func migrationFor(root string) (*migration, error) {
	converted, err := migrationOf(root)
	if err != nil {
		return nil, err
	}

	var notes []string

	if foreign := foreignSources(root, ""); len(foreign) > 0 {
		quotedPaths := make([]string, len(foreign))
		for index, source := range foreign {
			quotedPaths[index] = "`" + source + "`"
		}

		notes = append(notes, strings.Join(quotedPaths, ", ")+
			": bedrock compiles Java and Kotlin, not Scala or Groovy; port those sources first")
	}

	tool := "Scala"
	found := false

	for _, candidate := range unreadTools() {
		if !fsutil.Exists(filepath.Join(root, candidate.file)) {
			continue
		}

		if !found {
			tool, found = candidate.tool, true
		}

		notes = append(notes, "`"+candidate.file+"`: adopt cannot read "+candidate.tool+
			" builds; write each module's `build.gradle.kts` by hand")
	}

	if len(notes) == 0 {
		return converted, nil
	}

	if converted == nil {
		converted = newMigration(tool)
	}

	for _, note := range notes {
		converted.notes.add(note)
	}

	return converted, nil
}
