package swift_test

import (
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const swiftManifest = "// swift-tools-version: 6.4\nimport PackageDescription\n" +
	"let package = Package(name: \"X\", platforms: [.macOS(.v13)])\n"

func TestAdoptAddsCxxInteropForACFamilyTarget(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Package.swift":         swiftManifest,
		"Sources/A/a.swift":     "import B\n",
		"Sources/B/b.cpp":       "int b;\n",
		"Sources/B/include/b.h": "int b;\n",
	})

	// Act
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "swift", nil), bedrocktest.Select(t, "cpp", nil))

	// Assert
	if manifest := bedrocktest.Read(t, root, "Package.swift"); !strings.Contains(manifest, "swift-bedrock-cxx") {
		t.Errorf("manifest lacks the `swift-bedrock-cxx` block:\n%s", manifest)
	}
}

func TestAdoptRemovesACxxInteropBlockNoTargetNeeds(t *testing.T) {
	t.Parallel()

	// Arrange
	stale := swiftManifest + "\n// >>> swift-bedrock-cxx\npackage.cxxLanguageStandard = .cxx20\n// <<< swift-bedrock-cxx\n"
	root := bedrocktest.Repo(t, map[string]string{
		"Package.swift":                  stale,
		"Sources/A/a.swift":              "let a = 1\n",
		"Sources/A/Resources/native.cpp": "int a;\n",
	})

	// Act
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "swift", nil), bedrocktest.Select(t, "cpp", nil))

	// Assert
	manifest := bedrocktest.Read(t, root, "Package.swift")
	if strings.Contains(manifest, "swift-bedrock-cxx") || strings.Contains(manifest, "cxxLanguageStandard") {
		t.Errorf("manifest keeps the C++ interop block:\n%s", manifest)
	}

	if !strings.Contains(manifest, "// >>> swift-bedrock\n") {
		t.Errorf("manifest lacks the `swift-bedrock` block:\n%s", manifest)
	}
}

func TestAdoptLeavesCxxInteropOffForCxxResourcesOfASwiftTarget(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Package.swift":                  swiftManifest,
		"Sources/A/a.swift":              "let a = 1\n",
		"Sources/A/Resources/native.cpp": "int a;\n",
		"Sources/A/Resources/native.h":   "int a;\n",
	})

	// Act
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "swift", nil), bedrocktest.Select(t, "cpp", nil))

	// Assert
	if manifest := bedrocktest.Read(t, root, "Package.swift"); strings.Contains(manifest, "swift-bedrock-cxx") {
		t.Errorf("manifest turns C++ interop on for a package with no C-family target:\n%s", manifest)
	}
}
