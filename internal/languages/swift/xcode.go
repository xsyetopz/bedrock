package swift

import (
	"os"
	"slices"
	"strings"
	"unicode"
)

// xcconfigReference is the object id of the `Bedrock.xcconfig` file
// reference wired into a project.
const xcconfigReference = "B3D20C0000000000000000A1"

const (
	endFileSection = "/* End PBXFileReference section */"
	buildConfigISA = "isa = XCBuildConfiguration;\n"
	childrenOpen   = "children = (\n"
)

// xcodeProjects lists the `.xcodeproj` folders at the repository root,
// sorted.
func xcodeProjects(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var projects []string

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".xcodeproj") {
			projects = append(projects, entry.Name())
		}
	}

	slices.Sort(projects)

	return projects
}

// indexFrom is `strings.Index` that starts at from.
func indexFrom(text, sub string, from int) int {
	if from > len(text) {
		return -1
	}

	found := strings.Index(text[from:], sub)
	if found == -1 {
		return -1
	}

	return from + found
}

// idAt returns the run of letters and digits that starts at from.
func idAt(text string, from int) string {
	rest := text[from:]

	for i, c := range rest {
		if !unicode.IsLetter(c) && !unicode.IsNumber(c) {
			return rest[:i]
		}
	}

	return rest
}

// valueOf returns the object id after `key = ` past after.
func valueOf(key, text string, after int) (string, bool) {
	start := indexFrom(text, key+" = ", after)
	if start == -1 {
		return "", false
	}

	return idAt(text, start+len(key+" = ")), true
}

// objectAt returns the offset just after the tab-indented object id.
func objectAt(id, text string) (int, bool) {
	start := strings.Index(text, "\t\t"+id+" ")
	if start == -1 {
		return 0, false
	}

	return start + len("\t\t"+id+" "), true
}

// childrenOf returns the object ids listed under `key = (` past after.
func childrenOf(key, text string, after int) ([]string, bool) {
	start := indexFrom(text, key+" = (\n", after)
	if start == -1 {
		return nil, false
	}

	end := indexFrom(text, ");", start)
	if end == -1 {
		return nil, false
	}

	var ids []string

	for line := range strings.SplitSeq(text[start+len(key)+5:end], "\n") {
		if id := idAt(strings.TrimSpace(line), 0); id != "" {
			ids = append(ids, id)
		}
	}

	return ids, true
}

func insertAt(text string, index int, insert string) string {
	return text[:index] + insert + text[index:]
}

// wireConfiguration gives one build configuration the xcconfig as its base,
// or reports false when it cannot.
func wireConfiguration(text, id string) (string, bool) {
	start, ok := objectAt(id, text)
	if !ok {
		return "", false
	}

	isa := indexFrom(text, buildConfigISA, start)
	if isa == -1 {
		return "", false
	}

	body := isa + len(buildConfigISA)

	settings := indexFrom(text, "buildSettings", body)
	if settings == -1 || strings.Contains(text[body:settings], "baseConfigurationReference") {
		return "", false
	}

	return insertAt(text, body,
		"\t\t\tbaseConfigurationReference = "+xcconfigReference+" /* Bedrock.xcconfig */;\n"), true
}

// addReference adds the xcconfig file reference to the main group and to the
// file section.
func addReference(text, mainGroup string) (string, bool) {
	group, ok := objectAt(mainGroup, text)
	if !ok {
		return "", false
	}

	children := indexFrom(text, childrenOpen, group)
	if children == -1 || !strings.Contains(text, endFileSection) {
		return "", false
	}

	withGroup := insertAt(text, children+len(childrenOpen),
		"\t\t\t\t"+xcconfigReference+" /* Bedrock.xcconfig */,\n")

	return insertAt(withGroup, strings.Index(withGroup, endFileSection),
		"\t\t"+xcconfigReference+" /* Bedrock.xcconfig */ = {isa = PBXFileReference; "+
			"lastKnownFileType = text.xcconfig; name = Bedrock.xcconfig; "+
			`path = Config/Bedrock.xcconfig; sourceTree = "<group>"; };`+"\n"), true
}

// wired returns pbxproj with Bedrock.xcconfig as the base configuration of
// each project-level build configuration, or false when it already is or
// another file is.
func wired(pbxproj string) (string, bool) {
	const marker = "isa = PBXProject;"

	project := strings.Index(pbxproj, marker)
	if project == -1 || strings.Contains(pbxproj, "Bedrock.xcconfig") {
		return "", false
	}

	after := project + len(marker)

	list, hasList := valueOf("buildConfigurationList", pbxproj, after)
	mainGroup, hasGroup := valueOf("mainGroup", pbxproj, after)

	listObject, hasObject := objectAt(list, pbxproj)
	if !hasList || !hasGroup || !hasObject {
		return "", false
	}

	configurations, ok := childrenOf("buildConfigurations", pbxproj, listObject)
	if !ok {
		return "", false
	}

	text := pbxproj

	for _, id := range configurations {
		if text, ok = wireConfiguration(text, id); !ok {
			return "", false
		}
	}

	return addReference(text, mainGroup)
}

// overrides returns the build settings in pbxproj that override what
// `Bedrock.xcconfig` sets.
func overrides(pbxproj string) []string {
	var found []string

	for line := range strings.SplitSeq(pbxproj, "\n") {
		setting := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(setting, "SWIFT_VERSION = ") && !strings.HasPrefix(setting, "SWIFT_VERSION = 6"):
			found = append(found, "`"+setting+"` keeps the target out of Swift 6 mode")
		case strings.HasPrefix(setting, "OTHER_SWIFT_FLAGS = ") && !strings.Contains(setting, "$(inherited)"):
			found = append(found, "`OTHER_SWIFT_FLAGS` without `$(inherited)` drops the upcoming features")
		}
	}

	return found
}
