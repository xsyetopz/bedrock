package swift

import (
	"fmt"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/template"
)

// Platform names the app kind switches on.
const (
	platformIOS     = "ios"
	platformMacOS   = "macos"
	platformWatchOS = "watchos"
)

// applePlatform is one platform an Xcode app target can build for.
type applePlatform struct {
	// name is the shared platform name, such as `ios`.
	name string
	// sdk is the SDKROOT of an app that builds for this platform alone.
	sdk string
	// sdkPattern is what a build setting condition matches, such as `iphone*`;
	// the device and simulator SDKs share a prefix.
	sdkPattern string
	// supported are the SUPPORTED_PLATFORMS values.
	supported []string
	// deployment is the deployment target setting and its floor: the oldest
	// release the installed SDK still deploys to.
	deployment, floor string
	// family is the TARGETED_DEVICE_FAMILY value, empty for macOS.
	family string
}

// applePlatforms lists the platforms of the `app` kind in the order the
// project settings name them.
func applePlatforms() []applePlatform {
	return []applePlatform{
		{
			name: platformIOS, sdk: "iphoneos", sdkPattern: "iphone*",
			supported:  []string{"iphoneos", "iphonesimulator"},
			deployment: "IPHONEOS_DEPLOYMENT_TARGET", floor: "15.0", family: "1,2",
		},
		{name: platformMacOS, sdk: "macosx", sdkPattern: "macosx*", supported: []string{"macosx"}},
		{
			name: "tvos", sdk: "appletvos", sdkPattern: "appletv*",
			supported:  []string{"appletvos", "appletvsimulator"},
			deployment: "TVOS_DEPLOYMENT_TARGET", floor: "15.0", family: "3",
		},
		{
			name: "visionos", sdk: "xros", sdkPattern: "xr*",
			supported:  []string{"xros", "xrsimulator"},
			deployment: "XROS_DEPLOYMENT_TARGET", floor: "1.0", family: "7",
		},
		{
			name: platformWatchOS, sdk: "watchos", sdkPattern: "watch*",
			supported:  []string{"watchos", "watchsimulator"},
			deployment: "WATCHOS_DEPLOYMENT_TARGET", floor: "9.0", family: "4",
		},
	}
}

// appPlatforms returns the platform names the `app` kind accepts.
func appPlatforms() []string {
	var names []string

	for _, p := range applePlatforms() {
		names = append(names, p.name)
	}

	return names
}

// chosenPlatforms returns the app's platforms from the `platform` option, in
// table order, macOS alone by default.
func chosenPlatforms(c *language.Context) ([]applePlatform, error) {
	named, err := config.SplitList(c.Options["platform"])
	if err != nil {
		return nil, fmt.Errorf("platform: %w", err)
	}

	if len(named) == 0 {
		named = []string{platformMacOS}
	}

	var chosen []applePlatform

	for _, p := range applePlatforms() {
		if slices.Contains(named, p.name) {
			chosen = append(chosen, p)
		}
	}

	for _, name := range named {
		if !slices.Contains(appPlatforms(), name) {
			return nil, fmt.Errorf("platform %q: the app kind builds for %s", name, strings.Join(appPlatforms(), ", "))
		}
	}

	return chosen, nil
}

// appVariables returns the project placeholders for the platforms. macOS
// alone leaves the project as it was before platforms existed. One other
// platform sets its SDK, as Xcode's platform App templates do; several set
// SDKROOT to `auto` and list the SDKs, as Xcode's Multiplatform App does.
func appVariables(chosen []applePlatform) template.Variables {
	multi := len(chosen) > 1
	vars := template.Variables{"sdkRoot": chosen[0].sdk, "projectSettings": ""}

	var project []string

	if multi {
		vars["sdkRoot"] = "auto"

		var sdks []string

		for _, p := range chosen {
			sdks = append(sdks, p.supported...)
		}

		slices.Sort(sdks)
		project = append(project, `SUPPORTED_PLATFORMS = "`+strings.Join(sdks, " ")+`";`, "SUPPORTS_MACCATALYST = NO;")
	}

	var families []string

	for _, p := range chosen {
		if p.family != "" {
			families = append(families, p.family)
		}

		if p.deployment != "" {
			project = append(project, p.deployment+" = "+p.floor+";")
		}
	}

	if len(families) > 0 {
		project = append(project, `TARGETED_DEVICE_FAMILY = "`+strings.Join(families, ",")+`";`)
	}

	slices.Sort(project)

	if len(project) > 0 {
		vars["projectSettings"] = "\n\t\t\t\t" + strings.Join(project, "\n\t\t\t\t")
	}

	vars["appSettings"] = strings.Join(appSettings(chosen, multi), "\n\t\t\t\t")

	return vars
}

// appSettings returns the app target's settings that differ by platform. A
// platform's setting is conditional on its SDK only when other platforms
// share the target.
func appSettings(chosen []applePlatform, multi bool) []string {
	var lines []string

	set := func(key, value string, p applePlatform) {
		if multi {
			key = `"` + key + "[sdk=" + p.sdkPattern + `]"`
		}

		lines = append(lines, key+" = "+value+";")
	}

	hasMac := false

	for _, p := range chosen {
		switch p.name {
		case platformMacOS:
			hasMac = true

			set("INFOPLIST_KEY_NSPrincipalClass", "NSApplication", p)
			set("LD_RUNPATH_SEARCH_PATHS", `"@executable_path/../Frameworks"`, p)
		case platformIOS:
			set("INFOPLIST_KEY_UIApplicationSceneManifest_Generation", "YES", p)
			set("INFOPLIST_KEY_UILaunchScreen_Generation", "YES", p)
		case "visionos":
			set("INFOPLIST_KEY_UIApplicationSceneManifest_Generation", "YES", p)
		case platformWatchOS:
			set("INFOPLIST_KEY_WKApplication", "YES", p)
		}
	}

	if !hasMac || multi {
		// Apps on every other platform load frameworks from `Frameworks`
		// beside the executable; with macOS in the mix, its condition
		// above overrides this default.
		lines = append(lines, `LD_RUNPATH_SEARCH_PATHS = "@executable_path/Frameworks";`)
	}

	slices.Sort(lines)

	return lines
}
