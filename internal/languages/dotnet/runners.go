package dotnet

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// runner is the test runner `dotnet test` must use for a test project: mtp
// for a Microsoft.Testing.Platform application, which the .NET 10 SDK
// refuses to run in VSTest mode even when it also carries a VSTest adapter,
// and vstest for the rest.
type runner string

const (
	runnerNone   runner = ""
	runnerMTP    runner = "mtp"
	runnerVSTest runner = "vstest"
)

// facts decide a project's runner: whether it builds on `MSTest.Sdk`, the
// NuGet packages it references, and which of the runner properties it sets
// to true, the last two lower-cased.
type facts struct {
	packages map[string]bool
	flags    map[string]bool
	mstest   bool
}

// mtpPackages make a project a Microsoft.Testing.Platform application:
// xUnit v3 on its MTP v1 or v2 runner, and TUnit, which runs on nothing else.
// `xunit.v3.mtp-off` and `TUnit.Core`, a library of attributes, do not.
func mtpPackages() []string {
	return []string{"xunit.v3", "xunit.v3.mtp-v1", "xunit.v3.mtp-v2", "tunit", "tunit.engine"}
}

// vstestPackages make a project a VSTest project: the test SDK, which sets
// `IsTestProject`, the MSTest metapackage, which brings it, and the VSTest
// adapters of xUnit, NUnit, MSTest, and Expecto.
func vstestPackages() []string {
	return []string{
		"microsoft.net.test.sdk",
		"mstest",
		"mstest.testadapter",
		"nunit3testadapter",
		"xunit.runner.visualstudio",
		"yolodev.expecto.testsdk",
	}
}

// runnerSwitches maps each runner switch to the packages whose build files
// act on it: the switch alone, as a `Directory.Build.props` sets it for
// every project, leaves a project without the adapter no test application.
func runnerSwitches() map[string][]string {
	return map[string][]string{
		"enablemstestrunner": {"mstest", "mstest.testadapter"},
		"enablenunitrunner":  {"nunit3testadapter"},
	}
}

// properties are every property facts record: the runner switches, the
// property the test frameworks set for Microsoft.Testing.Platform,
// `IsTestProject`, and `UseVSTest` of `MSTest.Sdk`.
// `UseMicrosoftTestingPlatformRunner` set to false is no way out: xUnit v3
// stays such an application, and VSTest mode still refuses it.
func properties() []string {
	return []string{
		"EnableMSTestRunner",
		"EnableNUnitRunner",
		"IsTestingPlatformApplication",
		"IsTestProject",
		"UseVSTest",
	}
}

// references reports whether the project references any of the packages.
func (f facts) references(names []string) bool {
	return slices.ContainsFunc(names, func(name string) bool { return f.packages[name] })
}

// runner is the runner the facts call for, or runnerNone for a project with
// no tests.
func (f facts) runner() runner {
	if f.mstest {
		if f.flags["usevstest"] {
			return runnerVSTest
		}

		return runnerMTP
	}

	switched := false

	for flag, adapters := range runnerSwitches() {
		switched = switched || (f.flags[flag] && f.references(adapters))
	}

	if f.references(mtpPackages()) || switched || f.flags["istestingplatformapplication"] {
		return runnerMTP
	}

	if f.references(vstestPackages()) || f.flags["istestproject"] {
		return runnerVSTest
	}

	return runnerNone
}

// textFacts are the facts a project file's own text states.
func textFacts(text string) facts {
	found := facts{packages: map[string]bool{}, flags: map[string]bool{}, mstest: false}

	found.mstest = regexp.MustCompile(`(?i)\bSdk="MSTest\.Sdk|<Sdk\b[^>]*\bName="MSTest\.Sdk"`).MatchString(text)

	reference := regexp.MustCompile(`(?i)<(?:Global)?PackageReference\b[^>]*?\bInclude="([^"]+)"`)
	for _, match := range reference.FindAllStringSubmatch(text, -1) {
		found.packages[strings.ToLower(match[1])] = true
	}

	for _, name := range properties() {
		if regexp.MustCompile(`(?i)<` + name + `>\s*true\s*</` + name + `>`).MatchString(text) {
			found.flags[strings.ToLower(name)] = true
		}
	}

	return found
}

// centralFiles are the files MSBuild imports around a project that can add a
// test package or set a runner property for it: the `Directory.Build.props`
// and `Directory.Build.targets` it finds walking up, and the
// `Directory.Packages.props` whose `GlobalPackageReference` reaches every
// project.
func centralFiles() []string {
	return []string{"Directory.Build.props", "Directory.Build.targets", "Directory.Packages.props"}
}

// centralTrigger matches text that shows a central file may change a
// project's runner.
func centralTrigger() *regexp.Regexp {
	names := slices.Concat(mtpPackages(), vstestPackages())
	for index, name := range names {
		names[index] = regexp.QuoteMeta(name)
	}

	return regexp.MustCompile(`(?i)Include="(?:` + strings.Join(names, "|") + `)"|<(?:` +
		strings.Join(properties(), "|") + `)>`)
}

// centralText is the text of the central files in the directories from the
// project's up to the root.
func centralText(root, project string) (string, error) {
	var texts []string

	for dir := path.Dir(project); ; dir = path.Dir(dir) {
		for _, name := range centralFiles() {
			text, _, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(dir), name))
			if err != nil {
				return "", err
			}

			texts = append(texts, text)
		}

		if dir == "." {
			break
		}
	}

	return strings.Join(texts, "\n"), nil
}

// evaluatedFacts are the facts MSBuild evaluates for a project, with the
// central files' conditions applied, such as a test package added only to
// projects named `*.Tests`. Evaluation needs no restore and writes nothing.
// It reports false when `dotnet` is missing or fails on the project.
func evaluatedFacts(ctx context.Context, c *language.Context, project, text string) (facts, bool) {
	args := []string{"msbuild", project, "-getItem:PackageReference"}
	for _, name := range append(properties(), "UsingMSTestSdk") {
		args = append(args, "-getProperty:"+name)
	}

	args = append(args, "-nodeReuse:false")

	result, err := c.Runner.Run(ctx, execx.Command{Dir: c.Root, Name: id, Args: args})
	if err != nil || !result.OK() {
		return facts{}, false
	}

	var parsed struct {
		Properties map[string]any `json:"Properties"`
		Items      struct {
			PackageReference []struct {
				Identity string `json:"Identity"`
			} `json:"PackageReference"`
		} `json:"Items"`
	}

	if json.Unmarshal(result.Stdout, &parsed) != nil {
		return facts{}, false
	}

	isTrue := func(name string) bool {
		value, ok := parsed.Properties[name].(string)

		return ok && strings.EqualFold(value, "true")
	}

	found := facts{packages: map[string]bool{}, flags: map[string]bool{}, mstest: false}
	found.mstest = isTrue("UsingMSTestSdk") || textFacts(text).mstest

	for _, item := range parsed.Items.PackageReference {
		found.packages[strings.ToLower(item.Identity)] = true
	}

	for _, name := range properties() {
		if isTrue(name) {
			found.flags[strings.ToLower(name)] = true
		}
	}

	return found, true
}

// projectRunner is the runner of one project: from its own text, or, when a
// central file around it names a test package or runner property, from
// MSBuild's evaluation, falling back to the text without `dotnet`.
func projectRunner(ctx context.Context, c *language.Context, project string) (runner, error) {
	text, _, err := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(project)))
	if err != nil {
		return runnerNone, err
	}

	central, err := centralText(c.Root, project)
	if err != nil {
		return runnerNone, err
	}

	if centralTrigger().MatchString(central) {
		if evaluated, ok := evaluatedFacts(ctx, c, project, text); ok {
			return evaluated.runner(), nil
		}
	}

	return textFacts(text).runner(), nil
}

// testProject is a project that needs a runner.
type testProject struct {
	path   string
	runner runner
}

// testProjects are the test projects under the root with the runner each
// needs, sorted by path. Evaluations run at most `jobs` at a time, since
// each starts MSBuild.
func testProjects(ctx context.Context, c *language.Context) ([]testProject, error) {
	projects, err := projectFiles(ctx, c)
	if err != nil {
		return nil, err
	}

	runners := make([]runner, len(projects))
	errs := make([]error, len(projects))
	slots := make(chan struct{}, jobs())

	var group sync.WaitGroup

	for index, project := range projects {
		group.Add(1)

		slots <- struct{}{}

		go func() {
			defer group.Done()
			defer func() { <-slots }()

			runners[index], errs[index] = projectRunner(ctx, c, project)
		}()
	}

	group.Wait()

	if joined := errors.Join(errs...); joined != nil {
		return nil, joined
	}

	var tests []testProject

	for index, project := range projects {
		if runners[index] != runnerNone {
			tests = append(tests, testProject{path: project, runner: runners[index]})
		}
	}

	return tests, nil
}

// TestFiles returns the files that a test project compiles: those under its
// directory, as the SDK's default `Compile` glob takes them. A file outside
// every test project stays product code.
func (Dotnet) TestFiles(ctx context.Context, c *language.Context, files []string) (map[string]bool, error) {
	tests, err := testProjects(ctx, c)
	if err != nil {
		return nil, err
	}

	found := map[string]bool{}

	for _, file := range files {
		for _, test := range tests {
			if dir := path.Dir(test.path); dir == "." || strings.HasPrefix(file, dir+"/") {
				found[file] = true

				break
			}
		}
	}

	return found, nil
}
