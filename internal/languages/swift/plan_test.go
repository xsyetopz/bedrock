package swift_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/languages/swift"
)

const oldManifest = "// swift-tools-version:5.9\nimport PackageDescription\n" +
	"let package = Package(name: \"X\", platforms: [.macOS(.v10_15)])\n"

func TestAdoptRaisesTheToolsVersionAndAddsTheStrictBlock(t *testing.T) {
	t.Parallel()

	// Arrange
	root, _ := planSwift(t, map[string]string{"Package.swift": oldManifest})

	// Act
	result := bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "swift", nil))

	// Assert
	manifest := bedrocktest.Read(t, root, "Package.swift")
	if !strings.HasPrefix(manifest, "// swift-tools-version: 6.4") {
		t.Errorf("manifest keeps the old tools version:\n%s", manifest)
	}

	if !strings.Contains(manifest, ".macOS(.v12)") || strings.Contains(manifest, ".v10_15") {
		t.Errorf("manifest keeps the old macOS minimum:\n%s", manifest)
	}

	if !strings.Contains(manifest, "swift-bedrock") {
		t.Errorf("manifest lacks the `swift-bedrock` block:\n%s", manifest)
	}

	if !result.ProjectFiles["Package.swift"] {
		t.Error("`Package.swift` is not the project's own")
	}
}

func TestAdoptConvertsXCTestAndKeepsTheFileTheProjects(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"Package.swift": "let package = 1\n", "Tests/ATests/A.swift": xctestSource}

	// Act
	_, result := planSwift(t, files)

	// Assert
	if got := bedrocktest.Written(result.Steps, "Tests/ATests/A.swift"); !strings.Contains(got, "import Testing") {
		t.Errorf("test file is not converted:\n%s", got)
	}

	if !result.ProjectFiles["Tests/ATests/A.swift"] {
		t.Error("the converted file is not the project's own")
	}
}

func TestTestLintConfigNeedsATestsFolder(t *testing.T) {
	t.Parallel()

	// Arrange
	with := map[string]string{"Package.swift": "let package = 1\n", "Tests/ATests/A.swift": "import Testing\n"}
	without := map[string]string{"Package.swift": "let package = 1\n"}

	// Act
	_, first := planSwift(t, with)
	_, second := planSwift(t, without)

	// Assert
	if !slices.Contains(bedrocktest.Paths(first.Steps), "Tests/.swiftlint.yml") {
		t.Error("a repository with `Tests` lacks `Tests/.swiftlint.yml`")
	}

	if slices.Contains(bedrocktest.Paths(second.Steps), "Tests/.swiftlint.yml") {
		t.Error("a repository without `Tests` gets `Tests/.swiftlint.yml`")
	}
}

func TestAdoptWiresAnXcodeProjectToTheXcconfig(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"A.xcodeproj/project.pbxproj": pbxproj}

	// Act
	root, result := planSwift(t, files)

	// Assert
	if bedrocktest.Written(result.Steps, "Config/Bedrock.xcconfig") == "" {
		t.Error("plan lacks `Config/Bedrock.xcconfig`")
	}

	wired := bedrocktest.Written(result.Steps, "A.xcodeproj/project.pbxproj")
	if got := strings.Count(wired, "Bedrock.xcconfig"); got < 3 {
		t.Errorf("project references the xcconfig %d times, want a file reference and two configurations:\n%s",
			got, wired)
	}

	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "swift", nil))

	if again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "swift", nil)); len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestAdoptLeavesAProjectWithAnotherBaseConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	own := strings.Replace(
		pbxproj,
		"isa = XCBuildConfiguration;",
		"isa = XCBuildConfiguration;\n\t\t\tbaseConfigurationReference = ZZZ1;",
		1,
	)

	// Act
	_, result := planSwift(t, map[string]string{"A.xcodeproj/project.pbxproj": own})

	// Assert
	if bedrocktest.Written(result.Steps, "A.xcodeproj/project.pbxproj") != "" {
		t.Error("plan rewrites a project with its own base configuration")
	}
}

func TestAdoptNotesThatThereIsNothingToAdopt(t *testing.T) {
	t.Parallel()

	// Act
	_, result := planSwift(t, nil)

	// Assert
	if slices.ContainsFunc(bedrocktest.Paths(result.Steps), func(p string) bool { return p == "Package.swift" }) {
		t.Error("plan starts a package without `init`")
	}

	if !slices.ContainsFunc(result.Notes, func(n string) bool { return strings.Contains(n, "`bedrock init`") }) {
		t.Errorf("notes lack the `bedrock init` hint: %v", result.Notes)
	}
}

func TestAdoptNotesCxxSourcesWithoutTheCppLanguage(t *testing.T) {
	t.Parallel()

	// Act
	_, result := planSwift(
		t,
		map[string]string{"Package.swift": "let package = 1\n", "Sources/c/a.cpp": "int a;\n"},
	)

	// Assert
	if !slices.ContainsFunc(result.Notes, func(n string) bool { return strings.Contains(n, "`--cxx`") }) {
		t.Errorf("notes lack the `--cxx` hint: %v", result.Notes)
	}
}

func TestSecondAdoptPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"Package.swift": oldManifest, "Tests/ATests/A.swift": xctestSource}
	root, _ := planSwift(t, files)
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "swift", nil))

	// Act
	again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "swift", nil))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestInitStartsAPackageForEachKind(t *testing.T) {
	t.Parallel()

	for kind, want := range map[string]string{
		"library":           ".library(name: ",
		"executable":        ".executableTarget(",
		"tool":              `.product(name: "ArgumentParser"`,
		"macro":             ".macro(",
		"build-tool-plugin": ".buildTool()",
		"command-plugin":    ".command(",
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := namedRepo(t, "Starter")

			// Act
			bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", config.Options{"kind": kind}))

			// Assert
			if manifest := bedrocktest.Read(t, root, "Package.swift"); !strings.Contains(manifest, want) {
				t.Errorf("manifest lacks %s:\n%s", want, manifest)
			}

			again := bedrocktest.Plan(t, root, true, bedrocktest.Select(t, "swift", config.Options{"kind": kind}))
			if len(again.Steps) != 0 {
				t.Errorf("second init plans %v", bedrocktest.Paths(again.Steps))
			}
		})
	}
}

func TestInitAppStartsAnXcodeProjectThatUsesTheXcconfig(t *testing.T) {
	t.Parallel()

	// Arrange
	root := namedRepo(t, "Starter")

	// Act
	result := bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", config.Options{"kind": "app"}))

	// Assert
	var projects []string

	for _, path := range bedrocktest.Paths(result.Steps) {
		if strings.HasSuffix(path, ".xcodeproj/project.pbxproj") {
			projects = append(projects, path)
		}
	}

	if len(projects) != 1 {
		t.Fatalf("init writes projects %v, want one", projects)
	}

	if !strings.Contains(bedrocktest.Read(t, root, projects[0]), "Bedrock.xcconfig") {
		t.Error("the starter project does not use the xcconfig")
	}

	if bedrocktest.Read(t, root, "Config/Bedrock.xcconfig") == "" {
		t.Error("init writes no `Config/Bedrock.xcconfig`")
	}
}

func TestInitKeepsAnExistingManifest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := namedRepo(t, "Starter")
	bedrocktest.Write(t, root, "Package.swift", "let package = 1\n")
	bedrocktest.CommitAll(t, root)

	// Act
	result := bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", config.Options{"kind": "library"}))

	// Assert
	if !strings.Contains(bedrocktest.Read(t, root, "Package.swift"), "let package = 1") {
		t.Error("init replaces the project's manifest")
	}

	if !result.ProjectFiles["Package.swift"] {
		t.Error("`Package.swift` is not the project's own")
	}
}

func TestInitRejectsADirectoryThatCannotNameAPackage(t *testing.T) {
	t.Parallel()

	// Arrange
	root := namedRepo(t, "1bad")

	c := bedrocktest.Context(t, root, bedrocktest.Select(t, "swift", nil))
	c.Init = true

	// Act
	_, err := swift.New().Plan(t.Context(), c)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "cannot name a Swift package") {
		t.Errorf("error = %v, want one that says the directory cannot name a package", err)
	}
}

const pbxproj = `// !$*UTF8*$!
{
	objects = {

/* Begin PBXFileReference section */
/* End PBXFileReference section */

/* Begin PBXGroup section */
		AAA1 = {
			isa = PBXGroup;
			children = (
				BBB1 /* Sources */,
			);
		};
/* End PBXGroup section */

/* Begin PBXProject section */
		PPP1 /* Project object */ = {
			isa = PBXProject;
			buildConfigurationList = LLL1 /* list */;
			mainGroup = AAA1;
		};
/* End PBXProject section */

/* Begin XCBuildConfiguration section */
		CCC1 /* Debug */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
			};
			name = Debug;
		};
		CCC2 /* Release */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
			};
			name = Release;
		};
/* End XCBuildConfiguration section */

/* Begin XCConfigurationList section */
		LLL1 /* list */ = {
			isa = XCConfigurationList;
			buildConfigurations = (
				CCC1 /* Debug */,
				CCC2 /* Release */,
			);
		};
/* End XCConfigurationList section */
	};
}
`
