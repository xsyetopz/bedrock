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

	if testing.Short() {
		t.Skip("end-to-end tests run real toolchains")
	}
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

func TestInitBuildsTestsAndPassesTheGuardWithSwiftPM(t *testing.T) {
	t.Parallel()
	skipWithoutSwift(t)

	// The app kind is an Xcode project, which SwiftPM cannot build.
	for _, kind := range []string{"library", "executable"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

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
		})
	}
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
