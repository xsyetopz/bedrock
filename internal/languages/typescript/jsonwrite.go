package typescript

import (
	"bytes"
	"encoding/json"
	"strings"
)

// quote is a JSON string literal; unlike encoding/json's default it leaves
// <, >, and & as they are, as JSON.stringify does.
func quote(text string) string {
	var buf bytes.Buffer

	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	// A string always encodes.
	_ = encoder.Encode(text)

	return strings.TrimSuffix(buf.String(), "\n")
}

// compact is value as one-line JSON, for notes.
func compact(value any) string {
	var out strings.Builder

	writeValue(&out, value, -1)

	return out.String()
}

// toJSON is value in the repo's tab-indented style, ending in a newline. It
// writes no trailing commas.
func toJSON(value any) string {
	var out strings.Builder

	writeValue(&out, value, 0)
	out.WriteByte('\n')

	return out.String()
}

// writeValue writes value; a negative depth writes it on one line.
func writeValue(out *strings.Builder, value any, depth int) {
	switch v := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if v {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case number:
		out.WriteString(string(v))
	case string:
		out.WriteString(quote(v))
	case []any:
		writeList(out, v, depth)
	case *object:
		writeObject(out, v, depth)
	}
}

func newline(out *strings.Builder, depth int) {
	if depth >= 0 {
		out.WriteByte('\n')
		out.WriteString(strings.Repeat("\t", depth))
	}
}

func nextDepth(depth int) int {
	if depth < 0 {
		return depth
	}

	return depth + 1
}

func writeList(out *strings.Builder, items []any, depth int) {
	if len(items) == 0 {
		out.WriteString("[]")

		return
	}

	out.WriteByte('[')

	for i, item := range items {
		if i > 0 {
			out.WriteByte(',')
		}

		newline(out, nextDepth(depth))
		writeValue(out, item, nextDepth(depth))
	}

	newline(out, depth)
	out.WriteByte(']')
}

func writeObject(out *strings.Builder, o *object, depth int) {
	if o.len() == 0 {
		out.WriteString("{}")

		return
	}

	out.WriteByte('{')

	for i, key := range o.keys {
		if i > 0 {
			out.WriteByte(',')
		}

		newline(out, nextDepth(depth))
		out.WriteString(quote(key))
		out.WriteByte(':')

		if depth >= 0 {
			out.WriteByte(' ')
		}

		writeValue(out, o.values[key], nextDepth(depth))
	}

	newline(out, depth)
	out.WriteByte('}')
}
