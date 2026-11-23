package jvm

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xsyetopz/bedrock/internal/blocks"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// blockName is the block jvm owns in settings.gradle.kts and
// gradle.properties.
const blockName = "jvm-bedrock"

// legacyFiles are what older bedrock releases wrote: files under
// `gradle/bedrock`, which core's drift check replaced, and the wrapper jar as
// base64, which bedrock now writes as the jar itself.
func legacyFiles() []string {
	return []string{
		"gradle/bedrock/managed.sha256",
		"gradle/bedrock/src/main/kotlin/bedrock/core/Guard.kt",
		"gradle/wrapper/gradle-wrapper.jar.b64",
	}
}

func rootName() *regexp.Regexp { return regexp.MustCompile(`rootProject\.name\s*=\s*['"]([^'"]+)['"]`) }

// groupOption returns the group option, or "" when none is set.
func groupOption(c *language.Context) string { return c.Options["group"] }

// blockFirst returns current with the block first: `plugins` and
// `pluginManagement` must precede the rest of a settings file.
func blockFirst(current string, found bool, header, content string) string {
	order := []string{blockName}
	if found && blocks.Has(current, blocks.Slash, blockName) {
		return blocks.Upsert(current, blocks.Slash, order, blocks.Block{Name: blockName, Content: content})
	}

	marked := blocks.Upsert("", blocks.Slash, order, blocks.Block{Name: blockName, Content: content})
	if !found {
		current = header
	}

	return marked + "\n" + current
}

func includeLine(modules []string) string {
	if len(modules) == 0 {
		return ""
	}

	quotedModules := make([]string, len(modules))
	for index, module := range modules {
		quotedModules[index] = `":` + module + `"`
	}

	return "include(" + strings.Join(quotedModules, ", ") + ")\n"
}

func settingsStep(c *language.Context, m *migration, modules []string) (step.Step, error) {
	kts, ktsFound, err := fsutil.ReadText(filepath.Join(c.Root, fileSettingsGradleKts))
	if err != nil {
		return nil, err
	}

	groovy, _, err := fsutil.ReadText(filepath.Join(c.Root, fileSettingsGradle))
	if err != nil {
		return nil, err
	}

	block, _, err := c.Dir("jvm", "blocks").Read(fileSettingsGradleKts, template.Variables{})
	if err != nil {
		return nil, err
	}

	name := filepath.Base(c.Root)
	if match := rootName().FindStringSubmatch(kts + groovy); match != nil {
		name = match[1]
	}

	// A Gradle build's own settings would apply plugins before bedrock's, so
	// they start over.
	if m != nil && m.tool == "Gradle" {
		kts, ktsFound = "", false
	}

	settings := blockFirst(kts, ktsFound, `rootProject.name = "`+name+`"`+"\n", block)

	lines := includeLine(modules)
	if m != nil {
		lines = includes(m)
	}

	if lines != "" && !strings.Contains(settings, lines) {
		settings += lines
	}

	return step.Write{Path: fileSettingsGradleKts, Content: []byte(settings), Executable: false}, nil
}

func declaredGroup() *regexp.Regexp { return regexp.MustCompile(`(?m)^group\s*=\s*(.*?)\s*$`) }

// propertiesStep writes `gradle.properties` with the block and the group, and
// returns the notes about the group.
func propertiesStep(c *language.Context, m *migration, fallback string) (step.Step, []string, error) {
	current, _, err := fsutil.ReadText(filepath.Join(c.Root, fileGradleProperties))
	if err != nil {
		return nil, nil, err
	}

	block, _, err := c.Dir("jvm", "blocks").Read(fileGradleProperties, template.Variables{})
	if err != nil {
		return nil, nil, err
	}

	text := blocks.Upsert(current, blocks.Hash, []string{blockName}, blocks.Block{Name: blockName, Content: block})

	var notes []string

	option := groupOption(c)

	declared := declaredGroup().FindStringSubmatch(text)
	switch {
	case declared == nil:
		chosen := option

		if chosen == "" && m != nil {
			chosen = m.group
		}

		if chosen == "" {
			chosen = fallback
		}

		if chosen == "" {
			notes = append(notes, "`gradle.properties`: set `group=` to the package the code lives in; "+
				"NullAway checks it and Android derives namespaces from it")
		} else {
			text += "group=" + chosen + "\n"
		}
	case option != "" && declared[1] != option:
		// The group option, kept in .editorconfig as bedrock_jvm_group, is what
		// the user chose last.
		text = strings.Replace(text, declared[0], "group="+option, 1)
		notes = append(notes, "`gradle.properties`: `group` "+declared[1]+" becomes "+option+
			", `bedrock_jvm_group` in `.editorconfig`; NullAway and the Android namespaces follow it.")
	}

	return step.Write{Path: fileGradleProperties, Content: []byte(text), Executable: false}, notes, nil
}

// writtenPaths returns the paths the write steps change.
func writtenPaths(steps ...[]step.Step) []string {
	var paths []string

	for _, list := range steps {
		for _, each := range list {
			if written, ok := each.(step.Write); ok {
				paths = append(paths, written.Path)
			}
		}
	}

	return paths
}

// Plan plans the jvm part of an adopt: build tool migration, settings,
// properties, kinds, and legacy removals.
func (JVM) Plan(_ context.Context, c *language.Context) (language.Contribution, error) {
	converted, err := migrationFor(c.Root)
	if err != nil {
		return language.Contribution{}, err
	}

	name := filepath.Base(c.Root)
	fallback := "com.example." + packageName(name)

	group := groupOption(c)
	if group == "" {
		group = fallback
	}

	vars := template.Variables{"name": name, "package": group, "path": strings.ReplaceAll(group, ".", "/")}

	var kinds kindPlan

	if c.Init {
		kinds, err = kindSteps(c, vars)
		if err != nil {
			return language.Contribution{}, err
		}
	}

	kindGroup := ""
	if len(kinds.steps) > 0 {
		kindGroup = fallback
	}

	settings, err := settingsStep(c, converted, kinds.modules)
	if err != nil {
		return language.Contribution{}, err
	}

	properties, notes, err := propertiesStep(c, converted, kindGroup)
	if err != nil {
		return language.Contribution{}, err
	}

	var migrated []step.Step
	if converted != nil {
		migrated, err = render(c.Root, converted)
		if err != nil {
			return language.Contribution{}, err
		}

		notes = append(notes, converted.notes.list...)
	}

	steps := []step.Step{settings, properties}
	steps = append(steps, migrated...)
	steps = append(steps, kinds.steps...)

	for _, legacy := range fsutil.Existing(c.Root, legacyFiles()) {
		steps = append(steps, step.Remove{Path: legacy})
	}

	unused, err := unusedConventions(c, steps)
	if err != nil {
		return language.Contribution{}, err
	}

	return language.Contribution{
		Steps: steps,
		Notes: notes,
		ProjectFiles: append(
			[]string{fileSettingsGradleKts, fileGradleProperties},
			writtenPaths(migrated, kinds.steps)...),
		Unused: unused,
	}, nil
}
