package haxe

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// targets are flags that pick an output target, in the long and the older
// one-dash form.
func targets() []string {
	return []string{
		"js", "lua", "swf", "neko", "php", "cpp", "cppia", "cs", "java", "jvm", "python", "hl",
		"interp", runFlag, "x",
	}
}

// runFlag is `--run` here and the `run` subcommand of haxelib.
const runFlag = "run"

// runners are flags after which a compilation runs something: the
// interpreter, or `--cmd`.
func runners() []string { return []string{"interp", runFlag, "x", "cmd"} }

func flagPattern() *regexp.Regexp { return regexp.MustCompile(`^--?([\w-]+)`) }

func testPattern() *regexp.Regexp { return regexp.MustCompile(`(?i)^tests?\b`) }

// hxml is what an hxml file, with the hxml files it includes, asks of the
// compiler.
type hxml struct {
	// target is whether it names an output target, so it compiles as written.
	target bool
	// runs is whether a compilation of it runs a program: `--interp`,
	// `--run`, or `--cmd`.
	runs bool
}

// build is an hxml file of a repo, by its path from the root.
type build struct {
	hxml

	path string
}

// hxmlLines returns the lines of an hxml text that carry arguments: trimmed,
// without blanks and `#` comments.
func hxmlLines(text string) []string {
	var lines []string

	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.TrimSpace(raw)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}

	return lines
}

// flagOf returns a line's flag without its dashes, such as cp for -cp src,
// or "" for a bare argument.
func flagOf(line string) string {
	if match := flagPattern().FindStringSubmatch(line); match != nil {
		return match[1]
	}

	return ""
}

// readHxml reads the hxml file at path, relative to root, which the compiler
// runs from. An included hxml file adds its flags, as the compiler reads it
// in place; one already read, or missing, adds none.
func readHxml(root, path string, seen []string) (hxml, error) {
	text, _, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return hxml{}, err
	}

	var result hxml

	reading := append(slices.Clone(seen), path)
	lines := hxmlLines(text)

	for _, line := range lines {
		flag := flagOf(line)
		if flag == "" {
			continue
		}

		result.target = result.target || slices.Contains(targets(), flag)
		result.runs = result.runs || slices.Contains(runners(), flag)
	}

	for _, line := range lines {
		if flagOf(line) != "" || !strings.HasSuffix(line, ".hxml") || slices.Contains(reading, line) {
			continue
		}

		included, includeErr := readHxml(root, line, reading)
		if includeErr != nil {
			return hxml{}, includeErr
		}

		result.target = result.target || included.target
		result.runs = result.runs || included.runs
	}

	return result, nil
}

// gitFiles lists the paths git tracks or would track that match pattern,
// outside the local haxelib repo. A directory that is no work tree lists
// none.
func gitFiles(ctx context.Context, runner execx.Runner, root, pattern string) ([]string, error) {
	result, err := runner.Run(ctx, execx.Command{
		Dir:  root,
		Name: "git",
		Args: []string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", pattern},
	})
	if err != nil {
		return nil, err
	}

	var paths []string

	for path := range strings.SplitSeq(string(result.Stdout), "\x00") {
		if path != "" && !strings.HasPrefix(path, ".haxelib/") {
			paths = append(paths, path)
		}
	}

	slices.Sort(paths)

	return paths, nil
}

// hxmlBuilds returns each hxml file of root with what it asks of the compiler.
func hxmlBuilds(ctx context.Context, runner execx.Runner, root string) ([]build, error) {
	paths, err := gitFiles(ctx, runner, root, "*.hxml")
	if err != nil {
		return nil, err
	}

	builds := make([]build, 0, len(paths))

	for _, path := range paths {
		read, readErr := readHxml(root, path, nil)
		if readErr != nil {
			return nil, readErr
		}

		builds = append(builds, build{path: path, hxml: read})
	}

	return builds, nil
}

// isTestHxml reports whether path names a test build, such as test.hxml,
// tests.hxml, or test-native.hxml.
func isTestHxml(path string) bool {
	return testPattern().MatchString(filepath.Base(path))
}
