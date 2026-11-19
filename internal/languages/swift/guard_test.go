package swift_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const manifestFile = "Package.swift"

func anyContains(found []string, want string) bool {
	return slices.ContainsFunc(found, func(f string) bool { return strings.Contains(f, want) })
}

func TestGuardAcceptsAStrictPackage(t *testing.T) {
	t.Parallel()

	// Act
	found, _ := guardWith(t, map[string]string{manifestFile: "x\n"}, strictDump(), false)

	// Assert
	if len(found) != 0 {
		t.Errorf("findings = %v", found)
	}
}

func TestGuardReportsWhatTheManifestResolvesTo(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct{ dump, want string }{
		"old tools version": {
			`{"toolsVersion":{"_version":"5.9.0"},"swiftLanguageVersions":["6"],"targets":[]}`,
			"swift-tools-version is 5.9.0; use 6.4",
		},
		"other language modes": {
			`{"toolsVersion":{"_version":"6.4.0"},"swiftLanguageVersions":["5","6"],"targets":[]}`,
			"the language modes are [5, 6]; use [.v6] alone",
		},
		"unsafe flags": {
			`{"toolsVersion":{"_version":"6.4.0"},"swiftLanguageVersions":["6"],"targets":[` +
				`{"name":"Lib","type":"regular","settings":[{"kind":{"unsafeFlags":{"_0":["-x"]}}}]}]}`,
			"target Lib uses `unsafeFlags`",
		},
		"missing strict settings": {
			`{"toolsVersion":{"_version":"6.4.0"},"swiftLanguageVersions":["6"],"targets":[` +
				`{"name":"Lib","type":"regular","settings":[{"kind":{"strictMemorySafety":{}}}]}]}`,
			"target Lib lacks enableUpcomingFeature:ExistentialAny",
		},
		"printed no package": {"not json", "printed no package"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := strictDump()
			fake.stdout = test.dump

			// Act
			found, _ := guardWith(t, map[string]string{manifestFile: "x\n"}, fake, false)

			// Assert
			if !anyContains(found, test.want) {
				t.Errorf("findings = %v, want one containing %q", found, test.want)
			}
		})
	}
}

func TestGuardSkipsTargetsWithoutSwiftSettings(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := strictDump()
	fake.stdout = `{"toolsVersion":{"_version":"6.4.0"},"swiftLanguageVersions":["6"],` +
		`"targets":[{"name":"C","type":"system"},{"name":"B","type":"binary"},{"name":"P","type":"plugin"}]}`

	// Act
	found, _ := guardWith(t, map[string]string{manifestFile: "x\n"}, fake, false)

	// Assert
	if len(found) != 0 {
		t.Errorf("findings = %v", found)
	}
}

func TestGuardSaysWhenSwiftIsMissingOrFails(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := dumper{present: false}
	failing := dumper{present: true, code: 1, stderr: "\n error: bad manifest\nmore\n"}

	// Act
	gone, _ := guardWith(t, map[string]string{manifestFile: "x\n"}, missing, false)
	failed, _ := guardWith(t, map[string]string{manifestFile: "x\n"}, failing, false)

	// Assert
	if !anyContains(gone, "`swift` is missing, so the manifest was not checked") {
		t.Errorf("findings = %v", gone)
	}

	if !anyContains(failed, "`swift package dump-package` failed: error: bad manifest") {
		t.Errorf("findings = %v", failed)
	}
}

func TestGuardDoesNotDumpWithoutAManifest(t *testing.T) {
	t.Parallel()

	// Act
	found, _ := guardWith(t, map[string]string{"README.md": "x\n"}, dumper{present: false}, false)

	// Assert
	if len(found) != 0 {
		t.Errorf("findings = %v", found)
	}
}

func TestGuardFlagsXCTestOutsideUITests(t *testing.T) {
	t.Parallel()

	// Act
	found, _ := guardWith(t, map[string]string{"Tests/A.swift": xctestSource}, strictDump(), false)

	// Assert
	if !anyContains(found, "`Tests/A.swift`: imports `XCTest` outside a `*UITests` target") {
		t.Errorf("findings = %v", found)
	}
}

func TestGuardSkipsWhatSwiftLintExcludes(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		".swiftlint.yml":                "excluded:\n  - Vendor\n  - \"**/Generated\"\n",
		"Vendor/Kit/A.swift":            xctestSource,
		"Sources/App/Generated/B.swift": xctestSource,
		"Sources/App/Generated.swift":   xctestSource,
		"Sources/Lib/Vendor/C.swift":    xctestSource,
	}

	// Act
	found, _ := guardWith(t, files, dumper{present: false}, false)

	// Assert
	for _, kept := range []string{"Sources/App/Generated.swift", "Sources/Lib/Vendor/C.swift"} {
		if !anyContains(found, "`"+kept+"`") {
			t.Errorf("no finding for %s: %v", kept, found)
		}
	}

	for _, excluded := range []string{"Vendor/Kit/A.swift", "Sources/App/Generated/B.swift"} {
		if anyContains(found, "`"+excluded+"`") {
			t.Errorf("finding for excluded %s: %v", excluded, found)
		}
	}
}

func TestGuardLeavesXCTestInUITestTargets(t *testing.T) {
	t.Parallel()

	// Act
	found, _ := guardWith(t, map[string]string{"AppUITests/A.swift": xctestSource}, strictDump(), false)

	// Assert
	if len(found) != 0 {
		t.Errorf("findings = %v", found)
	}
}

func TestGuardFixConvertsXCTest(t *testing.T) {
	t.Parallel()

	// Act
	found, root := guardWith(t, map[string]string{"Tests/A.swift": xctestSource}, strictDump(), true)

	// Assert
	if len(found) != 0 {
		t.Errorf("findings = %v", found)
	}

	if !strings.Contains(bedrocktest.Read(t, root, "Tests/A.swift"), "import Testing") {
		t.Error("the file is not converted")
	}
}

func TestGuardFixReportsWhatConversionLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	source := "import XCTest\nfinal class A: XCTestCase {\n  func testB() {\n    expectation(description: \"x\")\n  }\n}\n"

	// Act
	found, _ := guardWith(t, map[string]string{"Tests/A.swift": source}, strictDump(), true)

	// Assert
	if !anyContains(found, "`Tests/A.swift`: line ") {
		t.Errorf("findings = %v", found)
	}
}

func TestGuardReportsFileWideSuppressionsWithTheLine(t *testing.T) {
	t.Parallel()

	// Act
	found, _ := guardWith(
		t,
		map[string]string{"Sources/A.swift": "// swiftlint:disable all\nlet a = 1\n"},
		strictDump(),
		false,
	)

	// Assert
	if len(found) != 1 || !strings.HasPrefix(found[0], "`Sources/A.swift`:1: ") {
		t.Errorf("findings = %v", found)
	}
}

func TestGuardFlagsForeignDependencyFiles(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"Cartfile", "Podfile", ".swiftformat"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			// Act
			found, _ := guardWith(t, map[string]string{file: ""}, strictDump(), false)

			// Assert
			if len(found) != 1 || !strings.HasPrefix(found[0], file+": ") {
				t.Errorf("findings = %v", found)
			}
		})
	}
}

func TestGuardFlagsAnXcodeProjectThatSkipsTheXcconfigAndOverridesSettings(t *testing.T) {
	t.Parallel()

	// Act
	found, _ := guardWith(
		t,
		map[string]string{"A.xcodeproj/project.pbxproj": "SWIFT_VERSION = 5.0;\n"},
		strictDump(),
		false,
	)

	// Assert
	if len(found) != 2 || !strings.Contains(found[0], "`Config/Bedrock.xcconfig`") ||
		!strings.HasSuffix(found[1], "; remove it") {
		t.Errorf("findings = %v", found)
	}
}
