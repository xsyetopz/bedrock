package cpp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

const dirMode = 0o750

// cmakeProjects returns the CMake projects: each top `CMakeLists.txt`'s
// directory.
func cmakeProjects(ctx context.Context, c *language.Context) []string {
	return projectsOf(ctx, c, "CMakeLists.txt")
}

// configureCmake configures project in its bedrock build directory with the
// File API codemodel query the test classification reads, and
// compile_commands.json. BUILD_TESTING is the CTest module's switch, so the
// tests' targets are in the model.
func configureCmake(ctx context.Context, c *language.Context, project string) (string, error) {
	build := buildDir(c.Root, "cmake", project)
	query := filepath.Join(build, ".cmake", "api", "v1", "query")

	if err := os.MkdirAll(query, dirMode); err != nil {
		return "", fmt.Errorf("create `%s`: %w", query, err)
	}

	if err := os.WriteFile(filepath.Join(query, "codemodel-v2"), nil, 0o600); err != nil {
		return "", fmt.Errorf("write the codemodel query: %w", err)
	}

	result := run(ctx, c, c.Root, "cmake", "-S", filepath.Join(c.Root, project), "-B", build,
		"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON", "-DBUILD_TESTING=ON")
	if result.code != 0 {
		return "", toolFailed("cmake configure of %s failed: %s", displayName(project), firstLine(result.stderr))
	}

	return build, nil
}

// cmakeEntries returns the compile_commands.json entries of each CMake
// project, configured afresh.
func cmakeEntries(ctx context.Context, c *language.Context) ([]map[string]any, error) {
	return eachProject(c, cmakeProjects(ctx, c), func(project string) ([]map[string]any, error) {
		build, err := configureCmake(ctx, c, project)
		if err != nil {
			return nil, err
		}

		return readEntries(filepath.Join(build, "compile_commands.json"))
	})
}

// readEntries reads a compile_commands.json, empty when the file is absent.
func readEntries(path string) ([]map[string]any, error) {
	text, found, err := fsutil.ReadText(path)
	if err != nil || !found {
		return nil, err
	}

	var entries []map[string]any
	if parseErr := json.Unmarshal([]byte(text), &entries); parseErr != nil {
		return nil, fmt.Errorf("parse `%s`: %w", path, parseErr)
	}

	return entries, nil
}

// readJSON decodes the file at path into into; an absent file leaves it as is.
func readJSON(path string, into any) error {
	text, found, err := fsutil.ReadText(path)
	if err != nil || !found {
		return err
	}

	if parseErr := json.Unmarshal([]byte(text), into); parseErr != nil {
		return fmt.Errorf("parse `%s`: %w", path, parseErr)
	}

	return nil
}

// addTest matches an add_test line of a CTestTestfile.cmake. A multi-config
// generator, Visual Studio's or Ninja Multi-Config, indents one per
// configuration inside `if(CTEST_CONFIGURATION_TYPE ...)`.
func addTest() *regexp.Regexp { return regexp.MustCompile(`(?m)^[ \t]*add_test\((.*)\)\r?$`) }

// quotedArgument returns the text of the quoted argument that starts at line[i]
// and the index of its closing quote, or false when it never closes.
func quotedArgument(line string, i int) (string, int, bool) {
	end := i + 1
	for end < len(line) && line[end] != '"' {
		if line[end] == '\\' {
			end++
		}

		end++
	}

	if end >= len(line) {
		return "", i, false
	}

	return line[i+1 : end], end, true
}

// bracketArgument returns the text of the bracket argument that starts at
// line[i] and the index of its last bracket, or false when there is none.
func bracketArgument(line string, i int) (string, int, bool) {
	equals := len(line[i+1:]) - len(strings.TrimLeft(line[i+1:], "="))
	open := i + 1 + equals

	if open >= len(line) || line[open] != '[' {
		return "", i, false
	}

	closing := "]" + strings.Repeat("=", equals) + "]"

	end := strings.Index(line[open+1:], closing)
	if end < 0 {
		return "", i, false
	}

	return line[open+1 : open+1+end], open + 1 + end + len(closing) - 1, true
}

// cmakeArguments returns the quoted and bracket arguments of one `add_test`
// line's text, such as "run" and [=[a b]=].
func cmakeArguments(line string) []string {
	var args []string

	for i := 0; i < len(line); i++ {
		var (
			text  string
			last  int
			found bool
		)

		switch line[i] {
		case '"':
			text, last, found = quotedArgument(line, i)
		case '[':
			text, last, found = bracketArgument(line, i)
		}

		if found {
			args = append(args, text)
			i = last
		}
	}

	return args
}

// testCommandMinArgs is the count of `add_test` arguments up to the command:
// the test's name, then the program it runs.
const testCommandMinArgs = 2

// testedIn adds to tested the programs, relative to the root, that the
// `add_test` calls of one CTestTestfile.cmake run.
func testedIn(path, build string, inRepo func(string) (string, bool), tested map[string]bool) error {
	text, _, err := fsutil.ReadText(path)
	if err != nil {
		return err
	}

	for _, line := range addTest().FindAllStringSubmatch(text, -1) {
		args := cmakeArguments(line[1])
		if len(args) < testCommandMinArgs {
			continue
		}

		command := args[1]
		if !filepath.IsAbs(command) {
			command = filepath.Join(build, command)
		}

		if rel, ok := inRepo(command); ok {
			tested[rel] = true
		}
	}

	return nil
}

// testedPaths returns the paths, relative to the root, of the programs the
// tests of the build at build run, from the CTestTestfile.cmake files
// configure wrote. Only each test's command counts: a tool passed as an
// argument, such as one a test checks, stays product code.
func testedPaths(build string, inRepo func(string) (string, bool)) (map[string]bool, error) {
	tested := map[string]bool{}

	walkErr := filepath.WalkDir(build, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		if err != nil {
			return err
		}

		if entry.IsDir() || entry.Name() != "CTestTestfile.cmake" {
			return nil
		}

		return testedIn(path, build, inRepo, tested)
	})
	if walkErr != nil {
		return nil, fmt.Errorf("read the tests of `%s`: %w", build, walkErr)
	}

	return tested, nil
}

// codemodel is the part of a File API codemodel the classification reads.
type codemodel struct {
	Paths struct {
		Source string `json:"source"`
	} `json:"paths"`
	Configurations []struct {
		Targets []struct {
			JSONFile string `json:"jsonFile"`
		} `json:"targets"`
	} `json:"configurations"`
}

// latestCodemodel returns the codemodel of the latest File API reply in
// reply, or false when configure wrote none.
func latestCodemodel(reply string) (codemodel, bool, error) {
	var model codemodel

	entries, err := os.ReadDir(reply)
	// No reply directory means the project is not configured yet.
	if errors.Is(err, fs.ErrNotExist) {
		return model, false, nil
	}

	if err != nil {
		return model, false, fmt.Errorf("read `%s`: %w", reply, err)
	}

	var indexes []string

	for _, entry := range entries {
		if name := entry.Name(); strings.HasPrefix(name, "index-") && strings.HasSuffix(name, ".json") {
			indexes = append(indexes, name)
		}
	}

	if len(indexes) == 0 {
		return model, false, nil
	}

	slices.Sort(indexes)

	var index struct {
		Reply struct {
			Codemodel struct {
				JSONFile string `json:"jsonFile"`
			} `json:"codemodel-v2"`
		} `json:"reply"`
	}

	if err = readJSON(filepath.Join(reply, indexes[len(indexes)-1]), &index); err != nil {
		return model, false, err
	}

	if err = readJSON(filepath.Join(reply, index.Reply.Codemodel.JSONFile), &model); err != nil {
		return model, false, err
	}

	return model, true, nil
}

// cmakeBuild is what a File API target object is read against.
type cmakeBuild struct {
	inRepo func(string) (string, bool)
	// tested are the programs the tests run, relative to the root.
	tested map[string]bool
	// dir is the build directory, which artifact paths are relative to.
	dir string
	// source is the source directory, which source paths are relative to.
	source string
}

// cmakeTarget is a File API target object.
type cmakeTarget struct {
	Install json.RawMessage `json:"install"`
	ID      string          `json:"id"`
	Sources []struct {
		Path        string `json:"path"`
		IsGenerated bool   `json:"isGenerated"`
	} `json:"sources"`
	Dependencies []struct {
		ID string `json:"id"`
	} `json:"dependencies"`
	Artifacts []struct {
		Path string `json:"path"`
	} `json:"artifacts"`
}

func (b cmakeBuild) resolve(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(base, path)
}

// target converts a File API target object.
func (b cmakeBuild) target(raw cmakeTarget) target {
	out := target{
		id:        raw.ID,
		sources:   nil,
		deps:      nil,
		test:      false,
		installed: len(raw.Install) > 0,
	}

	for _, source := range raw.Sources {
		if source.IsGenerated {
			continue
		}

		// Generated sources and those outside the repository are no file of it.
		if path, ok := b.inRepo(b.resolve(b.source, source.Path)); ok && !inBuildDirs(path) {
			out.sources = append(out.sources, path)
		}
	}

	for _, dep := range raw.Dependencies {
		out.deps = append(out.deps, dep.ID)
	}

	for _, artifact := range raw.Artifacts {
		if path, ok := b.inRepo(b.resolve(b.dir, artifact.Path)); ok && b.tested[path] {
			out.test = true
		}
	}

	return out
}

// cmakeTargets returns the targets of the configured build of project, from
// the File API codemodel, with the tests from CTest. A target whose artifact a
// test runs is a test target; one with an install rule is product code.
func cmakeTargets(root, project string) ([]target, error) {
	dir := buildDir(root, "cmake", project)
	reply := filepath.Join(dir, ".cmake", "api", "v1", "reply")

	model, found, err := latestCodemodel(reply)
	if err != nil || !found || len(model.Configurations) == 0 {
		return nil, err
	}

	inRepo := repoPaths(root)

	tested, err := testedPaths(dir, inRepo)
	if err != nil {
		return nil, err
	}

	build := cmakeBuild{inRepo: inRepo, tested: tested, dir: dir, source: model.Paths.Source}

	var targets []target

	for _, entry := range model.Configurations[0].Targets {
		var raw cmakeTarget
		if err = readJSON(filepath.Join(reply, entry.JSONFile), &raw); err != nil {
			return nil, err
		}

		targets = append(targets, build.target(raw))
	}

	return targets, nil
}
