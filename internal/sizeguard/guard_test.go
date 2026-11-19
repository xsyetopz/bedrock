package sizeguard_test

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/sizeguard"
)

// sized is a test-only language whose sources the guard bounds.
type sized struct {
	lizard []string
	tests  []string
}

func (s sized) Spec() language.Spec {
	return language.Spec{ID: "sized", Lizard: s.lizard}
}

// TestFiles treats each file whose name ends in a suffix of tests as a test.
func (s sized) TestFiles(_ context.Context, _ *language.Context, files []string) (map[string]bool, error) {
	found := map[string]bool{}

	for _, file := range files {
		for _, suffix := range s.tests {
			if strings.HasSuffix(file, suffix) {
				found[file] = true
			}
		}
	}

	return found, nil
}

// measured is one function a fake lizard reports.
type measured struct {
	path   string
	name   string
	line   int
	nloc   int
	ccn    int
	params int
}

// fakeLizard stands in for the lizard program, answering with the
// measurements it was given in lizard's own csv and xml formats.
type fakeLizard struct {
	functions []measured
	files     map[string]int
	missing   bool
}

func (f fakeLizard) LookPath(name string) bool {
	return !f.missing && name == "lizard"
}

func (f fakeLizard) Run(_ context.Context, command execx.Command) (execx.Result, error) {
	switch {
	case slices.Contains(command.Args, "--version"):
		return execx.Result{Stdout: []byte(sizeguard.LizardVersion + "\n")}, nil
	case slices.Contains(command.Args, "--csv"):
		return execx.Result{Stdout: []byte(f.csv())}, nil
	default:
		return execx.Result{Stdout: []byte(f.xml())}, nil
	}
}

func (f fakeLizard) csv() string {
	var out strings.Builder

	for _, fn := range f.functions {
		fmt.Fprintf(&out, "%d,%d,10,%d,5,\"%s@%d-9@./%s\",\"./%s\",\"%s\",\"%s x\",%d,9\n",
			fn.nloc, fn.ccn, fn.params, fn.name, fn.line, fn.path, fn.path, fn.name, fn.name, fn.line)
	}

	return out.String()
}

func (f fakeLizard) xml() string {
	var out strings.Builder

	out.WriteString("<cppncss>\n<measure type=\"File\">\n")

	for path, nloc := range f.files {
		fmt.Fprintf(&out, "<item name=\"./%s\"><value>1</value><value>%d</value><value>1</value></item>\n", path, nloc)
	}

	out.WriteString("</measure>\n</cppncss>\n")

	return out.String()
}

const bounds = `root = true

[*.{java,kt}]
bedrock_file_code_lines_warning = 603
bedrock_function_code_lines_error = 56
bedrock_function_complexity_error = 15
bedrock_function_parameters_error = 7

[*.rs]
bedrock_file_code_lines_warning = 603
bedrock_test_file_code_lines_warning = 1000
`

func guard(
	t *testing.T,
	runner execx.Runner,
	root string,
	lang sized,
) sizeguard.Report {
	t.Helper()

	engine := &plan.Engine{Runner: runner, Stdout: io.Discard, Stderr: io.Discard}

	report, err := sizeguard.Guard(t.Context(), engine, root, []plan.Selected{{
		Language: lang,
		Options:  config.Options{},
	}})
	if err != nil {
		t.Fatal(err)
	}

	return report
}

func expectLines(t *testing.T, what string, got, want []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

func TestGuardFailsOverLongFunctionsAndOnlyWarnsAboutLongFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{".editorconfig": bounds})
	lizard := fakeLizard{
		files: map[string]int{"src/Big.kt": 700, "src/Small.kt": 20},
		functions: []measured{
			{path: "src/Big.kt", line: 9, name: "run", nloc: 80, ccn: 3, params: 8},
			{path: "src/Small.kt", line: 1, name: "ok", nloc: 5, ccn: 1, params: 1},
		},
	}

	// Act
	report := guard(t, lizard, root, sized{lizard: []string{"kotlin"}})

	// Assert
	expectLines(t, "errors", report.Errors, []string{
		"src/Big.kt:9: `run` code lines is 80, over 56",
		"src/Big.kt:9: `run` parameters is 8, over 7",
	})
	expectLines(t, "warnings", report.Warnings, []string{"src/Big.kt: file code lines is 700, over 603"})
}

func TestGuardBoundsATestFileByTheTestFileBound(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{".editorconfig": bounds})
	lizard := fakeLizard{files: map[string]int{
		"src/lib.rs":       700,
		"src/a/checks.rs":  700,
		"src/b/checks.rs":  1200,
		"src/b/helpers.rs": 100,
	}}

	// Act
	report := guard(t, lizard, root, sized{lizard: []string{"rust"}, tests: []string{"checks.rs"}})

	// Assert
	slices.Sort(report.Warnings)
	expectLines(t, "warnings", report.Warnings, []string{
		"src/b/checks.rs: test file code lines is 1200, over 1000",
		"src/lib.rs: file code lines is 700, over 603",
	})
}

func TestGuardBoundsOnlyTheMetricsASectionSets(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{".editorconfig": bounds})
	lizard := fakeLizard{
		files:     map[string]int{"src/lib.rs": 700},
		functions: []measured{{path: "src/lib.rs", line: 3, name: "run", nloc: 90, ccn: 30, params: 9}},
	}

	// Act
	report := guard(t, lizard, root, sized{lizard: []string{"rust"}})

	// Assert
	expectLines(t, "errors", report.Errors, nil)
	expectLines(t, "warnings", report.Warnings, []string{"src/lib.rs: file code lines is 700, over 603"})
}

func TestGuardLetsANestedEditorconfigUnsetABound(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig":        bounds,
		"legacy/.editorconfig": "[*.kt]\nbedrock_function_code_lines_error = unset\n",
	})
	lizard := fakeLizard{functions: []measured{
		{path: "src/A.kt", line: 2, name: "a", nloc: 80},
		{path: "legacy/B.kt", line: 2, name: "b", nloc: 80},
	}}

	// Act
	report := guard(t, lizard, root, sized{lizard: []string{"kotlin"}})

	// Assert
	expectLines(t, "errors", report.Errors, []string{"src/A.kt:2: `a` code lines is 80, over 56"})
}

func TestGuardReportsInvalidBoundsAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig": "root = true\n\n[*.kt]\n" +
			"bedrock_function_code_lines_error = 10\n" +
			"bedrock_file_code_lines_warning = lots\n" +
			"bedrock_function_parameters_error = 0\n" +
			"bedrock_function_lines_error = 56\n" +
			"bedrock_function_complexity_error = unset\n",
	})
	lizard := fakeLizard{functions: []measured{{path: "A.kt", line: 1, name: "a", nloc: 80}}}

	// Act
	report := guard(t, lizard, root, sized{lizard: []string{"kotlin"}})

	// Assert
	expectLines(t, "errors", report.Errors, []string{
		".editorconfig: `bedrock_file_code_lines_warning` = lots; a bound is a positive integer or `unset`",
		".editorconfig: `bedrock_function_parameters_error` = 0; a bound is a positive integer or `unset`",
		".editorconfig: `bedrock_function_lines_error` names no bound; bedrock bounds `file_code_lines`, " +
			"`test_file_code_lines`, `function_code_lines`, `function_complexity`, `function_parameters`, " +
			"each with `_warning` or `_error`",
	})
	expectLines(t, "warnings", report.Warnings, nil)
}

func TestGuardReportsWhenNoLizardRunnerIsInstalled(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{".editorconfig": bounds})

	// Act
	report := guard(t, fakeLizard{missing: true}, root, sized{lizard: []string{"kotlin", "java"}})

	// Assert
	want := "`lizard` " + sizeguard.LizardVersion +
		" cannot run: install `uv` or `pipx` (`just setup` does), so bedrock can bound kotlin, java sizes"
	expectLines(t, "errors", report.Errors, []string{want})
}

func TestGuardSkipsALanguageWithNoSizeGap(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{".editorconfig": bounds})

	// Act
	report := guard(t, fakeLizard{missing: true}, root, sized{})

	// Assert
	expectLines(t, "errors", report.Errors, nil)
}

// realLizard skips the test unless a runner for the real lizard is installed.
func realLizard(t *testing.T) {
	t.Helper()

	for _, name := range []string{"lizard", "uvx", "pipx"} {
		if _, err := exec.LookPath(name); err == nil {
			return
		}
	}

	t.Skip("lizard, uvx, and pipx are all absent; run `just setup`")
}

func longMethod(name string) string {
	var body strings.Builder
	for i := range 60 {
		fmt.Fprintf(&body, "    int v%d = %d;\n", i, i)
	}

	return "class " + name + " {\n  void run() {\n" + body.String() + "  }\n}\n"
}

func rustFile(lines int, attribute string) string {
	var body strings.Builder
	for i := range lines {
		fmt.Fprintf(&body, "    let v%d = %d;\n", i, i)
	}

	return attribute + "\nfn run() {\n" + body.String() + "}\n"
}

func TestGuardMeasuresRealJavaWithLizard(t *testing.T) {
	t.Parallel()
	realLizard(t)

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig":        bounds,
		"src/A.java":           longMethod("A"),
		"legacy/.editorconfig": "[*.java]\nbedrock_function_code_lines_error = unset\n",
		"legacy/B.java":        longMethod("B"),
	})

	// Act
	report := guard(t, execx.System{Stdout: io.Discard, Stderr: io.Discard}, root, sized{lizard: []string{"java"}})

	// Assert
	if len(report.Errors) != 1 || !strings.HasPrefix(report.Errors[0], "src/A.java:2: ") {
		t.Errorf("errors = %q, want one starting with src/A.java:2", report.Errors)
	}
}

func TestGuardBoundsRealTestFilesByTheTestFileBound(t *testing.T) {
	t.Parallel()
	realLizard(t)

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig": "root = true\n\n[*.rs]\n" +
			"bedrock_file_code_lines_warning = 20\nbedrock_test_file_code_lines_warning = 50\n",
		"src/live.rs":   rustFile(30, "#[inline]"),
		"src/checks.rs": rustFile(30, "#[test]"),
	})

	// Act
	report := guard(t, execx.System{Stdout: io.Discard, Stderr: io.Discard}, root,
		sized{lizard: []string{"rust"}, tests: []string{"checks.rs"}})

	// Assert
	if len(report.Warnings) != 1 || !strings.HasPrefix(report.Warnings[0], "src/live.rs: file code lines") {
		t.Errorf("warnings = %q, want one for src/live.rs", report.Warnings)
	}
}
