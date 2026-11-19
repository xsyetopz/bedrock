package swift

import "strings"

// lineWidth is swift-format's default line length, which `.swift-format`
// keeps.
const lineWidth = 100

// indentStep is the indent of one nesting level.
const indentStep = 2

// node is a piece of Swift source that wraps the way swift-format does, so a
// manifest bedrock writes needs no format pass: a call or array that fits the
// line stays on it, and one that does not puts each argument or item on its
// own line.
type node struct {
	// text is the whole source of a leaf.
	text string
	// call names the function a call node invokes, such as `.target`.
	call string
	// args are the labelled arguments of a call.
	args []argument
	// items are the elements of an array node.
	items []node
	// array marks an array node, since an empty one has no items.
	array bool
}

type argument struct {
	label string
	value node
}

func leaf(text string) node { return node{text: text} }

func array(items ...node) node { return node{items: items, array: true} }

// stringArray returns an array of string literals.
func stringArray(names ...string) node {
	items := make([]node, 0, len(names))
	for _, name := range names {
		items = append(items, leaf(`"`+name+`"`))
	}

	return array(items...)
}

func call(name string, args ...argument) node { return node{call: name, args: args} }

func arg(label string, value node) argument { return argument{label: label, value: value} }

// quoted is the string literal of s.
func quoted(s string) node { return leaf(`"` + s + `"`) }

func (a argument) flat() string {
	if a.label == "" {
		return a.value.flat()
	}

	return a.label + ": " + a.value.flat()
}

// flat is the node on one line.
func (n node) flat() string {
	switch {
	case n.call != "":
		parts := make([]string, 0, len(n.args))
		for _, a := range n.args {
			parts = append(parts, a.flat())
		}

		return n.call + "(" + strings.Join(parts, ", ") + ")"
	case n.array:
		parts := make([]string, 0, len(n.items))
		for _, item := range n.items {
			parts = append(parts, item.flat())
		}

		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return n.text
	}
}

// render writes the node starting at column col, where the lines of its body
// indent by indent, leaving room for tail characters after it on its last
// line.
func (n node) render(indent, col, tail int) string {
	flat := n.flat()
	if col+len(flat)+tail <= lineWidth || (n.call == "" && !n.array) {
		return flat
	}

	pad := strings.Repeat(" ", indent+indentStep)

	var out strings.Builder

	switch {
	case n.call != "":
		out.WriteString(n.call + "(\n")

		for i, a := range n.args {
			prefix := a.label + ": "
			if a.label == "" {
				prefix = ""
			}

			last := 0
			if i < len(n.args)-1 {
				last = 1
			}

			out.WriteString(pad + prefix + a.value.render(indent+indentStep, indent+indentStep+len(prefix), last))

			if last == 1 {
				out.WriteString(",")
			}

			out.WriteString("\n")
		}

		out.WriteString(strings.Repeat(" ", indent) + ")")
	default:
		out.WriteString("[\n")

		// The last item takes no comma: the shipped .swift-format sets
		// multilineTrailingCommaBehavior to neverUsed.
		for i, item := range n.items {
			comma := ","
			if i == len(n.items)-1 {
				comma = ""
			}

			out.WriteString(pad + item.render(indent+indentStep, indent+indentStep, len(comma)) + comma + "\n")
		}

		out.WriteString(strings.Repeat(" ", indent) + "]")
	}

	return out.String()
}
