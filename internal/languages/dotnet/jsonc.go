package dotnet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// object is a JSON object that keeps its keys in file order, so a manifest
// bedrock rewrites changes only the keys it means to.
type object struct {
	values map[string]any
	keys   []string
}

func newObject() *object { return &object{values: map[string]any{}, keys: nil} }

// get returns the value at key, or nil.
func (o *object) get(key string) any { return o.values[key] }

// section returns the nested object at key, or an empty one, so a missing or
// mistyped section reads as no settings.
func (o *object) section(key string) *object {
	if nested, ok := o.values[key].(*object); ok {
		return nested
	}

	return newObject()
}

// set stores value at key; a new key goes last.
func (o *object) set(key string, value any) {
	if _, found := o.values[key]; !found {
		o.keys = append(o.keys, key)
	}

	o.values[key] = value
}

// clone copies the object one level deep, which is as far as a rewrite edits.
func (o *object) clone() *object {
	next := newObject()
	for _, key := range o.keys {
		next.set(key, o.values[key])
	}

	return next
}

// parseObject reads JSONC text, with comments and trailing commas, as an
// object. Any other top-level value reads as an empty object.
func parseObject(text string) (*object, error) {
	decoder := json.NewDecoder(bytes.NewReader(stripJSONC(text)))
	decoder.UseNumber()

	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}

	if root, ok := value.(*object); ok {
		return root, nil
	}

	return newObject(), nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return token, nil
	}

	if delim == '[' {
		return decodeArray(decoder)
	}

	return decodeObject(decoder)
}

func decodeArray(decoder *json.Decoder) ([]any, error) {
	items := []any{}

	for decoder.More() {
		item, err := decodeValue(decoder)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return items, nil
}

func decodeObject(decoder *json.Decoder) (*object, error) {
	result := newObject()

	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}

		name, _ := key.(string)

		value, err := decodeValue(decoder)
		if err != nil {
			return nil, err
		}

		result.set(name, value)
	}

	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return result, nil
}

// stripJSONC removes comments and trailing commas outside strings, which
// leaves JSON that encoding/json reads.
func stripJSONC(text string) []byte {
	in := []byte(text)
	out := make([]byte, 0, len(in))

	for index := 0; index < len(in); index++ {
		char := in[index]

		switch {
		case char == '"':
			var literal []byte

			literal, index = stringLiteral(in, index)
			out = append(out, literal...)
		case char == '/' && index+1 < len(in) && in[index+1] == '/':
			index = indexFrom(in, index, "\n") - 1
		case char == '/' && index+1 < len(in) && in[index+1] == '*':
			index = indexFrom(in, index+len("/*"), "*/") + 1
		case char == '}' || char == ']':
			out = append(trimComma(out), char)
		default:
			out = append(out, char)
		}
	}

	return out
}

// trimComma drops a comma, and the space after it, that ends out.
func trimComma(out []byte) []byte {
	trimmed := bytes.TrimRight(out, " \t\r\n")
	if !bytes.HasSuffix(trimmed, []byte(",")) {
		return out
	}

	return bytes.TrimSuffix(trimmed, []byte(","))
}

// stringLiteral returns the string literal that starts at in[start] and the
// index of its closing quote.
func stringLiteral(in []byte, start int) ([]byte, int) {
	for index := start + 1; index < len(in); index++ {
		switch in[index] {
		case '\\':
			index++
		case '"':
			return in[start : index+1], index
		}
	}

	return in[start:], len(in) - 1
}

// indexFrom returns the index of the first end at or after from, or the
// length of in when it never appears.
func indexFrom(in []byte, from int, end string) int {
	found := bytes.Index(in[from:], []byte(end))
	if found < 0 {
		return len(in)
	}

	return from + found
}

// marshal writes value in the repository's JSON style: tab-indented, ending
// in a newline, with no trailing commas.
func marshal(value any) ([]byte, error) {
	var out bytes.Buffer

	if err := writeValue(&out, value, 0); err != nil {
		return nil, err
	}

	out.WriteByte('\n')

	return out.Bytes(), nil
}

var errUnsupported = errors.New("unsupported json value")

func writeValue(out *bytes.Buffer, value any, depth int) error {
	switch value := value.(type) {
	case *object:
		return writeObject(out, value, depth)
	case []any:
		return writeArray(out, value, depth)
	case []string:
		items := make([]any, 0, len(value))
		for _, item := range value {
			items = append(items, item)
		}

		return writeArray(out, items, depth)
	case json.Number:
		out.WriteString(value.String())
	case string, bool, nil:
		return writeScalar(out, value)
	default:
		return fmt.Errorf("%w: %T", errUnsupported, value)
	}

	return nil
}

func writeScalar(out *bytes.Buffer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	// Encode ends each value with a newline that the caller places itself.
	out.Truncate(out.Len() - 1)

	return nil
}

func writeObject(out *bytes.Buffer, value *object, depth int) error {
	if len(value.keys) == 0 {
		out.WriteString("{}")

		return nil
	}

	out.WriteString("{\n")

	for index, key := range value.keys {
		out.WriteString(strings.Repeat("\t", depth+1))

		if err := writeScalar(out, key); err != nil {
			return err
		}

		out.WriteString(": ")

		if err := writeValue(out, value.values[key], depth+1); err != nil {
			return err
		}

		out.WriteString(separator(index, len(value.keys)))
	}

	out.WriteString(strings.Repeat("\t", depth) + "}")

	return nil
}

func writeArray(out *bytes.Buffer, items []any, depth int) error {
	if len(items) == 0 {
		out.WriteString("[]")

		return nil
	}

	out.WriteString("[\n")

	for index, item := range items {
		out.WriteString(strings.Repeat("\t", depth+1))

		if err := writeValue(out, item, depth+1); err != nil {
			return err
		}

		out.WriteString(separator(index, len(items)))
	}

	out.WriteString(strings.Repeat("\t", depth) + "]")

	return nil
}

// separator ends item index of count: a comma between items, none after the
// last, since bedrock writes no trailing commas.
func separator(index, count int) string {
	if index < count-1 {
		return ",\n"
	}

	return "\n"
}
