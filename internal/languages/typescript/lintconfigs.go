package typescript

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// withoutExtends is a config value without its `extends`, which the template
// inlines instead.
func withoutExtends(value any) any {
	if o, ok := value.(*object); ok {
		return o.without(func(key string) bool { return key == nameExtends })
	}

	return value
}

// droppedKeys are the keys whose values the repo's config sets differently
// from the template, which the rewrite replaces.
func droppedKeys(current *object, template, ignoreKey string) ([]string, error) {
	known, err := parseObject(template)
	if err != nil {
		return nil, fmt.Errorf("parse the template: %w", err)
	}

	var dropped []string

	for _, key := range current.keys {
		if key != nameExtends && key != ignoreKey &&
			!deepEqual(withoutExtends(current.get(key)), withoutExtends(known.get(key))) {
			dropped = append(dropped, key)
		}
	}

	return dropped, nil
}

// extendsLegacy reports whether a tool config still names the old
// `typescript-bedrock` package, such as in `extends`.
func extendsLegacy(config *object) bool {
	return config != nil && strings.Contains(compact(config), legacyPackage)
}

func quotedList(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, "`"+quote(name)+"`")
	}

	return strings.Join(quoted, ", ")
}

func ignoreNotes(list ignoreList, current *object, dropped, restored []string) []string {
	var notes []string

	if len(dropped) > 0 || extendsLegacy(current) {
		detail := ""
		if len(dropped) > 0 {
			detail = "; its own " + quotedList(dropped) + " were not carried over, git has them"
		}

		notes = append(notes, "`"+list.path+"`: now the full bedrock config, without `extends`"+detail+".")
	}

	if len(restored) > 0 {
		notes = append(notes, "`"+list.path+"`: puts back "+quotedList(restored)+" in `"+quote(list.key)+
			"`; bedrock owns its own entries, and yours go after them.")
	}

	return notes
}

// planIgnoreLists rewrites each lint config from the template plus the
// repo's own ignore entries.
func planIgnoreLists(c *language.Context) (part, error) {
	var out part

	for _, list := range ignoreKeys() {
		template, _, err := c.Dir("typescript", "files").Read(list.path, nil)
		if err != nil {
			return part{}, fmt.Errorf("read template `%s`: %w", list.path, err)
		}

		current, err := readJSONC(filepath.Join(c.Root, list.path))
		if err != nil {
			return part{}, err
		}

		var dropped []string
		if current != nil {
			if dropped, err = droppedKeys(current, template, list.key); err != nil {
				return part{}, err
			}
		}

		restored, err := droppedEntries(template, current, list.key)
		if err != nil {
			return part{}, err
		}

		merged, err := mergeIgnoreList(template, current, list.key)
		if err != nil {
			return part{}, err
		}

		out.steps = append(out.steps, step.Write{Path: list.path, Content: []byte(merged), Executable: false})
		out.notes = append(out.notes, ignoreNotes(list, current, dropped, restored)...)
	}

	return out, nil
}
