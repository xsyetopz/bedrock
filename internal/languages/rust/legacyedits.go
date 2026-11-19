package rust

import (
	"regexp"
	"slices"
	"strings"
)

func isAlias(unit string) bool {
	key, ok := keyOf(unit)

	return ok && (key == "bedrock" || key == "alias.bedrock") && strings.Contains(unit, "xtask")
}

// withoutBedrockAlias returns the config without the `bedrock` alias that ran
// the xtask crate, which older releases wrote.
func withoutBedrockAlias(text string) string {
	sections := parseSections(text)

	if !slices.ContainsFunc(sections, func(entry section) bool { return slices.ContainsFunc(entry.units, isAlias) }) {
		return text
	}

	stripped := make([]section, 0, len(sections))

	for _, entry := range sections {
		units := slices.DeleteFunc(slices.Clone(entry.units), isAlias)
		// An alias table that held only the bedrock alias goes with it.
		if next := withUnits(entry, units); !entry.named() || entry.name != "alias" || !hasNoEntries(next) {
			stripped = append(stripped, next)
		}
	}

	return renderSections(tidy(stripped))
}

// withoutBedrockGit returns `deny.toml` without the `allow-git` entry for the
// bedrock repository, which the old xtask crate needed.
func withoutBedrockGit(text string) string {
	sections := parseSections(text)

	sources, found := findSection(sections, "sources")
	if !found {
		return text
	}

	index := slices.IndexFunc(sources.units, func(unit string) bool {
		key, ok := keyOf(unit)

		return ok && key == "allow-git"
	})
	if index == -1 {
		return text
	}

	doc, _ := parseToml(sources.units[index])
	listed, _ := at(doc, "allow-git").([]any)
	pattern := regexp.MustCompile(`xsyetopz/bedrock(?:\.git)?/?$`)

	rest := make([]any, 0, len(listed))

	for _, entry := range listed {
		if url, isString := entry.(string); !isString || !pattern.MatchString(url) {
			rest = append(rest, entry)
		}
	}

	if len(rest) == len(listed) {
		return text
	}

	before := slices.Clone(sources.units[:index])
	comment := regexp.MustCompile(`^\s*#.*(?:xtask|bedrock)`)

	for len(before) > 0 && comment.MatchString(before[len(before)-1]) {
		before = before[:len(before)-1]
	}

	var replacement []string
	if len(rest) > 0 {
		replacement = []string{"allow-git = " + jsonText(rest) + "\n"}
	}

	units := append(append(before, replacement...), sources.units[index+1:]...)

	return renderSections(tidy(upsertSection(sections, withUnits(sources, units))))
}

func memberLinePattern() *regexp.Regexp {
	return regexp.MustCompile(`^\s*["']xtask["']\s*,?\s*(?:#.*)?\n?$`)
}

// withoutMember returns the `members` entry without `"xtask"`, whether the
// list is inline or one member per line.
func withoutMember(unit string) string {
	lines := splitLines(unit)
	pattern := memberLinePattern()

	kept := slices.DeleteFunc(slices.Clone(lines), pattern.MatchString)
	if len(kept) < len(lines) {
		return strings.Join(kept, "")
	}

	// Each replacement handles a different position in the list: last, first,
	// or only.
	for _, replacement := range []string{`,\s*["']xtask["']`, `["']xtask["']\s*,\s*`, `["']xtask["']`} {
		if location := regexp.MustCompile(replacement).FindStringIndex(unit); location != nil {
			unit = unit[:location[0]] + unit[location[1]:]
		}
	}

	return unit
}

// withoutXtaskMember drops the `"xtask"` workspace member, which older
// releases added.
func withoutXtaskMember(text string) string {
	sections := parseSections(text)

	workspace, found := findSection(sections, "workspace")
	if !found {
		return text
	}

	index := slices.IndexFunc(workspace.units, func(unit string) bool {
		key, ok := keyOf(unit)

		return ok && key == "members"
	})
	if index == -1 {
		return text
	}

	units := slices.Clone(workspace.units)
	units[index] = withoutMember(units[index])

	return renderSections(upsertSection(sections, withUnits(workspace, units)))
}
