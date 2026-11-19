package swift

import (
	"fmt"
	"slices"
	"strings"
)

// record is a JSON object from `swift package dump-package`.
type record = map[string]any

// asRecord returns the value as an object, or an empty one. The checks stay
// lenient because SwiftPM's dump format changes between releases.
func asRecord(value any) record {
	if object, ok := value.(map[string]any); ok {
		return object
	}

	return record{}
}

func asArray(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}

	return nil
}

func asString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}

	return ""
}

func asStrings(value any) []string {
	var texts []string

	for _, item := range asArray(value) {
		if text, ok := item.(string); ok {
			texts = append(texts, text)
		}
	}

	return texts
}

// strict is what every target's settings must contain, as `setting` or
// `setting:value`.
func strict() []string {
	return []string{
		"enableUpcomingFeature:ExistentialAny",
		"enableUpcomingFeature:InternalImportsByDefault",
		"enableUpcomingFeature:MemberImportVisibility",
		"enableUpcomingFeature:InferIsolatedConformances",
		"enableUpcomingFeature:NonisolatedNonsendingByDefault",
		"enableUpcomingFeature:ImmutableWeakCaptures",
		"strictMemorySafety",
		"treatAllWarnings:error",
	}
}

// settingNames returns the `setting` or `setting:value` names one target's
// swift settings carry.
func settingNames(settings []record) map[string]bool {
	names := map[string]bool{}

	for _, kind := range settings {
		for key, value := range kind {
			if argument, ok := asRecord(value)["_0"].(string); ok {
				names[key+":"+argument] = true
			} else {
				names[key] = true
			}
		}
	}

	return names
}

func targetFindings(target record) []string {
	name := asString(target["name"])
	if name == "" {
		name = "?"
	}

	if slices.Contains([]string{"system", "binary", "plugin"}, asString(target["type"])) {
		return nil
	}

	var settings []record
	for _, setting := range asArray(target["settings"]) {
		settings = append(settings, asRecord(asRecord(setting)["kind"]))
	}

	found := settingNames(settings)

	var problems []string

	missing := slices.DeleteFunc(strict(), func(setting string) bool { return found[setting] })
	if len(missing) > 0 {
		problems = append(
			problems,
			fmt.Sprintf("`Package.swift`: target %s lacks %s; keep the `swift-bedrock` block last",
				name, strings.Join(missing, ", ")),
		)
	}

	if slices.ContainsFunc(settings, func(kind record) bool { _, ok := kind["unsafeFlags"]; return ok }) {
		problems = append(problems, fmt.Sprintf("`Package.swift`: target %s uses `unsafeFlags`, "+
			"which keeps others from depending on the package", name))
	}

	return problems
}

// manifestFindings returns the problems with the package that
// `swift package dump-package` describes.
func manifestFindings(pkg record) []string {
	tools := asString(asRecord(pkg["toolsVersion"])["_version"])
	modes := asStrings(pkg["swiftLanguageVersions"])

	var problems []string

	if !strings.HasPrefix(tools, "6.4") {
		problems = append(problems, fmt.Sprintf("`Package.swift`: swift-tools-version is %s; use 6.4", tools))
	}

	if len(modes) != 1 || modes[0] != "6" {
		problems = append(problems, fmt.Sprintf("`Package.swift`: the language modes are [%s]; use [.v6] alone",
			strings.Join(modes, ", ")))
	}

	for _, target := range asArray(pkg["targets"]) {
		problems = append(problems, targetFindings(asRecord(target))...)
	}

	return problems
}
