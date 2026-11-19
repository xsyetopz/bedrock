package dotnet

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

const (
	// dotnetMajor is the .NET release every adopted repo builds on: the
	// current LTS.
	dotnetMajor = 10
	// sdkVersion is the SDK `global.json` pins; `latestFeature` rolls forward
	// to any later 10.0 band and patch.
	sdkVersion = "10.0.100"
	// globalJSON is the file that pins the SDK a repo builds with.
	globalJSON = "global.json"
	// testingPlatform is the `global.json` `test.runner` that opts
	// `dotnet test` into Microsoft.Testing.Platform.
	testingPlatform = "Microsoft.Testing.Platform"
	// defaultManifest is the tool manifest bedrock writes.
	defaultManifest = ".config/dotnet-tools.json"
	rollForward     = "rollForward"
)

// pinnedTool is a `dotnet` local tool the recipes run, at its NuGet version.
type pinnedTool struct {
	id      string
	version string
	command string
}

// tools are the pinned tools, in the order the manifest lists them.
func tools() []pinnedTool {
	return []pinnedTool{
		{id: csharpier, version: "1.4.0", command: csharpier},
		{id: fantomas, version: "8.1.3", command: fantomas},
		{id: "dotnet-fsharplint", version: "0.27.2", command: "dotnet-fsharplint"},
	}
}

// toolManifests are the places `dotnet tool restore` reads a local tool
// manifest from at the root: `.config/` first, where bedrock writes one, and
// the root itself, where `dotnet new tool-manifest` puts one since .NET 10.
func toolManifests() []string {
	return []string{defaultManifest, "dotnet-tools.json"}
}

// toolManifestPath is the tool manifest a repo already has, or the
// `.config/` one bedrock writes.
func toolManifestPath(root string) string {
	for _, path := range toolManifests() {
		if fsutil.Exists(filepath.Join(root, filepath.FromSlash(path))) {
			return path
		}
	}

	return defaultManifest
}

// readObject returns the JSONC object at path under root, or nil when the
// file is missing.
func readObject(root, path string) (*object, error) {
	text, found, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil || !found {
		return nil, err
	}

	parsed, err := parseObject(text)
	if err != nil {
		return nil, fmt.Errorf("read `%s`: %w", path, err)
	}

	return parsed, nil
}

// majorOf is the major version of an SDK version string such as `8.0.100`,
// or false.
func majorOf(version any) (int, bool) {
	text, ok := version.(string)
	if !ok {
		return 0, false
	}

	match := regexp.MustCompile(`^(\d+)\.`).FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}

	major, err := strconv.Atoi(match[1])

	return major, err == nil
}

// globalJSONStep is `global.json` pinning the .NET 10 SDK, and with
// onTestingPlatform the Microsoft.Testing.Platform runner, which the .NET 10
// SDK needs to run such a test project at all. A repo's own file keeps its
// other keys, such as `msbuild-sdks`; only an `sdk` older than 10 moves, and
// a newer one stays the project's choice.
func globalJSONStep(root string, onTestingPlatform bool) ([]step.Step, error) {
	current, err := readObject(root, globalJSON)
	if err != nil {
		return nil, err
	}

	exists := current != nil
	if !exists {
		current = newObject()
	}

	sdk := current.section("sdk")
	test := current.section("test")
	major, hasMajor := majorOf(sdk.get("version"))
	sdkPinned := hasMajor && major >= dotnetMajor
	runnerSet := !onTestingPlatform || test.get("runner") == testingPlatform

	if exists && sdkPinned && runnerSet {
		return nil, nil
	}

	next := current.clone()

	if sdkPinned {
		next.set("sdk", sdk)
	} else {
		pinned := sdk.clone()
		pinned.set("version", sdkVersion)
		pinned.set(rollForward, "latestFeature")
		next.set("sdk", pinned)
	}

	if !runnerSet {
		runner := test.clone()
		runner.set("runner", testingPlatform)
		next.set("test", runner)
	}

	content, err := marshal(next)
	if err != nil {
		return nil, err
	}

	return []step.Step{step.Write{Path: globalJSON, Content: content, Executable: false}}, nil
}

// pinned reports whether the manifest's tools already sit at the pins.
// `dotnet tool install` rewrites the manifest in its own layout, so a tool
// counts by its fields, not by the text.
func pinned(manifestTools *object) bool {
	for _, tool := range tools() {
		entry := manifestTools.section(tool.id)

		commands, _ := entry.get("commands").([]any)
		roll, rollIsBool := entry.get(rollForward).(bool)

		if entry.get("version") != tool.version || len(commands) != 1 || commands[0] != tool.command ||
			!rollIsBool || roll {
			return false
		}
	}

	return true
}

// toolManifestStep is the repo's tool manifest, or a new
// `.config/dotnet-tools.json`, with each pinned tool at its version. Tools
// the project added stay; bedrock's own move to its pins, so
// `dotnet tool restore` installs what the recipes run.
func toolManifestStep(root string) ([]step.Step, error) {
	path := toolManifestPath(root)

	current, err := readObject(root, path)
	if err != nil {
		return nil, err
	}

	exists := current != nil
	if !exists {
		current = newObject()
	}

	manifestTools := current.section("tools").clone()
	if exists && pinned(manifestTools) {
		return nil, nil
	}

	for _, tool := range tools() {
		entry := manifestTools.section(tool.id).clone()
		entry.set("version", tool.version)
		entry.set("commands", []any{tool.command})
		entry.set(rollForward, false)
		manifestTools.set(tool.id, entry)
	}

	next := newObject()
	next.set("version", json.Number("1"))
	next.set("isRoot", true)

	for _, key := range current.keys {
		next.set(key, current.get(key))
	}

	next.set("tools", manifestTools)

	content, err := marshal(next)
	if err != nil {
		return nil, err
	}

	return []step.Step{step.Write{Path: path, Content: content, Executable: false}}, nil
}

// retargetPattern matches a literal target framework property with the
// given tag. A value with `$` is an MSBuild property and stays.
func retargetPattern(tag string) *regexp.Regexp {
	return regexp.MustCompile(`<` + tag + `>([^<$]*)</` + tag + `>`)
}

// retarget moves one target framework to .NET 10: `net5.0` through `net9.0`
// and every `netcoreapp`, keeping a platform suffix such as `-windows`.
// `netstandard` and .NET Framework targets, such as `net48`, stay, since a
// library keeps them for callers on older runtimes.
func retarget(framework string) string {
	framework = strings.TrimSpace(framework)

	match := regexp.MustCompile(`^net(coreapp)?(\d+)\.(\d+)(-.+)?$`).FindStringSubmatch(framework)
	if match == nil {
		return framework
	}

	major, err := strconv.Atoi(match[2])
	if match[1] == "" && (err != nil || major >= dotnetMajor) {
		return framework
	}

	return "net" + strconv.Itoa(dotnetMajor) + ".0" + match[4]
}

// retargetProject is text with each literal TargetFramework and
// TargetFrameworks list retargeted; an MSBuild property stays.
func retargetProject(text string) string {
	for _, tag := range []string{"TargetFramework", "TargetFrameworks"} {
		text = retargetPattern(tag).ReplaceAllStringFunc(text, func(whole string) string {
			list := retargetPattern(tag).FindStringSubmatch(whole)[1]

			var frameworks []string

			for item := range strings.SplitSeq(list, ";") {
				if strings.TrimSpace(item) == "" {
					continue
				}

				if next := retarget(item); !slices.Contains(frameworks, next) {
					frameworks = append(frameworks, next)
				}
			}

			return "<" + tag + ">" + strings.Join(frameworks, ";") + "</" + tag + ">"
		})
	}

	return text
}

// retargetSteps rewrites the project and props files whose target
// frameworks move to .NET 10.
func retargetSteps(ctx context.Context, c *language.Context) ([]step.Step, error) {
	paths, err := listMatching(ctx, c, "**/{*.csproj,*.fsproj,Directory.Build.props}")
	if err != nil {
		return nil, err
	}

	var steps []step.Step

	for _, path := range paths {
		text, found, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(path)))
		if readErr != nil {
			return nil, readErr
		}

		if next := retargetProject(text); found && next != text {
			steps = append(steps, step.Write{Path: path, Content: []byte(next), Executable: false})
		}
	}

	return steps, nil
}

// langVersionNotes notes each project that pins a C# `LangVersion` older
// than the one .NET 10 defaults to, since editorconfig rules such as
// `csharp_prefer_system_threading_lock` need C# 13 and later.
func langVersionNotes(ctx context.Context, c *language.Context) ([]string, error) {
	paths, err := listMatching(ctx, c, "**/{*.csproj,Directory.Build.props}")
	if err != nil {
		return nil, err
	}

	pattern := regexp.MustCompile(`<LangVersion>\s*(\d+)(?:\.\d+)?\s*</LangVersion>`)

	var notes []string

	for _, path := range paths {
		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(path)))
		if readErr != nil {
			return nil, readErr
		}

		match := pattern.FindStringSubmatch(text)
		if match == nil {
			continue
		}

		if version, convErr := strconv.Atoi(match[1]); convErr == nil && version < csharpDefault {
			notes = append(notes, "`"+path+"`: `LangVersion` "+match[1]+" holds C# below the C# "+
				strconv.Itoa(
					csharpDefault,
				)+" that .NET 10 defaults to; delete it unless a consumer needs the older language.")
		}
	}

	return notes, nil
}

// csharpDefault is the C# version .NET 10 defaults to.
const csharpDefault = 14
