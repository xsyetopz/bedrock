package typescript

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/step"
)

// part is the steps and notes one config file contributes to a plan.
type part struct {
	steps []step.Step
	notes []string
}

// readJSONC reads a JSONC file's object, or nil when the file is missing.
func readJSONC(path string) (*object, error) {
	text, found, err := fsutil.ReadText(path)
	if err != nil || !found {
		return nil, err
	}

	value, err := parseJSONC(text)
	if err != nil {
		return nil, fmt.Errorf("parse `%s`: %w", path, err)
	}

	return asObject(value), nil
}

// parseObject reads template text that must be an object.
func parseObject(text string) (*object, error) {
	value, err := parseJSONC(text)
	if err != nil {
		return nil, err
	}

	return asObject(value), nil
}

// jsonWrite rewrites a JSON file only when its data changes, so its layout
// and comments stay and a second adopt plans nothing.
func jsonWrite(root, path string, next *object) ([]step.Step, error) {
	current, err := readJSONC(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return nil, err
	}

	if current != nil && deepEqual(current, next) {
		return nil, nil
	}

	return []step.Step{step.Write{Path: path, Content: []byte(toJSON(next)), Executable: false}}, nil
}

// joinCode lists values as inline code, comma separated.
func joinCode(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "`"+value+"`")
	}

	return strings.Join(quoted, ", ")
}
