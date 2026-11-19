package haxe_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/languages/haxe"
)

// e2eSources are a class, a build that names its target, and a test build
// that runs on the interpreter.
func e2eSources() map[string]string {
	return map[string]string{
		"src/demo/Greeting.hx": "package demo;\n\nclass Greeting {\n\tpublic static function of(name:String):String {\n" +
			"\t\treturn \"Hello, \" + name;\n\t}\n}\n",
		"src/Main.hx": "import demo.Greeting;\n\nclass Main {\n\tstatic function main() {\n" +
			"\t\ttrace(Greeting.of(\"world\"));\n\t}\n}\n",
		"test/TestMain.hx": "import demo.Greeting;\n\nclass TestMain {\n\tstatic function main() {\n" +
			"\t\tSys.exit(Greeting.of(\"you\") == \"Hello, you\" ? 0 : 1);\n\t}\n}\n",
		"build.hxml": "-cp src\n--main Main\n--js bin/main.js\n",
		"test.hxml":  "-cp src\n-cp test\n--main TestMain\n--interp\n",
	}
}

func requireHaxe(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("e2e runs the real toolchain")
	}

	if _, err := exec.LookPath("haxe"); err != nil {
		t.Skip("haxe is not installed")
	}
}

// requireStyleTools skips unless haxelib resolves the formatter and
// checkstyle, as `just haxe::setup` installs them.
func requireStyleTools(t *testing.T) {
	t.Helper()
	requireHaxe(t)

	if err := exec.CommandContext(t.Context(), "haxelib", "path", "checkstyle", "formatter").Run(); err != nil {
		t.Skip("haxelib has no checkstyle and formatter")
	}
}

// runSub runs `bedrock haxe <name>` with the real toolchain and returns its
// exit code.
func runSub(t *testing.T, root, name string, args ...string) int {
	t.Helper()

	c := bedrocktest.Context(t, root, bedrocktest.Select(t, "haxe", nil))
	c.Runner = execx.System{Stdout: os.Stderr, Stderr: os.Stderr}
	c.Stdout, c.Stderr = os.Stderr, os.Stderr

	for _, sub := range haxe.New().Subcommands() {
		if sub.Name == name {
			code, err := sub.Run(t.Context(), c, args)
			if err != nil {
				t.Fatal(err)
			}

			return code
		}
	}

	t.Fatalf("no subcommand %s", name)

	return 1
}

func TestE2EAdoptedProjectTypechecksAndRunsItsTests(t *testing.T) {
	t.Parallel()
	requireHaxe(t)

	// Arrange
	root := bedrocktest.Repo(t, e2eSources())
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "haxe", nil))

	// Act
	typechecked := runSub(t, root, "typecheck")
	tested := runSub(t, root, "test")

	// Assert
	if typechecked != 0 || tested != 0 {
		t.Errorf("typecheck %d, test %d", typechecked, tested)
	}

	if _, err := os.Stat(filepath.Join(root, "bin", "main.js")); err == nil {
		t.Error("typecheck wrote `bin/main.js`")
	}
}

func TestE2EFailingTestOrTypeErrorFailsItsCommand(t *testing.T) {
	t.Parallel()
	requireHaxe(t)

	// Arrange
	files := e2eSources()
	files["src/demo/Greeting.hx"] = "package demo;\n\nclass Greeting {\n\tpublic static function of(name:String):String {\n" +
		"\t\treturn \"Hi, \" + name;\n\t}\n}\n"
	files["src/Main.hx"] = "class Main {\n\tstatic function main() {\n\t\tvar n:Int = \"text\";\n\t}\n}\n"
	root := bedrocktest.Repo(t, files)

	// Act
	typechecked := runSub(t, root, "typecheck")
	tested := runSub(t, root, "test")

	// Assert
	if typechecked != 1 || tested != 1 {
		t.Errorf("typecheck %d, test %d, want 1 and 1", typechecked, tested)
	}
}

func TestE2EAdoptedProjectPassesFormatAndLintAndAFindingFailsLint(t *testing.T) {
	t.Parallel()
	requireStyleTools(t)

	// Arrange
	root := bedrocktest.Repo(t, e2eSources())
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "haxe", nil))

	// Act
	formatted := runSub(t, root, "format", "--check")
	linted := runSub(t, root, "lint")
	bedrocktest.Write(t, root, "src/Unused.hx", "import demo.Greeting;\n\nclass Unused {}\n")
	flagged := runSub(t, root, "lint")

	// Assert
	if formatted != 0 || linted != 0 || flagged != 1 {
		t.Errorf("format %d, lint %d, flagged %d, want 0, 0, 1", formatted, linted, flagged)
	}
}
