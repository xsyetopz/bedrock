package swift

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.yaml.in/yaml/v3"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

const lintConfig = ".swiftlint.yml"

// lintExcluded returns whether SwiftLint skips a path, by the `excluded`
// list in the repository's `.swiftlint.yml`, so the guard reads the sources
// the linter reads. Each entry is a glob relative to the root that excludes
// the path it matches and everything under it; a bare name such as
// `Generated` matches only at the root.
func lintExcluded(root string) (func(path string) bool, error) {
	text, found, err := fsutil.ReadText(filepath.Join(root, lintConfig))
	if err != nil {
		return nil, err
	}

	var config struct {
		Excluded []string `yaml:"excluded"`
	}

	if found {
		if err = yaml.Unmarshal([]byte(text), &config); err != nil {
			return nil, fmt.Errorf("read `%s`: %w", lintConfig, err)
		}
	}

	return func(path string) bool {
		for _, entry := range config.Excluded {
			glob := strings.TrimSuffix(strings.TrimPrefix(entry, "./"), "/")
			if doublestar.MatchUnvalidated(glob, path) || doublestar.MatchUnvalidated(glob+"/**", path) {
				return true
			}
		}

		return false
	}, nil
}
