package swift_test

import (
	"io"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock"
	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/languages/swift"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/step"
)

// cppStub stands in for the C++ language, which the Go catalog does not
// have yet. Swift reads the C++ starter and manifest templates through the
// `cpp` id, so the stub only has to be selected.
type cppStub struct{}

func (cppStub) Spec() language.Spec {
	return language.Spec{ID: "cpp", Block: "cpp-bedrock"}
}

func skipE2E(t *testing.T) {
	t.Helper()

	bedrocktest.E2E(t, "end-to-end tests run real toolchains")
}

func skipWithoutSwift(t *testing.T) {
	t.Helper()
	skipE2E(t)

	if !(execx.System{Stdout: io.Discard, Stderr: io.Discard}).LookPath("swift") {
		t.Skip("`swift` is not installed")
	}
}

// swiftOptions are the options of an init of kind.
func swiftOptions(kind string) config.Options { return config.Options{"kind": kind} }

func TestInitWritesARepositoryThatPassesTheGuardFilesCheck(t *testing.T) {
	t.Parallel()
	skipE2E(t)

	for _, kind := range swift.New().Spec().Kinds {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := namedRepo(t, "Starter")
			bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", swiftOptions(kind)))
			bedrocktest.CommitAll(t, root)
			c := bedrocktest.Context(t, root, bedrocktest.Select(t, "swift", swiftOptions(kind)))

			// Act
			found, err := swift.New().Guard(t.Context(), c, false)
			if err != nil {
				t.Fatal(err)
			}

			// Assert
			for _, finding := range found {
				// `swift` may be absent or unable to resolve here; the manifest check
				// belongs to the SwiftPM test below.
				if !strings.HasPrefix(finding, "`Package.swift`:") {
					t.Errorf("unexpected finding: %s", finding)
				}
			}
		})
	}
}

// buildsTestsAndGuards inits a package of kind and runs the real SwiftPM
// build, test and guard on it.
func buildsTestsAndGuards(t *testing.T, kind string) {
	t.Helper()

	// Arrange
	root := namedRepo(t, "Starter")
	bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", swiftOptions(kind)))
	bedrocktest.CommitAll(t, root)
	c := bedrocktest.Context(t, root, bedrocktest.Select(t, "swift", swiftOptions(kind)))

	// Act
	build := runIn(t, c, "build")
	tests := runIn(t, c, "test")
	found, err := swift.New().Guard(t.Context(), c, false)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if build.Code != 0 || !strings.Contains(string(build.Stdout)+string(build.Stderr), "Build complete") {
		t.Errorf("build exited %d:\n%s%s", build.Code, build.Stdout, build.Stderr)
	}

	if tests.Code != 0 {
		t.Errorf("tests exited %d:\n%s%s", tests.Code, tests.Stdout, tests.Stderr)
	}

	if len(found) != 0 {
		t.Errorf("findings = %v", found)
	}
}

func TestInitBuildsTestsAndPassesTheGuardWithSwiftPM(t *testing.T) {
	t.Parallel()
	skipWithoutSwift(t)

	// The app kind is an Xcode project, which SwiftPM cannot build. The tool
	// and macro kinds fetch and compile swift-syntax or ArgumentParser, so they
	// run one after another below.
	for _, kind := range []string{"library", "executable", "build-tool-plugin", "command-plugin"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			buildsTestsAndGuards(t, kind)
		})
	}
}

//nolint:paralleltest // swift-syntax is a heavy build, so these kinds build in sequence.
func TestInitBuildsTestsAndPassesTheGuardForKindsWithDependencies(t *testing.T) {
	skipWithoutSwift(t)

	for _, kind := range []string{"tool", "macro"} {
		t.Run(kind, func(t *testing.T) { buildsTestsAndGuards(t, kind) }) //nolint:paralleltest // see above.
	}
}

// simulatorDestinations are the xcodebuild destinations of the app platforms,
// with the SDK each one needs.
func simulatorDestinations() []struct{ platform, sdk, destination string } {
	return []struct{ platform, sdk, destination string }{
		{"ios", "iphonesimulator", "generic/platform=iOS Simulator"},
		{"tvos", "appletvsimulator", "generic/platform=tvOS Simulator"},
		{"visionos", "xrsimulator", "generic/platform=visionOS Simulator"},
		{"watchos", "watchsimulator", "generic/platform=watchOS Simulator"},
		{"macos", "macosx", "platform=macOS"},
	}
}

// hasSDK reports whether Xcode has the SDK installed.
func hasSDK(t *testing.T, sdk string) bool {
	t.Helper()

	result, err := (execx.System{Stdout: io.Discard, Stderr: io.Discard}).Run(t.Context(), execx.Command{
		Name: "xcrun",
		Args: []string{"--sdk", sdk, "--show-sdk-path"},
	})

	return err == nil && result.Code == 0
}

//nolint:paralleltest // each build is a full Xcode build, so the platforms build in sequence.
func TestInitAppBuildsForEachPlatformWithXcodebuild(t *testing.T) {
	skipE2E(t)

	if !(execx.System{Stdout: io.Discard, Stderr: io.Discard}).LookPath("xcodebuild") {
		t.Skip("`xcodebuild` is not installed")
	}

	for _, target := range simulatorDestinations() {
		t.Run(target.platform, func(t *testing.T) { //nolint:paralleltest // see above.
			if !hasSDK(t, target.sdk) {
				t.Skipf("the %s SDK is not installed", target.sdk)
			}

			// Arrange
			root := namedRepo(t, "Starter")
			options := config.Options{"kind": "app", "platform": target.platform}
			bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", options))
			bedrocktest.CommitAll(t, root)
			c := bedrocktest.Context(t, root, bedrocktest.Select(t, "swift", options))

			// Act
			built := xcodebuild(t, c, target.destination)

			// Assert
			if built.Code != 0 {
				t.Errorf("xcodebuild exited %d:\n%s%s", built.Code, built.Stdout, built.Stderr)
			}
		})
	}
}

// xcodebuild builds the starter app and its tests for the destination, without
// signing.
func xcodebuild(t *testing.T, c *language.Context, destination string) execx.Result {
	t.Helper()

	result, err := c.Runner.Run(t.Context(), execx.Command{
		Dir:  c.Root,
		Name: "xcodebuild",
		Args: []string{
			"build-for-testing", "-project", "Starter.xcodeproj", "-scheme", "Starter",
			"-destination", destination, "-derivedDataPath", t.TempDir(),
			"CODE_SIGNING_ALLOWED=NO",
		},
		Unset: execx.HookGitVars(),
	})
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func TestInitLibraryWithCppBuildsCxxInterop(t *testing.T) {
	t.Parallel()
	skipWithoutSwift(t)

	// Arrange
	root := namedRepo(t, "Starter")
	registry, err := language.NewRegistry(swift.New(), cppStub{})
	if err != nil {
		t.Fatal(err)
	}

	engine := &plan.Engine{
		Registry:  registry,
		Templates: bedrock.Templates(),
		Runner:    execx.System{Stdout: io.Discard, Stderr: io.Discard},
		Stdout:    io.Discard,
		Stderr:    io.Discard,
	}
	selected := []plan.Selected{
		{Language: swift.New(), Options: swiftOptions("library")},
		{Language: cppStub{}, Options: config.Options{}},
	}

	built, err := engine.Build(t.Context(), root, selected, plan.Options{Version: bedrocktest.Version, Init: true})
	if err != nil {
		t.Fatal(err)
	}

	if err = step.Apply(root, built.Steps); err != nil {
		t.Fatal(err)
	}

	bedrocktest.CommitAll(t, root)

	c := engine.Context(root, selected, selected[0], false)

	// Act
	build := runIn(t, c, "build")
	tests := runIn(t, c, "test")

	// Assert
	if build.Code != 0 {
		t.Errorf("build exited %d:\n%s%s", build.Code, build.Stdout, build.Stderr)
	}

	if tests.Code != 0 {
		t.Errorf("tests exited %d:\n%s%s", tests.Code, tests.Stdout, tests.Stderr)
	}

	if !strings.Contains(bedrocktest.Read(t, root, "Package.swift"), "swift-bedrock-cxx") {
		t.Error("`Package.swift` lacks the `swift-bedrock-cxx` block")
	}
}

// runIn runs `swift <verb> --disable-sandbox` in the context's repository,
// as the recipes do.
func runIn(t *testing.T, c *language.Context, verb string) execx.Result {
	t.Helper()

	result, err := c.Runner.Run(t.Context(), execx.Command{
		Dir:   c.Root,
		Name:  "swift",
		Args:  []string{verb, "--disable-sandbox"},
		Unset: execx.HookGitVars(),
	})
	if err != nil {
		t.Fatal(err)
	}

	return result
}
