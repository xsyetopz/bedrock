package typescript

import (
	"slices"
	"strconv"
)

// object is a JSON object that keeps its keys in the order they were first
// set. package.json and tsconfig.json keep a person's key order, and Go maps
// lose it.
type object struct {
	values map[string]any
	keys   []string
}

// number is a JSON number kept as written, so 1.0 and 10 survive a rewrite.
type number string

func newObject() *object {
	return &object{values: map[string]any{}, keys: nil}
}

// set stores value under key, keeping the key's place when it exists.
func (o *object) set(key string, value any) {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}

	o.values[key] = value
}

func (o *object) get(key string) any {
	return o.values[key]
}

func (o *object) has(key string) bool {
	_, ok := o.values[key]

	return ok
}

func (o *object) len() int {
	return len(o.keys)
}

func (o *object) clone() *object {
	next := newObject()
	for _, key := range o.keys {
		next.set(key, o.values[key])
	}

	return next
}

// overlay sets every entry of extra on a copy of o.
func (o *object) overlay(extra *object) *object {
	next := o.clone()
	for _, key := range extra.keys {
		next.set(key, extra.values[key])
	}

	return next
}

// without returns a copy of o minus the keys that drop reports.
func (o *object) without(drop func(key string) bool) *object {
	next := newObject()

	for _, key := range o.keys {
		if !drop(key) {
			next.set(key, o.values[key])
		}
	}

	return next
}

// asObject returns value when it is an object, else an empty one.
func asObject(value any) *object {
	if o, ok := value.(*object); ok && o != nil {
		return o
	}

	return newObject()
}

// asStrings returns the string items of value, or none when it is not a list.
func asStrings(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))

	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}

	return out
}

// stringList converts names to a JSON list value.
func stringList(items []string) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}

	return out
}

// unique returns items without repeats, first occurrence first.
func unique(items []string) []string {
	out := make([]string, 0, len(items))

	for _, item := range items {
		if !slices.Contains(out, item) {
			out = append(out, item)
		}
	}

	return out
}

// deepEqual compares two parsed values the way a data check wants: object
// key order does not matter, list order does.
func deepEqual(a, b any) bool {
	switch left := a.(type) {
	case *object:
		right, ok := b.(*object)

		return ok && objectsEqual(left, right)
	case []any:
		right, ok := b.([]any)

		return ok && slices.EqualFunc(left, right, deepEqual)
	case number:
		right, ok := b.(number)

		return ok && numbersEqual(left, right)
	default:
		return a == b
	}
}

func objectsEqual(a, b *object) bool {
	if a.len() != b.len() {
		return false
	}

	for _, key := range a.keys {
		if !b.has(key) || !deepEqual(a.values[key], b.values[key]) {
			return false
		}
	}

	return true
}

func numbersEqual(a, b number) bool {
	left, errLeft := strconv.ParseFloat(string(a), 64)
	right, errRight := strconv.ParseFloat(string(b), 64)

	return errLeft == nil && errRight == nil && left == right
}
