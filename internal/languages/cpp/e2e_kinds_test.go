package cpp_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
)

// llvmBin returns the directory of clang-format and run-clang-tidy: PATH's,
// else Homebrew's LLVM, which does not link them into PATH.
func llvmBin() (string, bool) {
	if path, err := exec.LookPath("run-clang-tidy"); err == nil {
		return filepath.Dir(path), true
	}

	prefix, err := exec.Command("brew", "--prefix", "llvm").Output()
	if err != nil {
		return "", false
	}

	dir := filepath.Join(strings.TrimSpace(string(prefix)), "bin")
	if _, statErr := os.Stat(filepath.Join(dir, "run-clang-tidy")); statErr != nil {
		return "", false
	}

	return dir, true
}

// requireTools skips a test that builds with the real toolchain when it is
// short, or when a tool is off PATH.
func requireTools(t *testing.T, tools ...string) {
	t.Helper()

	if testing.Short() {
		t.Skip("builds a project with the real toolchain")
	}

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("`%s` is not on `PATH`", tool)
		}
	}
}

// runIn runs name with args in dir and reports whether it exited 0, with its
// output.
func runIn(t *testing.T, dir string, env []string, name string, args ...string) (bool, string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), name, args...)
	command.Dir = dir
	command.Env = append(os.Environ(), env...)

	output, err := command.CombinedOutput()

	return err == nil, string(output)
}

// initKind inits kind with platforms in a new repository and commits it.
func initKind(t *testing.T, kind string, platforms ...string) string {
	t.Helper()

	options := config.Options{"kind": kind}
	if len(platforms) > 0 {
		options["platform"] = strings.Join(platforms, ",")
	}

	root := bedrocktest.Repo(t, nil)
	bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "cpp", options))
	bedrocktest.CommitAll(t, root)

	return root
}

// bedrockBinary builds the CLI of this checkout, for the recipes that run
// `bedrock cpp ...`.
func bedrockBinary(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "bedrock")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", path, "./cmd/bedrock")
	build.Dir = filepath.Join("..", "..", "..")

	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build bedrock: %v\n%s", err, output)
	}

	return path
}

// justHome returns a HOME whose cache holds the local bedrock under the
// version `.editorconfig` pins, so `just` runs it instead of downloading the
// release. The asset names follow the justfile's.
func justHome(t *testing.T) string {
	t.Helper()

	binary, err := os.ReadFile(bedrockBinary(t))
	if err != nil {
		t.Fatal(err)
	}

	arch := map[string]string{"arm64": "arm64", "amd64": "x64"}[runtime.GOARCH]
	cache := ".cache"

	if runtime.GOOS == "darwin" {
		cache = filepath.Join("Library", "Caches")
	}

	home := t.TempDir()
	dir := filepath.Join(home, cache, "bedrock", bedrocktest.Version)

	if err = os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	asset := filepath.Join(dir, "bedrock-"+runtime.GOOS+"-"+arch)
	if err = os.WriteFile(asset, binary, 0o700); err != nil {
		t.Fatal(err)
	}

	return home
}

func TestEndToEndAKindPassesTheJustRecipes(t *testing.T) {
	t.Parallel()

	for kind := range kindFiles() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			requireTools(t, "just", "cmake", "go")

			if _, ok := llvmBin(); !ok {
				t.Skip("`run-clang-tidy` is not installed")
			}

			// Arrange
			root := initKind(t, kind)
			env := []string{"HOME=" + justHome(t)}

			// Act
			var failed []string

			for _, recipe := range []string{"cpp::format-check", "cpp::lint", "cpp::test"} {
				if ok, output := runIn(t, root, env, "just", recipe); !ok {
					failed = append(failed, "just "+recipe+":\n"+output)
				}
			}

			// Assert
			if len(failed) != 0 {
				t.Errorf("failed recipes:\n%s", strings.Join(failed, "\n"))
			}
		})
	}
}

func TestEndToEndAKindBuildsAndTestsWithTheDefaultPreset(t *testing.T) {
	t.Parallel()

	for kind := range kindFiles() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			requireTools(t, "cmake")

			// Arrange
			root := initKind(t, kind)

			// Act
			var failed []string

			for _, args := range [][]string{
				{"cmake", "--preset", "default"},
				{"cmake", "--build", "--preset", "default"},
				{"ctest", "--preset", "default"},
			} {
				if ok, output := runIn(t, root, nil, args[0], args[1:]...); !ok {
					failed = append(failed, strings.Join(args, " ")+":\n"+output)
				}
			}

			// Assert
			if len(failed) != 0 {
				t.Errorf("failed steps:\n%s", strings.Join(failed, "\n"))
			}
		})
	}
}

func TestEndToEndASharedLibraryExportsItsAPI(t *testing.T) {
	t.Parallel()
	requireTools(t, "cmake")

	// Arrange
	root := initKind(t, "library")
	build := filepath.Join(root, "build", "shared")

	// Act
	var failed []string

	for _, args := range [][]string{
		{"cmake", "-S", ".", "-B", build, "-DBUILD_SHARED_LIBS=ON"},
		{"cmake", "--build", build},
		{"ctest", "--test-dir", build, "--output-on-failure"},
	} {
		if ok, output := runIn(t, root, nil, args[0], args[1:]...); !ok {
			failed = append(failed, strings.Join(args, " ")+":\n"+output)
		}
	}

	// Assert
	if len(failed) != 0 {
		t.Errorf("failed steps:\n%s", strings.Join(failed, "\n"))
	}
}

// readiness says how a platform's preset can run here.
type readiness struct {
	// skip is why the preset cannot build here, empty when it can.
	skip string
	// env sets what the preset's toolchain file reads.
	env []string
	// run is true when the platform's tests can run on this machine.
	run bool
}

func sdkReady(sdk string) string {
	if runtime.GOOS != "darwin" {
		return "builds on macOS"
	}

	if _, err := exec.LookPath("xcodebuild"); err != nil {
		return "`xcodebuild` is not on `PATH`"
	}

	if err := exec.Command("xcrun", "--sdk", sdk, "--show-sdk-path").Run(); err != nil {
		return "the " + sdk + " SDK is not installed"
	}

	return ""
}

// ndkDir returns the Android NDK: ANDROID_NDK_HOME, else the newest under
// ANDROID_HOME.
func ndkDir() string {
	if dir := os.Getenv("ANDROID_NDK_HOME"); dir != "" {
		return dir
	}

	for _, sdk := range []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT")} {
		if sdk == "" {
			continue
		}

		found, _ := filepath.Glob(filepath.Join(sdk, "ndk", "*"))
		if len(found) > 0 {
			slices.Sort(found)

			return found[len(found)-1]
		}
	}

	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func hostPlatform(want string) readiness {
	if runtime.GOOS != want {
		return readiness{skip: "runs on " + want, env: nil, run: false}
	}

	return readiness{skip: "", env: nil, run: true}
}

// platformReadiness probes the toolchain a platform's preset needs.
func platformReadiness(platform string) readiness {
	none := readiness{skip: "", env: nil, run: false}

	if _, err := exec.LookPath("cmake"); err != nil {
		return readiness{skip: "`cmake` is not on `PATH`", env: nil, run: false}
	}

	switch platform {
	case "linux", "freebsd", "windows":
		return hostPlatform(platform)
	case "macos":
		return hostPlatform("darwin")
	case "ios":
		return readiness{skip: sdkReady("iphoneos"), env: nil, run: false}
	case "tvos":
		return readiness{skip: sdkReady("appletvos"), env: nil, run: false}
	case "watchos":
		return readiness{skip: sdkReady("watchos"), env: nil, run: false}
	case "visionos":
		return readiness{skip: sdkReady("xros"), env: nil, run: false}
	}

	if _, err := exec.LookPath("ninja"); err != nil {
		return readiness{skip: "`ninja` is not on `PATH`", env: nil, run: false}
	}

	switch platform {
	case "maccatalyst":
		if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
			return readiness{skip: "builds the arm64 slice on Apple silicon", env: nil, run: false}
		}

		return readiness{skip: "", env: nil, run: true}
	case "android":
		ndk := ndkDir()
		if !fileExists(filepath.Join(ndk, "build", "cmake", "android.toolchain.cmake")) {
			return readiness{skip: "the Android NDK is not installed", env: nil, run: false}
		}

		return readiness{skip: "", env: []string{"ANDROID_NDK_HOME=" + ndk}, run: false}
	case "wasm":
		sdk := os.Getenv("EMSDK")
		if !fileExists(
			filepath.Join(sdk, "upstream", "emscripten", "cmake", "Modules", "Platform", "Emscripten.cmake"),
		) {
			return readiness{skip: "emsdk is not installed: set `EMSDK`", env: nil, run: false}
		}

		_, err := exec.LookPath("node")

		return readiness{skip: "", env: nil, run: err == nil}
	case "wasi":
		sdk := os.Getenv("WASI_SDK_PATH")
		if !fileExists(filepath.Join(sdk, "share", "cmake", "wasi-sdk-p1.cmake")) {
			return readiness{skip: "wasi-sdk is not installed: set `WASI_SDK_PATH`", env: nil, run: false}
		}

		_, err := exec.LookPath("wasmtime")

		return readiness{skip: "", env: nil, run: err == nil}
	}

	return none
}

// platformSteps lists the commands that build the platform's preset.
func platformSteps(t *testing.T, root, platform string, ready readiness) [][]string {
	t.Helper()

	steps := [][]string{
		{"cmake", "--preset", platform},
		{"cmake", "--build", "--preset", platform},
	}

	if ready.run && strings.Contains(bedrocktest.Read(t, root, "cmake/presets/"+platform+".json"), "testPresets") {
		steps = append(steps, []string{"ctest", "--preset", platform})
	}

	return steps
}

//nolint:paralleltest // each build takes a toolchain's whole attention, so the matrix runs one preset at a time
func TestEndToEndEachPlatformPresetConfiguresAndBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a project with the real toolchain")
	}

	for kind, platforms := range cppSpec(t).Platforms {
		for _, platform := range platforms {
			t.Run(kind+"@"+platform, func(t *testing.T) {
				ready := platformReadiness(platform)
				if ready.skip != "" {
					t.Skip(ready.skip)
				}

				// Arrange
				root := initKind(t, kind, platform)

				// Act
				for _, args := range platformSteps(t, root, platform, ready) {
					ok, output := runIn(t, root, ready.env, args[0], args[1:]...)

					// Assert
					if !ok {
						t.Fatalf("%s failed:\n%s", strings.Join(args, " "), output)
					}
				}
			})
		}
	}
}
