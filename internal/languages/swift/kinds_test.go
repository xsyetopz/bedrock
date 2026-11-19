package swift_test

import (
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/languages/swift"
)

const appProject = "Starter.xcodeproj/project.pbxproj"

// appOptions are the options of an app init for the comma-separated platforms.
func appOptions(platforms string) config.Options {
	options := config.Options{"kind": "app"}
	if platforms != "" {
		options["platform"] = platforms
	}

	return options
}

// initProject inits an app in a fresh repository and returns its project file.
func initProject(t *testing.T, options config.Options) string {
	t.Helper()

	root := namedRepo(t, "Starter")
	bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", options))

	return bedrocktest.Read(t, root, appProject)
}

func TestInitAppBuildsForEachPlatform(t *testing.T) {
	t.Parallel()

	for platform, want := range map[string][]string{
		"ios":      {"SDKROOT = iphoneos;", "IPHONEOS_DEPLOYMENT_TARGET = 15.0;", `TARGETED_DEVICE_FAMILY = "1,2";`},
		"tvos":     {"SDKROOT = appletvos;", "TVOS_DEPLOYMENT_TARGET = 15.0;", `TARGETED_DEVICE_FAMILY = "3";`},
		"visionos": {"SDKROOT = xros;", "XROS_DEPLOYMENT_TARGET = 1.0;", `TARGETED_DEVICE_FAMILY = "7";`},
		"watchos":  {"SDKROOT = watchos;", "WATCHOS_DEPLOYMENT_TARGET = 9.0;", "INFOPLIST_KEY_WKApplication = YES;"},
		"macos":    {"SDKROOT = macosx;", "INFOPLIST_KEY_NSPrincipalClass = NSApplication;"},
	} {
		t.Run(platform, func(t *testing.T) {
			t.Parallel()

			// Arrange
			options := appOptions(platform)

			// Act
			project := initProject(t, options)

			// Assert
			for _, line := range want {
				if !strings.Contains(project, line) {
					t.Errorf("%s project lacks %q", platform, line)
				}
			}

			if strings.Contains(project, "@@") {
				t.Errorf("%s project keeps an unfilled placeholder", platform)
			}
		})
	}
}

func TestInitAppWithoutPlatformsIsTheMacOSApp(t *testing.T) {
	t.Parallel()

	// Arrange
	plain := initProject(t, appOptions(""))

	// Act
	named := initProject(t, appOptions("macos"))

	// Assert
	if plain != named {
		t.Error("an app with no platform differs from `app@macos`")
	}

	for _, other := range []string{"iphoneos", "SUPPORTED_PLATFORMS", "DEPLOYMENT_TARGET"} {
		if strings.Contains(plain, other) {
			t.Errorf("the macOS app names %s", other)
		}
	}
}

func TestInitAppWithSeveralPlatformsSharesOneTarget(t *testing.T) {
	t.Parallel()

	// Arrange
	options := appOptions("ios,macos")

	// Act
	project := initProject(t, options)

	// Assert
	for _, want := range []string{
		"SDKROOT = auto;",
		`SUPPORTED_PLATFORMS = "iphoneos iphonesimulator macosx";`,
		`"INFOPLIST_KEY_NSPrincipalClass[sdk=macosx*]" = NSApplication;`,
		`"INFOPLIST_KEY_UILaunchScreen_Generation[sdk=iphone*]" = YES;`,
	} {
		if !strings.Contains(project, want) {
			t.Errorf("project lacks %q", want)
		}
	}

	const target = "isa = PBXNativeTarget;"
	if strings.Count(project, target) != strings.Count(initProject(t, appOptions("")), target) {
		t.Error("several platforms add targets; they share the app target")
	}
}

func TestInitAppSecondAdoptPlansNothingForEachPlatform(t *testing.T) {
	t.Parallel()

	for _, platforms := range []string{"ios", "tvos", "visionos", "watchos", "macos", "ios,macos"} {
		t.Run(platforms, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := namedRepo(t, "Starter")
			selection := bedrocktest.Select(t, "swift", appOptions(platforms))
			bedrocktest.Adopt(t, root, true, selection)

			// Act
			again := bedrocktest.Plan(t, root, true, selection)

			// Assert
			if len(again.Steps) != 0 {
				t.Errorf("second init plans %v", bedrocktest.Paths(again.Steps))
			}
		})
	}
}

func TestInitAppRejectsAPlatformItDoesNotBuildFor(t *testing.T) {
	t.Parallel()

	// Arrange
	root := namedRepo(t, "Starter")
	c := bedrocktest.Context(t, root, bedrocktest.Select(t, "swift", nil))
	c.Init = true
	c.Options = appOptions("android")

	// Act
	_, err := swift.New().Plan(t.Context(), c)

	// Assert
	if err == nil || !strings.Contains(err.Error(), `platform "android"`) {
		t.Errorf("error = %v, want one that names the platform", err)
	}
}

func TestInitRejectsAnExecutableAndAToolTogether(t *testing.T) {
	t.Parallel()

	// Arrange
	root := namedRepo(t, "Starter")
	c := bedrocktest.Context(t, root, bedrocktest.Select(t, "swift", nil))
	c.Init = true
	c.Options = config.Options{"kind": "executable,tool"}

	// Act
	_, err := swift.New().Plan(t.Context(), c)

	// Assert
	if err == nil {
		t.Error("init accepts an executable and a tool, which both own the `<Name>CLI` target")
	}
}

func TestInitPackageKindsStayCrossPlatform(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"library", "executable", "tool", "macro", "build-tool-plugin", "command-plugin"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := namedRepo(t, "Starter")

			// Act
			bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", swiftOptions(kind)))

			// Assert
			manifest := bedrocktest.Read(t, root, "Package.swift")
			for _, apple := range []string{".iOS(", ".tvOS(", ".watchOS(", ".visionOS(", ".macCatalyst("} {
				if strings.Contains(manifest, apple) {
					t.Errorf("%s manifest restricts the package to Apple platforms with %s", kind, apple)
				}
			}
		})
	}
}

func TestInitCombinesPackageKinds(t *testing.T) {
	t.Parallel()

	// Arrange
	root := namedRepo(t, "Starter")
	options := swiftOptions("library,tool,macro,command-plugin,build-tool-plugin")

	// Act
	bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "swift", options))

	// Assert
	manifest := bedrocktest.Read(t, root, "Package.swift")
	for _, want := range []string{
		`"StarterCLI"`, `"StarterMacros"`, `"StarterClient"`,
		`"StarterCommandPlugin"`, `"StarterBuildToolPlugin"`, "CompilerPluginSupport",
	} {
		if !strings.Contains(manifest, want) {
			t.Errorf("manifest lacks %s:\n%s", want, manifest)
		}
	}

	for line := range strings.SplitSeq(manifest, "\n") {
		if len(line) > 100 {
			t.Errorf("manifest line exceeds the swift-format line length: %q", line)
		}
	}
}
