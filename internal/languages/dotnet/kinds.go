package dotnet

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/template"
)

const (
	kindApple    = "apple"
	kindAvalonia = "avalonia"
)

// errNoApplePlatform rejects an `apple` init that names no Apple platform,
// since each platform is its own app project and `apple` has no host default.
var errNoApplePlatform = errors.New(
	"the apple kind needs a platform, such as `bedrock init dotnet:apple@ios,macos`")

// kinds are what `bedrock init` can start. Each is a preset from the .NET
// ecosystem's own templates (`dotnet new`, Avalonia.Templates), laid out as
// `src/<Project>.<Kind>` with a `tests/<Project>.<Kind>.Tests` project so
// several kinds can share a repo.
func kinds() []string {
	return []string{"android", kindApple, kindAvalonia, "console", "library", "webapi", "worker"}
}

// platforms maps the kinds that take `@<platform>` to the platforms they
// build for. `avalonia` targets the desktop (linux, macos, windows share one
// project), Android, iOS, and the browser; `apple` is native Apple code, one
// app project per platform; `android` is Android alone.
func platforms() map[string][]string {
	return map[string][]string{
		kindAvalonia: {"android", "ios", "linux", "macos", "wasm", "windows"},
		kindApple:    {"ios", "maccatalyst", "macos", "tvos"},
	}
}

// Variables fill the placeholders of the kind templates: `project`, the
// directory name as a C# namespace, and `appId`, a reverse-DNS application
// identifier for the mobile and Apple apps.
func (Dotnet) Variables(_ context.Context, c *language.Context) (template.Variables, error) {
	project := projectName(filepath.Base(c.Root))

	return template.Variables{"project": project, "appId": "com.example." + strings.ToLower(project)}, nil
}

// projectName turns a directory name into a PascalCase identifier that is a
// valid namespace, `App` when nothing usable is left, and prefixed when it
// would start with a digit.
func projectName(dir string) string {
	var name strings.Builder

	for _, word := range regexp.MustCompile(`[^\p{L}\p{N}]+`).Split(dir, -1) {
		for i, r := range word {
			if i == 0 {
				r = unicode.ToUpper(r)
			}

			name.WriteRune(r)
		}
	}

	switch result := name.String(); {
	case result == "":
		return "App"
	case unicode.IsDigit([]rune(result)[0]):
		return "App" + result
	default:
		return result
	}
}

// checkKinds rejects a kind selection the templates cannot build.
func checkKinds(c *language.Context) error {
	if !c.Init {
		return nil
	}

	kindList, err := config.SplitList(c.Options["kind"])
	if err != nil {
		return err
	}

	platformList, err := config.SplitList(c.Options["platform"])
	if err != nil {
		return err
	}

	for _, kind := range kindList {
		if kind == kindApple && len(platformList) == 0 {
			return errNoApplePlatform
		}
	}

	return nil
}
