package swift

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/languages/swift/xctest"
	"github.com/xsyetopz/bedrock/internal/step"
)

// dumpFindings checks the manifest SwiftPM resolves, not the text of it.
func dumpFindings(ctx context.Context, c *language.Context) []string {
	if !c.Runner.LookPath(swiftTool) {
		return []string{"`Package.swift`: `swift` is missing, so the manifest was not checked; " +
			"install it with swiftly or Xcode"}
	}

	// SwiftPM would nest its sandbox inside an agent's or hook's, and that fails.
	dumped, err := runSwift(ctx, c.Runner, c.Root, "package", "--disable-sandbox", "dump-package")
	if err != nil {
		return []string{"`Package.swift`: `swift package dump-package` failed: " + err.Error()}
	}

	if dumped.Code != 0 {
		return []string{"`Package.swift`: `swift package dump-package` failed: " + firstLine(string(dumped.Stderr))}
	}

	var pkg record
	if json.Unmarshal(dumped.Stdout, &pkg) != nil || pkg == nil {
		return []string{"`Package.swift`: `swift package dump-package` printed no package"}
	}

	return manifestFindings(pkg)
}

// fileFindings reports XCTest outside UI tests and file-wide suppressions
// in one file. With fix, it converts XCTest first.
func fileFindings(c *language.Context, file string, fix bool) ([]string, error) {
	text, found, err := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
	if err != nil || !found {
		return nil, err
	}

	var problems []string

	if !isUITest(file) && importsXCTest(text) {
		if converted, ok := xctest.Convert(text); fix && ok {
			write := step.Write{Path: file, Content: []byte(converted.Text), Executable: false}
			if err = step.Apply(c.Root, []step.Step{write}); err != nil {
				return nil, err
			}

			text = converted.Text

			for _, note := range converted.Notes {
				problems = append(problems, "`"+file+"`: "+note)
			}
		}

		if importsXCTest(text) {
			problems = append(problems, "`"+file+"`: imports `XCTest` outside a `*UITests` target; "+
				"use Swift Testing (`just lint-fix` converts it)")
		}
	}

	for _, note := range suppressions(text) {
		problems = append(problems, "`"+file+"`:"+note)
	}

	return problems, nil
}

// foreignFiles are dependency files of other tools, which `Package.swift`
// replaces.
func foreignFiles() [][2]string {
	return [][2]string{
		{"Cartfile", "Carthage: move each framework to a SwiftPM dependency in `Package.swift`"},
		{"Podfile", "CocoaPods: move each pod to a SwiftPM dependency in `Package.swift`"},
		{".swiftformat", "SwiftFormat: `swift-format` and `.swift-format` replaced it"},
	}
}

func foreignFindings(root string) []string {
	var problems []string

	for _, foreign := range foreignFiles() {
		if fsutil.Exists(filepath.Join(root, foreign[0])) {
			problems = append(problems, foreign[0]+": "+foreign[1])
		}
	}

	return problems
}

// projectFindings reports Xcode projects that skip `Config/Bedrock.xcconfig`
// or override what it sets.
func projectFindings(root string) ([]string, error) {
	var problems []string

	for _, project := range xcodeProjects(root) {
		file := project + "/project.pbxproj"

		text, found, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return nil, err
		}

		if !found {
			continue
		}

		if !strings.Contains(text, "Bedrock.xcconfig") {
			problems = append(problems, "`"+file+"`: set `Config/Bedrock.xcconfig` as the base "+
				"configuration of the project's Debug and Release")
		}

		notes := overrides(text)
		slices.Sort(notes)

		for _, note := range slices.Compact(notes) {
			problems = append(problems, "`"+file+"`: "+note+"; remove it")
		}
	}

	return problems, nil
}

// Guard reports what SwiftLint cannot see: the settings the manifest ends up
// with, XCTest outside UI tests, file-wide suppressions, foreign tools, and
// Xcode projects that skip `Config/Bedrock.xcconfig`. With fix, it converts
// XCTest first.
func (Swift) Guard(ctx context.Context, c *language.Context, fix bool) ([]language.Finding, error) {
	var findings []string

	if fsutil.Exists(filepath.Join(c.Root, manifestFile)) {
		findings = dumpFindings(ctx, c)
	}

	sources, err := fsutil.ListFiles(ctx, c.Runner, c.Root, func(file string) bool {
		return strings.HasSuffix(file, ".swift")
	})
	if err != nil {
		return nil, err
	}

	for _, file := range sources {
		found, fileErr := fileFindings(c, file, fix)
		if fileErr != nil {
			return nil, fileErr
		}

		findings = append(findings, found...)
	}

	findings = append(findings, foreignFindings(c.Root)...)

	projects, err := projectFindings(c.Root)

	return append(findings, projects...), err
}
