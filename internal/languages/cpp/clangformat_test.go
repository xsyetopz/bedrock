package cpp_test

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

// schemaProperty is one property of the clang-format JSON schema.
type schemaProperty struct {
	Properties map[string]schemaProperty `json:"properties"`
	Type       string                    `json:"type"`
	Enum       []any                     `json:"enum"`
}

// formatEntry is one key of a `.clang-format`, under parent when nested.
type formatEntry struct {
	parent string
	key    string
	value  string
}

// formatEntries returns the keys and scalar values of a flat `.clang-format`,
// one level of nesting at most.
func formatEntries(text string) []formatEntry {
	var (
		entries []formatEntry
		parent  string
	)

	for line := range strings.SplitSeq(text, "\n") {
		colon := strings.Index(line, ":")
		if colon < 0 || strings.HasPrefix(line, "#") {
			continue
		}

		nested := strings.HasPrefix(line, " ")
		key := strings.TrimSpace(line[:colon])
		value := strings.Trim(strings.TrimSpace(line[colon+1:]), `"`)

		entry := formatEntry{parent: "", key: key, value: value}
		if nested {
			entry.parent = parent
		} else {
			parent = ""
			if value == "" {
				parent = key
			}
		}

		entries = append(entries, entry)
	}

	return entries
}

// problem returns what is wrong with entry against the schema's properties.
func problem(properties map[string]schemaProperty, entry formatEntry) string {
	property, ok := properties[entry.key]
	if !ok {
		return entry.key + " is not in the schema"
	}

	var allowed []string

	for _, item := range property.Enum {
		if text, isText := item.(string); isText {
			allowed = append(allowed, text)
		}
	}

	if len(allowed) > 0 {
		if !slices.Contains(allowed, entry.value) {
			return fmt.Sprintf("%s: %s is not in %s", entry.key, entry.value, strings.Join(allowed, ", "))
		}

		return ""
	}

	switch property.Type {
	case "boolean":
		if entry.value != "true" && entry.value != "false" {
			return entry.key + ": " + entry.value + " is not a boolean"
		}
	case "integer":
		if _, err := strconv.Atoi(entry.value); err != nil {
			return entry.key + ": " + entry.value + " is not an integer"
		}
	}

	return ""
}

func TestClangFormatTemplateMatchesTheClangFormat21Schema(t *testing.T) {
	t.Parallel()

	// Arrange
	raw, err := os.ReadFile("testdata/clang-format-21.x.json")
	if err != nil {
		t.Fatal(err)
	}

	var schema schemaProperty
	if err = json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}

	root := bedrocktest.Repo(t, map[string]string{"CMakeLists.txt": "project(app C)\n"})

	// Act
	template := bedrocktest.Written(bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "cpp", nil)).Steps,
		".clang-format")

	// Assert
	if template == "" {
		t.Fatal("adopt writes no `.clang-format`")
	}

	for _, entry := range formatEntries(template) {
		if entry.value == "" {
			continue
		}

		properties := schema.Properties
		if entry.parent != "" {
			properties = schema.Properties[entry.parent].Properties
		}

		if message := problem(properties, entry); message != "" {
			t.Errorf("`.clang-format`: %s", message)
		}
	}
}
