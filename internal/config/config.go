// Package config reads and writes what a repository records about bedrock:
// the release it pins and each adopted language with its options. The
// bedrock block of .editorconfig holds them, as bedrock_* keys that editors
// ignore; releases before 0.5 kept them in .bedrock.json.
package config

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/editorconfig"
	"github.com/xsyetopz/bedrock/internal/fsutil"
)

const (
	// Path is the file whose bedrock block records the config.
	Path = ".editorconfig"
	// LegacyPath is where older releases kept it; adopt moves it into Path.
	LegacyPath = ".bedrock.json"
	// SourceVersion marks a repository that runs bedrock from its own tree.
	SourceVersion = "source"

	versionKey   = "bedrock_version"
	languagesKey = "bedrock_languages"
)

// Options are one language's settings, such as kind = command. Booleans are
// the strings "true" and "false", as .editorconfig writes them.
type Options map[string]string

// Bool reports whether key is set to true.
func (o Options) Bool(key string) bool {
	return o[key] == "true"
}

// Merge returns a copy of o with extra laid over it.
func (o Options) Merge(extra Options) Options {
	merged := make(Options, len(o)+len(extra))
	maps.Copy(merged, o)
	maps.Copy(merged, extra)

	return merged
}

// Config is the release and the languages a repository records.
type Config struct {
	// Languages maps each adopted language's id to its options.
	Languages map[string]Options
	// Version is the release just setup installs, or SourceVersion.
	Version string
}

// ErrLegacyShape reports a .bedrock.json bedrock cannot read.
var ErrLegacyShape = errors.New(
	"`" + LegacyPath + "`: expected `{ \"version\": string, \"languages\": { <id>: { <option>: string | boolean } } }`",
)

// Read returns the config from the bedrock block of .editorconfig, else from
// a .bedrock.json, or false when root has neither.
func Read(root string) (Config, bool, error) {
	text, found, err := fsutil.ReadText(filepath.Join(root, Path))
	if err != nil {
		return Config{}, false, err
	}

	if found {
		current, ok, parseErr := fromEditorconfig(text)
		if ok || parseErr != nil {
			return current, ok, parseErr
		}
	}

	legacy, found, err := fsutil.ReadText(filepath.Join(root, LegacyPath))
	if err != nil || !found {
		return Config{}, false, err
	}

	current, err := fromLegacy(legacy)

	return current, err == nil, err
}

func fromEditorconfig(text string) (Config, bool, error) {
	pairs := map[string]string{}

	for _, section := range editorconfig.Parse(text) {
		if section.Glob != "*" {
			continue
		}

		for _, pair := range section.Pairs {
			pairs[pair.Key] = pair.Value
		}
	}

	version, ok := pairs[versionKey]
	if !ok {
		return Config{}, false, nil
	}

	ids, err := SplitList(pairs[languagesKey])
	if err != nil {
		return Config{}, true, fmt.Errorf("`%s`: `%s`: %w", Path, languagesKey, err)
	}

	languages := map[string]Options{}

	for _, id := range ids {
		prefix := "bedrock_" + id + "_"
		options := Options{}

		for key, value := range pairs {
			if option, cut := strings.CutPrefix(key, prefix); cut {
				options[option] = value
			}
		}

		languages[id] = options
	}

	return Config{Version: version, Languages: languages}, true, nil
}

func fromLegacy(text string) (Config, error) {
	var raw struct {
		Languages map[string]map[string]any `json:"languages"`
		Version   *string                   `json:"version"`
	}

	if err := json.Unmarshal([]byte(text), &raw); err != nil || raw.Version == nil || raw.Languages == nil {
		return Config{}, ErrLegacyShape
	}

	languages := make(map[string]Options, len(raw.Languages))

	for id, values := range raw.Languages {
		options := Options{}

		for key, value := range values {
			switch value := value.(type) {
			case string:
				options[key] = value
			case bool:
				options[key] = strconv.FormatBool(value)
			default:
				return Config{}, ErrLegacyShape
			}
		}

		languages[id] = options
	}

	return Config{Version: *raw.Version, Languages: languages}, nil
}

// Pairs returns the bedrock_* lines the bedrock block of .editorconfig opens
// its [*] section with. bedrock_version keeps its own line, key = value,
// since the justfile reads it with sed to find the binary before bedrock runs.
func (c Config) Pairs() string {
	ids := slices.Sorted(maps.Keys(c.Languages))

	lines := []string{versionKey + " = " + c.Version}
	if len(ids) == 0 {
		lines = append(lines, languagesKey+" =")
	} else {
		lines = append(lines, languagesKey+" = "+strings.Join(ids, ", "))
	}

	for _, id := range ids {
		keys := slices.SortedFunc(maps.Keys(c.Languages[id]), cmp.Compare)
		for _, key := range keys {
			lines = append(lines, "bedrock_"+id+"_"+key+" = "+c.Languages[id][key])
		}
	}

	return strings.Join(lines, "\n")
}
