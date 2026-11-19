package typescript

import (
	"regexp"
	"slices"
)

// hookInstaller matches a script that installs git hooks. `just setup`
// installs the hooks lefthook.yml lists, with its pinned lefthook, so such a
// script is redundant.
func hookInstaller() *regexp.Regexp {
	return regexp.MustCompile(`^(?:typescript-bedrock hooks|husky|lefthook install|simple-git-hooks)(?:\s|$)`)
}

func jestOrVitest() *regexp.Regexp { return regexp.MustCompile(`\b(?:jest|vitest)\b`) }

// shadowing are the script names `bun fmt` and `bun check` would run in
// place of the built-in command.
func shadowing() []string { return []string{"check", nameFmt} }

// migrateScripts rewrites a manifest's scripts onto Bun and puts the
// template's scripts in; the template wins a conflict, with a note.
func migrateScripts(scripts, template *object) (*object, []string) {
	var notes []string

	kept := newObject()

	for _, name := range scripts.keys {
		value := scripts.get(name)
		text, isText := value.(string)

		if isText && hookInstaller().MatchString(text) {
			notes = append(notes, "Removed script "+quote(name)+": "+quote(text)+
				" installs git hooks, which `just setup` does now.")

			continue
		}

		command := value
		if isText {
			command = rewriteScript(text)
		}

		key := name
		if slices.Contains(shadowing(), name) {
			key = name + ":project"
			notes = append(notes, "Renamed script "+quote(name)+" to "+quote(key)+
				": the name shadows `bun "+name+"`.")
		}

		if !deepEqual(command, value) {
			notes = append(notes, "Script "+quote(name)+": "+compact(value)+" → "+compact(command))
		}

		if isText && jestOrVitest().MatchString(text) {
			notes = append(notes, "Script "+quote(name)+" ran Jest or Vitest. "+
				"Move its setup files and other config into `bunfig.toml` `[test]`, such as `preload`.")
		}

		kept.set(key, command)
	}

	tools := template.without(func(key string) bool { return key == nameTest })

	for _, name := range tools.keys {
		command := tools.get(name)
		if kept.has(name) && !deepEqual(kept.get(name), command) {
			notes = append(notes, "Replaced script "+quote(name)+" ("+compact(kept.get(name))+") with "+
				compact(command)+". Move any step it still needs, such as a build, into a `project-"+name+
				"` recipe in `project.just`, which `just "+name+"` runs.")
		}
	}

	if existing := kept.get(nameTest); existing != nil {
		kept.set(nameTest, existing)
	} else {
		kept.set(nameTest, template.get(nameTest))
	}

	return kept.overlay(tools), notes
}
