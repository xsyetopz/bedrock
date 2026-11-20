package swift

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/xsyetopz/bedrock/internal/blocks"
)

// toolsVersion is the tools version every adopted manifest declares.
const toolsVersion = "// swift-tools-version: 6.4"

// withToolsVersion replaces the first line when it is a tools-version
// comment, else puts one first.
func withToolsVersion(text string) string {
	lines := strings.Split(text, "\n")
	first := strings.ToLower(strings.Join(strings.Fields(lines[0]), ""))

	if strings.HasPrefix(first, "//swift-tools-version") {
		lines[0] = toolsVersion
	} else {
		lines = append([]string{toolsVersion}, lines...)
	}

	return strings.Join(lines, "\n")
}

func oldMacOS() *regexp.Regexp {
	return regexp.MustCompile(`\.macOS\(\s*(?:\.v10_\d+|\.v1[12]|"(?:10\.\d+|1[12])(?:\.\d+)*")\s*\)`)
}

// withMinimumMacOS raises a macOS target below 13, the oldest the templates
// build for, to `.v13`.
func withMinimumMacOS(text string) string {
	return oldMacOS().ReplaceAllString(text, ".macOS(.v13)")
}

// manifestOrder lists the manifest blocks in file order.
func manifestOrder() []string { return []string{swiftBlock, cxxBlock} }

// adoptedManifest returns the manifest with the tools version, the macOS
// floor, and the strict blocks.
func adoptedManifest(text string, managed []blocks.Block) string {
	return blocks.Upsert(withMinimumMacOS(withToolsVersion(text)), blocks.Slash, manifestOrder(), managed...)
}

// typeName turns raw, such as a directory name, into a Swift type name:
// `my-tool` becomes `MyTool`.
func typeName(raw string) string {
	words := strings.FieldsFunc(raw, func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsNumber(c) })

	var name strings.Builder

	for _, word := range words {
		runes := []rune(word)
		name.WriteString(strings.ToUpper(string(runes[:1])))
		name.WriteString(string(runes[1:]))
	}

	return name.String()
}

// isTypeName reports whether name starts with a letter and holds only ASCII
// letters and digits.
func isTypeName(name string) bool {
	for i, c := range name {
		ascii := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !ascii || (i == 0 && !unicode.IsLetter(c)) {
			return false
		}
	}

	return name != ""
}
