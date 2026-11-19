// Package golang is the Go brick: Go modules checked by golangci-lint v2
// with a strict config, go test, and recipes that bound memory and
// parallelism. It is the reference for a new language: a Spec, the
// capabilities it needs, and its templates under templates/go.
package golang

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// goMod is the module file.
const goMod = "go.mod"

// Version is the Go language version a new go.mod declares.
const Version = "1.27"

// Go is the Go language.
type Go struct{}

// New returns the Go language.
func New() Go { return Go{} }

// Spec declares Go's templates, markers, and tools.
func (Go) Spec() language.Spec {
	// Pure Go cross-compiles to these operating systems, which a service also runs on.
	servers := []string{"freebsd", "linux", "macos", "windows"}

	return language.Spec{
		ID:      "go",
		Block:   "go-bedrock",
		Aliases: []string{"golang"},
		Markers: []string{goMod, "go.work", ".golangci.yml"},
		Tools:   []language.Tool{{Name: "go", Install: "https://go.dev/dl, such as `brew install go`"}},
		Sources: []string{"**/*.go", goMod, "go.sum", "go.work", ".golangci.yml"},
		Kinds:   []string{"command", "library", "service"},
		Platforms: map[string][]string{
			"command": append(slices.Clone(servers), "wasi", "wasm"),
			"library": append(slices.Clone(servers), "ios", "wasi", "wasm"),
			"service": servers,
		},
	}
}

// Variables fill the module path and a package name for the starter code.
func (Go) Variables(_ context.Context, c *language.Context) (template.Variables, error) {
	module, _, err := moduleFor(c)
	if err != nil {
		return nil, err
	}

	return template.Variables{"module": module, "package": packageOf(filepath.Base(c.Root))}, nil
}

// Plan starts go.mod when the repository has none, so the starter config
// and files have a module to name.
func (Go) Plan(_ context.Context, c *language.Context) (language.Contribution, error) {
	if fsutil.Exists(filepath.Join(c.Root, goMod)) {
		return language.Contribution{}, nil
	}

	module, placeholder, err := moduleFor(c)
	if err != nil {
		return language.Contribution{}, err
	}

	var notes []string
	if placeholder {
		notes = append(notes, "`go.mod` names the placeholder module "+module+
			"; change it, and the `goimports` prefix in `.golangci.yml`, to the real path.")
	}

	return language.Contribution{
		Steps: []step.Step{step.Write{
			Path:       goMod,
			Content:    []byte(moduleFile(module, c)),
			Executable: false,
		}},
		Notes:        notes,
		ProjectFiles: []string{goMod},
	}, nil
}

// mobileTools is the tool block `just go::build ios` runs gomobile from; go mod
// tidy adds the requirement and its checksums.
const mobileTools = "\ntool (\n\tgolang.org/x/mobile/cmd/gobind\n\tgolang.org/x/mobile/cmd/gomobile\n)\n"

// moduleFile returns a new go.mod. A library built for iOS also pins gomobile.
func moduleFile(module string, c *language.Context) string {
	text := fmt.Sprintf("module %s\n\ngo %s\n", module, Version)

	kinds, _ := config.SplitList(c.Options["kind"])
	platforms, _ := config.SplitList(c.Options["platform"])

	if slices.Contains(kinds, "library") && slices.Contains(platforms, "ios") {
		text += mobileTools
	}

	return text
}

// AfterApply records the starter code's dependencies in go.mod and go.sum.
func (Go) AfterApply(ctx context.Context, c *language.Context) error {
	if !c.Runner.LookPath("go") {
		return nil
	}

	_, err := execx.Output(ctx, c.Runner, execx.Command{Dir: c.Root, Name: "go", Args: []string{"mod", "tidy"}})

	return err
}

// Runs names what AfterApply does.
func (Go) Runs() string { return "go mod tidy" }

// foreign are the dependency managers Go modules replaced.
func foreign() map[string]string {
	return map[string]string{
		"Gopkg.toml":         "dep",
		"Gopkg.lock":         "dep",
		"glide.yaml":         "Glide",
		"glide.lock":         "Glide",
		"vendor/vendor.json": "govendor",
		"Godeps/Godeps.json": "Godep",
		".golangci.yaml":     "",
		".golangci.toml":     "",
		".golangci.json":     "",
	}
}

// Guard reports dependency-manager files and a second golangci-lint config;
// go mod tidy and golangci-lint cover the rest.
func (Go) Guard(ctx context.Context, c *language.Context, _ bool) ([]language.Finding, error) {
	known := foreign()

	files, err := fsutil.ListFiles(ctx, c.Runner, c.Root, func(path string) bool {
		_, ok := known[path]

		return ok
	})
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	findings := make([]language.Finding, 0, len(files))

	for _, path := range files {
		if tool := known[path]; tool != "" {
			findings = append(
				findings,
				path+": "+tool+" file; Go modules replace it, so `go.mod` and `go.sum` hold the dependencies",
			)
		} else {
			findings = append(
				findings,
				path+": bedrock writes `.golangci.yml`, so keep the settings there and delete this file",
			)
		}
	}

	return findings, nil
}

func moduleLine() *regexp.Regexp { return regexp.MustCompile(`(?m)^module\s+("?)(\S+?)"?\s*$`) }

// moduleFor returns the module path: the repository's go.mod, else the
// module option, else a placeholder under example.com, which it reports.
func moduleFor(c *language.Context) (string, bool, error) {
	text, found, err := fsutil.ReadText(filepath.Join(c.Root, goMod))
	if err != nil {
		return "", false, err
	}

	if found {
		if match := moduleLine().FindStringSubmatch(text); match != nil {
			return match[2], false, nil
		}
	}

	if chosen := c.Options["module"]; chosen != "" {
		return chosen, false, nil
	}

	return "example.com/" + filepath.Base(c.Root), true, nil
}

// packageOf makes a lower-case Go package name from a directory name, such
// as my-app to myapp; one that starts with a digit gets a p prefix.
func packageOf(name string) string {
	word := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return -1
		}
	}, name)

	if word == "" || word[0] < 'a' {
		return "p" + word
	}

	return word
}
