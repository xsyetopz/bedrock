package sizeguard_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
)

// hookCheck is a fake lizard that records, at each run, which files the
// PYTHONPATH entry bedrock puts first holds.
type hookCheck struct {
	fakeLizard

	mu     *sync.Mutex
	staged *[]string
}

func (h hookCheck) Run(ctx context.Context, command execx.Command) (execx.Result, error) {
	for _, pair := range command.Env {
		value, ok := strings.CutPrefix(pair, "PYTHONPATH=")
		if !ok {
			continue
		}

		first, _, _ := strings.Cut(value, string(os.PathListSeparator))

		entries, err := os.ReadDir(first)
		if err != nil {
			return execx.Result{}, err
		}

		h.mu.Lock()
		for _, entry := range entries {
			*h.staged = append(*h.staged, entry.Name())
		}
		h.mu.Unlock()
	}

	return h.fakeLizard.Run(ctx, command)
}

func TestGuardRunsLizardWithTheHookAndPatchesFirstOnPythonPath(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{".editorconfig": bounds})
	staged := []string{}
	lizard := hookCheck{files: map[string]int{}, mu: &sync.Mutex{}, staged: &staged}

	// Act
	guard(t, lizard, root, sized{lizard: []string{"csharp"}})

	// Assert: both runs, the csv and the xml, see the hook and the patch.
	for _, name := range []string{"sitecustomize.py", "lizard_languages.csharp.patch"} {
		if count := strings.Count(strings.Join(staged, "\n")+"\n", name+"\n"); count != 2 {
			t.Errorf("%s is staged for %d of 2 runs: %v", name, count, staged)
		}
	}
}

// pythonWithHook copies the hook and patches, keyed by file name, into a new
// PYTHONPATH directory beside a package greet, and returns the output of
// python importing it.
func pythonWithHook(t *testing.T, patches map[string]string) (string, error) {
	t.Helper()

	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}

	hook, err := os.ReadFile(filepath.Join("lizard", "sitecustomize.py"))
	if err != nil {
		t.Fatal(err)
	}

	hookDir, packages := t.TempDir(), t.TempDir()
	files := map[string]string{
		filepath.Join(hookDir, "sitecustomize.py"):      string(hook),
		filepath.Join(packages, "greet", "__init__.py"): "",
		filepath.Join(packages, "greet", "words.py"):    "def hello():\n    return \"hello\"\n",
	}
	for name, patch := range patches {
		files[filepath.Join(hookDir, name)] = patch
	}

	for path, content := range files {
		if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o750); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}

		if writeErr := os.WriteFile(path, []byte(content), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}

	cmd := exec.CommandContext(t.Context(), python, "-B", "-c", "import greet.words; print(greet.words.hello())")
	cmd.Dir = packages
	cmd.Env = append(os.Environ(), "PYTHONPATH="+hookDir+string(os.PathListSeparator)+packages)
	out, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(out)), err
}

func TestHookAppliesAPatchAsPythonImportsTheModule(t *testing.T) {
	t.Parallel()

	// Arrange
	patch := "--- a/greet/words.py\n+++ b/greet/words.py\n@@ -1,2 +1,2 @@\n" +
		" def hello():\n-    return \"hello\"\n+    return \"patched\"\n"

	// Act
	out, err := pythonWithHook(t, map[string]string{"greet.patch": patch})

	// Assert
	if err != nil || out != "patched" {
		t.Errorf("python printed %q (%v), want patched", out, err)
	}
}

func TestHookStopsTheImportWhenAPatchNoLongerApplies(t *testing.T) {
	t.Parallel()

	// Arrange
	patch := "--- a/greet/words.py\n+++ b/greet/words.py\n@@ -1,2 +1,2 @@\n" +
		" def hello():\n-    return \"hi\"\n+    return \"patched\"\n"

	// Act
	out, err := pythonWithHook(t, map[string]string{"greet.patch": patch})

	// Assert
	want := "bedrock: greet.patch no longer applies to greet.words"
	if err == nil || !strings.Contains(out, want) {
		t.Errorf("python printed %q (%v), want a failure naming %q", out, err, want)
	}
}

func TestHookStopsTheImportWhenTwoPatchesChangeOneModule(t *testing.T) {
	t.Parallel()

	// Arrange
	patch := "--- a/greet/words.py\n+++ b/greet/words.py\n@@ -1,2 +1,2 @@\n" +
		" def hello():\n-    return \"hello\"\n+    return \"patched\"\n"

	// Act
	out, err := pythonWithHook(t, map[string]string{"a.patch": patch, "b.patch": patch})

	// Assert
	want := "bedrock: b.patch patches greet.words, which another patch changes"
	if err == nil || !strings.Contains(out, want) {
		t.Errorf("python printed %q (%v), want a failure naming %q", out, err, want)
	}
}

// verbatim has a C# verbatim string that ends in a backslash. The unpatched
// lizard reads the backslash as escaping the closing quote, so the string
// runs on to the next quote and Folder swallows One.
const verbatim = `class Text
{
    string Folder()
    {
        return @"C:\";
    }

    string One()
    {
        int a = 1;
        return "one";
    }
}
`

func TestPatchedLizardEndsAVerbatimStringAtItsClosingQuote(t *testing.T) {
	t.Parallel()
	bedrocktest.E2E(t, "runs the pinned `lizard` through `uvx`")

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig": "root = true\n\n[*.cs]\nbedrock_function_code_lines_error = 6\n",
		"src/Text.cs":   verbatim,
	})

	// Act
	report := guard(t, execx.System{}, root, sized{lizard: []string{"csharp"}})

	// Assert
	expectLines(t, "errors", report.Errors, nil)
}

// accessor has a property getter with a block body. The unpatched lizard
// reports no function for it, so the guard never measured a getter.
const accessor = `class Shop
{
    int total;

    int Total
    {
        get
        {
            int sum = 0;
            sum += total;
            sum += 1;
            return sum;
        }
    }

    int Twice() => total * 2;
}
`

func TestPatchedLizardMeasuresAPropertyAccessor(t *testing.T) {
	t.Parallel()
	bedrocktest.E2E(t, "runs the pinned `lizard` through `uvx`")

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig": "root = true\n\n[*.cs]\nbedrock_function_code_lines_error = 5\n",
		"src/Shop.cs":   accessor,
	})

	// Act
	report := guard(t, execx.System{}, root, sized{lizard: []string{"csharp"}})

	// Assert
	expectLines(t, "errors", report.Errors, []string{"src/Shop.cs:8: `Shop::Total::get` code lines is 6, over 5"})
}

// kotlinType passes a parameter named type. The unpatched lizard reads `type`
// as Go's keyword, skips the `}` that ends record, and runs record on to the
// end of the class.
const kotlinType = `class Types {
    private val names = mutableListOf<String>()

    fun record(type: String) {
        names.add(type)
    }

    fun summary(): String {
        val first = names.first()
        val last = names.last()
        val count = names.size
        return "$first..$last ($count)"
    }
}
`

func TestPatchedLizardEndsAKotlinFunctionThatUsesTheNameType(t *testing.T) {
	t.Parallel()
	bedrocktest.E2E(t, "runs the pinned `lizard` through `uvx`")

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig":            "root = true\n\n[*.kt]\nbedrock_function_code_lines_error = 4\n",
		"src/main/kotlin/Types.kt": kotlinType,
	})

	// Act
	report := guard(t, execx.System{}, root, sized{lizard: []string{"kotlin"}})

	// Assert
	expectLines(t, "errors", report.Errors, []string{"src/main/kotlin/Types.kt:8: `summary` code lines is 6, over 4"})
}

// scalaIndented has a Scala 3 method whose body is set off by indentation.
// The unpatched lizard ends it at its first line break, so it measured one
// line of it.
const scalaIndented = `object Shapes:
  def area(width: Int, height: Int): Int =
    val w = width.max(0)
    val h = height.max(0)
    val a = w * h
    a

  def unit: Int = 1
`

func TestPatchedLizardMeasuresAnIndentedScalaBody(t *testing.T) {
	t.Parallel()
	bedrocktest.E2E(t, "runs the pinned `lizard` through `uvx`")

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig":               "root = true\n\n[*.scala]\nbedrock_function_code_lines_error = 4\n",
		"src/main/scala/Shapes.scala": scalaIndented,
	})

	// Act
	report := guard(t, execx.System{}, root, sized{lizard: []string{"scala"}})

	// Assert
	expectLines(t, "errors", report.Errors, []string{"src/main/scala/Shapes.scala:2: `area` code lines is 5, over 4"})
}
