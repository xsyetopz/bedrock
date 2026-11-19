package dotnet

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

const (
	workflow     = ".github/workflows/dotnet.yml"
	props        = "Directory.Build.props"
	managedMark  = "# Managed by bedrock."
	workflowGlob = ".github/workflows/*.{yml,yaml}"
)

// Plan is the .NET part of an adopt: `global.json` on the .NET 10 SDK, and
// on Microsoft.Testing.Platform when every test project needs it, the pinned
// formatter and linter tools, and each project moved to `net10.0`, with notes
// for what a person decides.
func (Dotnet) Plan(ctx context.Context, c *language.Context) (language.Contribution, error) {
	if err := checkKinds(c); err != nil {
		return language.Contribution{}, err
	}

	tests, err := testProjects(ctx, c)
	if err != nil {
		return language.Contribution{}, err
	}

	current, _, err := fsutil.ReadText(filepath.Join(c.Root, globalJSON))
	if err != nil {
		return language.Contribution{}, err
	}

	onlyPlatform := slices.ContainsFunc(tests, isRunner(runnerMTP)) &&
		!slices.ContainsFunc(tests, isRunner(runnerVSTest))
	platform := strings.Contains(current, testingPlatform)

	steps, err := planSteps(ctx, c, onlyPlatform)
	if err != nil {
		return language.Contribution{}, err
	}

	notes, err := planNotes(ctx, c, tests, platform)
	if err != nil {
		return language.Contribution{}, err
	}

	projectFiles := []string{globalJSON}

	for _, s := range steps {
		if write, ok := s.(step.Write); ok {
			projectFiles = append(projectFiles, write.Path)
		}
	}

	return language.Contribution{Steps: steps, Notes: notes, ProjectFiles: projectFiles}, nil
}

func isRunner(want runner) func(testProject) bool {
	return func(test testProject) bool { return test.runner == want }
}

func planSteps(ctx context.Context, c *language.Context, onlyPlatform bool) ([]step.Step, error) {
	global, err := globalJSONStep(c.Root, onlyPlatform)
	if err != nil {
		return nil, err
	}

	manifest, err := toolManifestStep(c.Root)
	if err != nil {
		return nil, err
	}

	retargets, err := retargetSteps(ctx, c)
	if err != nil {
		return nil, err
	}

	return slices.Concat(global, manifest, retargets), nil
}

func planNotes(ctx context.Context, c *language.Context, tests []testProject, platform bool) ([]string, error) {
	replaced, err := workflowNote(c.Root)
	if err != nil {
		return nil, err
	}

	duplicates, err := duplicateWorkflowNotes(ctx, c)
	if err != nil {
		return nil, err
	}

	langs, err := langVersionNotes(ctx, c)
	if err != nil {
		return nil, err
	}

	text, found, err := fsutil.ReadText(filepath.Join(c.Root, props))
	if err != nil {
		return nil, err
	}

	var propsNotes []string
	if found {
		propsNotes = propsNote(text)
	}

	return slices.Concat(replaced, duplicates, langs, propsNotes, runnerNote(tests, platform)), nil
}

// workflowNote notes a .NET workflow bedrock did not write that adopt
// replaces, since its steps, such as packing or publishing, are the
// project's and a person moves them.
func workflowNote(root string) ([]string, error) {
	text, found, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(workflow)))
	if err != nil || !found || strings.HasPrefix(text, managedMark) {
		return nil, err
	}

	return []string{"`" + workflow + "`: replaced with bedrock's; git has yours. Move the steps it still needs into " +
		"`.github/actions/project-setup/action.yml` and the recipes it ran into `project.just`."}, nil
}

// duplicateWorkflowNotes notes each other workflow that sets up .NET or runs
// `dotnet`, such as a hand-written `ci.yml`: bedrock's .NET workflow now runs
// those checks, on the SDK `global.json` pins, so the old steps duplicate
// them on another SDK.
func duplicateWorkflowNotes(ctx context.Context, c *language.Context) ([]string, error) {
	paths, err := listMatching(ctx, c, workflowGlob)
	if err != nil {
		return nil, err
	}

	runs := regexp.MustCompile(`\bdotnet\s+\w`)

	var notes []string

	for _, path := range paths {
		if path == workflow {
			continue
		}

		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(path)))
		if readErr != nil {
			return nil, readErr
		}

		if (strings.Contains(text, "actions/setup-dotnet") || runs.MatchString(text)) &&
			!strings.HasPrefix(text, managedMark) {
			notes = append(notes, "`"+path+"`: sets up .NET or runs `dotnet`, which `"+workflow+
				"` now does on the SDK `global.json` pins; remove the steps it duplicates.")
		}
	}

	return notes, nil
}

// propsSetting is a setting bedrock's starter `Directory.Build.props` holds
// with the pattern that shows a project's own file sets it. The lint recipe
// and the editorconfig rules lean on them: IDE0005 needs the documentation
// file.
type propsSetting struct {
	pattern *regexp.Regexp
	text    string
}

func propsSettings() []propsSetting {
	return []propsSetting{
		{regexp.MustCompile(`<Nullable>\s*enable\s*</Nullable>`), "<Nullable>enable</Nullable>"},
		{
			regexp.MustCompile(`<AnalysisLevel>\s*latest-(?:recommended|all)\s*</AnalysisLevel>`),
			"<AnalysisLevel>latest-recommended</AnalysisLevel>",
		},
		{
			regexp.MustCompile(`<TreatWarningsAsErrors>\s*true\s*</TreatWarningsAsErrors>`),
			"<TreatWarningsAsErrors>true</TreatWarningsAsErrors>",
		},
		{
			regexp.MustCompile(`<GenerateDocumentationFile>\s*true\s*</GenerateDocumentationFile>`),
			"<GenerateDocumentationFile>true</GenerateDocumentationFile>",
		},
	}
}

// propsNote names the settings a project's own `Directory.Build.props`
// lacks, since the starter is written only where none exists, and the file
// is the project's to change.
func propsNote(text string) []string {
	var missing []string

	for _, setting := range propsSettings() {
		if !setting.pattern.MatchString(text) {
			missing = append(missing, "`"+setting.text+"`")
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return []string{"`" + props + "`: kept as yours; bedrock's starter also sets " + strings.Join(missing, ", ") +
		", which the lint build and the `.editorconfig` rules rely on."}
}

// runnerNote notes test projects that need both runners, since `dotnet test`
// on the .NET 10 SDK runs one per repo: a Microsoft.Testing.Platform project
// fails in VSTest mode, and a VSTest project fails in the other. Moving the
// VSTest projects over, such as xUnit v2 to `xunit.v3` or NUnit to
// `EnableNUnitRunner`, is the project's call.
func runnerNote(tests []testProject, platform bool) []string {
	names := func(want runner) string {
		var paths []string

		for _, test := range tests {
			if test.runner == want {
				paths = append(paths, test.path)
			}
		}

		return strings.Join(paths, ", ")
	}

	vstest, mtp := names(runnerVSTest), names(runnerMTP)

	switch {
	case vstest != "" && mtp != "":
		return []string{"`" + globalJSON + "`: " + mtp + " run on Microsoft.Testing.Platform and " + vstest +
			" on VSTest; `dotnet test` runs one, so move the VSTest projects over and set `test.runner` to `" +
			testingPlatform + "`."}
	case vstest != "" && platform:
		return []string{"`" + globalJSON + "`: `test.runner` is `" + testingPlatform + "`, but " + vstest +
			" run on VSTest; move them over or drop `test.runner`."}
	default:
		return nil
	}
}
