package typescript

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// ignoreKeys maps each tool config to the ignore list it keeps, which a
// repo may extend.
func ignoreKeys() []ignoreList {
	return []ignoreList{
		{path: ".markdownlint-cli2.jsonc", key: "ignores"},
		{path: ".oxlintrc.json", key: "ignorePatterns"},
	}
}

type ignoreList struct {
	path string
	key  string
}

func arrayOf(key string) *regexp.Regexp {
	return regexp.MustCompile(`("` + regexp.QuoteMeta(key) + `"\s*:\s*)\[([^\]]*)\]`)
}

// templateEntries are the entries the template lists under key.
func templateEntries(template, key string) ([]string, error) {
	found := arrayOf(key).FindStringSubmatch(template)
	if found == nil {
		return nil, nil
	}

	value, err := parseJSONC("[" + found[2] + "]")
	if err != nil {
		return nil, fmt.Errorf("read `%s` in the template: %w", key, err)
	}

	return asStrings(value), nil
}

// droppedEntries are the template's entries that current, a config the repo
// already has, dropped from its list. Bedrock owns them, so adopt puts them
// back.
func droppedEntries(template string, current *object, key string) ([]string, error) {
	if current == nil {
		return nil, nil
	}

	kept := asStrings(current.get(key))
	entries, err := templateEntries(template, key)

	return slices.DeleteFunc(entries, func(entry string) bool { return slices.Contains(kept, entry) }), err
}

// mergeIgnoreList writes the template's config with the entries current
// lists under key and the template lacks added after the template's own, in
// the template's layout and with its comments.
func mergeIgnoreList(template string, current *object, key string) (string, error) {
	pattern := arrayOf(key)

	found := pattern.FindStringSubmatchIndex(template)
	if found == nil {
		return template, nil
	}

	have, err := templateEntries(template, key)
	if err != nil {
		return "", err
	}

	have = unique(have)

	var extra []string

	if current != nil {
		extra = unique(slices.DeleteFunc(asStrings(current.get(key)), func(entry string) bool {
			return slices.Contains(have, entry)
		}))
	}

	if len(extra) == 0 {
		return template, nil
	}

	items := make([]string, 0, len(have)+len(extra))
	for _, entry := range append(slices.Clone(have), extra...) {
		items = append(items, quote(entry))
	}

	// No trailing comma: JSON rejects one, and `bun fmt` removes it from JSONC.
	body := strings.Join(items, ", ")
	if strings.Contains(template[found[4]:found[5]], "\n") {
		body = "\n\t\t" + strings.Join(items, ",\n\t\t") + "\n\t"
	}

	return template[:found[3]] + "[" + body + "]" + template[found[1]:], nil
}
