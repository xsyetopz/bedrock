// Package swift is the Swift brick: strict Swift 6 manifests, swift-format,
// SwiftLint, Swift Testing, Xcode wiring, and an XCTest converter.
package swift

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/template"
)

const (
	languageID = "swift"
	// swiftTool is the toolchain program the language runs.
	swiftTool = "swift"
	// swiftBlock and cxxBlock are the blocks of `Package.swift` that bedrock
	// keeps up to date.
	swiftBlock = "swift-bedrock"
	cxxBlock   = "swift-bedrock-cxx"
)

// Swift is the Swift language.
type Swift struct{}

// New returns the Swift language.
func New() Swift { return Swift{} }

// Spec declares Swift's templates, markers, and tools.
func (Swift) Spec() language.Spec {
	return language.Spec{
		ID:      languageID,
		Block:   swiftBlock,
		Markers: []string{manifestFile, ".swift-version", ".swift-format", ".swiftlint.yml"},
		Tools: []language.Tool{
			{Name: swiftTool, Install: "swiftly (https://www.swift.org/install/), or Xcode"},
			{Name: "swiftlint", Install: "brew install swiftlint"},
		},
		Sources: []string{
			"**/*.swift", manifestFile, ".swift-format", ".swift-version", ".swiftlint.yml",
			"**/.swiftlint.yml", "**/*.xcodeproj/project.pbxproj", "Config/*.xcconfig",
		},
		Kinds: kinds(),
		// Only the Xcode app is Apple-only; every package kind stays
		// cross-platform and builds wherever the Swift toolchain does.
		Platforms: map[string][]string{"app": appPlatforms()},
	}
}

// Variables fill the type name of the starter package.
func (Swift) Variables(_ context.Context, c *language.Context) (template.Variables, error) {
	name := typeName(filepath.Base(c.Root))

	return template.Variables{"typeName": name, "lowerName": strings.ToLower(name)}, nil
}

// AfterApply resolves the package's dependencies when swift is installed. A
// failure never fails the adopt, because the recipes run SwiftPM again and
// report what is wrong.
func (Swift) AfterApply(ctx context.Context, c *language.Context) error {
	if !c.Runner.LookPath(swiftTool) || !fsutil.Exists(filepath.Join(c.Root, manifestFile)) {
		return nil
	}

	// The result is dropped on purpose: see above.
	_, _ = runSwift(ctx, c.Runner, c.Root, "package", "--disable-sandbox", "resolve")

	return nil
}

// Runs says what AfterApply does, as the plan lists it.
func (Swift) Runs() string { return "swift package resolve" }
