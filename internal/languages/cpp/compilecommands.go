package cpp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// Subcommands are `bedrock cpp compile-commands` and `bedrock cpp test`.
func (Cpp) Subcommands() []language.Subcommand {
	return []language.Subcommand{
		{
			Name:    "compile-commands",
			Args:    "",
			Summary: "Write `compile_commands.json` for clang-tidy and clangd.",
			Run:     compileCommands,
		},
		{
			Name:    testName,
			Args:    "",
			Summary: "Build and run the tests of each CMake, Meson, and xmake project.",
			Run:     runTests,
		},
	}
}

// compileCommands writes `compile_commands.json` for clang-tidy and clangd
// from each build system the repository uses: a SwiftPM package's Clang
// targets, which SwiftPM emits no database for, and each CMake, Meson, and
// xmake project. CMake and Meson configure in `.cache/bedrock`, where
// `bedrock cpp test` and the test-file classification find the builds.
func compileCommands(ctx context.Context, c *language.Context, _ []string) (int, error) {
	entries, err := allEntries(ctx, c)
	if err != nil {
		_, _ = fmt.Fprintln(c.Stderr, err)

		return 1, nil
	}

	// Go sorts map keys, so the file only changes when the commands do.
	var text bytes.Buffer

	encoder := json.NewEncoder(&text)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err = encoder.Encode(entries); err != nil {
		return 1, fmt.Errorf("encode compile commands: %w", err)
	}

	write := step.Write{Path: "compile_commands.json", Content: text.Bytes(), Executable: false}
	if err = step.Apply(c.Root, []step.Step{write}); err != nil {
		return 1, fmt.Errorf("write `compile_commands.json`: %w", err)
	}

	return 0, nil
}

// allEntries gathers the entries of every build system.
func allEntries(ctx context.Context, c *language.Context) ([]map[string]any, error) {
	entries := []map[string]any{}

	sources := []func(context.Context, *language.Context) ([]map[string]any, error){
		packageEntries, cmakeEntries, mesonEntries, xmakeEntries,
	}

	for _, source := range sources {
		found, err := source(ctx, c)
		if err != nil {
			return nil, err
		}

		entries = append(entries, found...)
	}

	return entries, nil
}
