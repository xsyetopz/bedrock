package typescript

import (
	"fmt"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// yamlValue converts a YAML node into the ordered values the manifest code
// reads.
func yamlValue(node *yaml.Node) any {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil
		}

		return yamlValue(node.Content[0])
	case yaml.MappingNode:
		out := newObject()

		for i := 0; i+1 < len(node.Content); i += 2 {
			out.set(node.Content[i].Value, yamlValue(node.Content[i+1]))
		}

		return out
	case yaml.SequenceNode:
		items := make([]any, 0, len(node.Content))
		for _, item := range node.Content {
			items = append(items, yamlValue(item))
		}

		return items
	case yaml.AliasNode:
		return yamlValue(node.Alias)
	case yaml.ScalarNode:
		return yamlScalar(node)
	default:
		return nil
	}
}

func yamlScalar(node *yaml.Node) any {
	switch node.Tag {
	case "!!null":
		return nil
	case "!!bool":
		value, _ := strconv.ParseBool(node.Value)

		return value
	case "!!int", "!!float":
		return number(node.Value)
	default:
		return node.Value
	}
}

// parsePnpm reads `pnpm-workspace.yaml` into an object.
func parsePnpm(text string) (*object, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return nil, fmt.Errorf("parse `pnpm-workspace.yaml`: %w", err)
	}

	return asObject(yamlValue(&root)), nil
}
