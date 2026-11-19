package typescript

import (
	"fmt"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
)

// kindFields are the package.json fields each kind's starter files need: the
// entry point other packages import, or the executable.
func kindFields(kind, name string) *object {
	fields := newObject()

	switch kind {
	case "cli":
		bin := newObject()
		bin.set(name, "./src/cli.ts")
		fields.set("bin", bin)
	case "library":
		exports := newObject()
		exports.set(".", "./src/index.ts")
		fields.set("exports", exports)
	}

	return fields
}

// kindsFields are the fields of the kinds `bedrock init` starts, for fields
// a manifest lacks.
func kindsFields(c *language.Context, name string) (*object, error) {
	fields := newObject()
	if !c.Init {
		return fields, nil
	}

	kinds, err := config.SplitList(c.Options["kind"])
	if err != nil {
		return nil, fmt.Errorf("read the `kind` option: %w", err)
	}

	for _, kind := range kinds {
		fields = fields.overlay(kindFields(kind, name))
	}

	return fields, nil
}
