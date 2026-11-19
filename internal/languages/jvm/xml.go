package jvm

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// element is one element of a parsed XML file.
type element struct {
	attributes map[string]string
	name       string
	// text is the text of the element and everything below it, like the
	// DOM's textContent.
	text     string
	children []*element
}

var (
	errDoctype = errors.New("XML with a DOCTYPE is not read")
	errNoRoot  = errors.New("XML has no root element")
)

func doctype() *regexp.Regexp { return regexp.MustCompile(`(?i)<!DOCTYPE`) }

// parseXML parses an XML file into its root element. Maven, Ant, and Ivy
// files have no DTD, so a DOCTYPE is refused, which keeps a hostile file from
// reading others.
func parseXML(source string) (*element, error) {
	if doctype().MatchString(source) {
		return nil, errDoctype
	}

	decoder := xml.NewDecoder(strings.NewReader(source))
	// A pom declaring ISO-8859-1 stays readable; names and versions are ASCII.
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	var (
		stack []*element
		root  *element
	)

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("parse XML: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			opened := &element{name: t.Name.Local, attributes: attributesOf(t.Attr), text: "", children: nil}

			switch {
			case len(stack) > 0:
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, opened)
			case root == nil:
				root = opened
			}

			stack = append(stack, opened)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			for _, open := range stack {
				open.text += string(t)
			}
		}
	}

	if root == nil {
		return nil, errNoRoot
	}

	return root, nil
}

func attributesOf(attrs []xml.Attr) map[string]string {
	found := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		found[attr.Name.Local] = attr.Value
	}

	return found
}

// childrenNamed returns the elements directly below e called name; an empty
// name returns all of them. A nil element has none.
func (e *element) childrenNamed(name string) []*element {
	if e == nil {
		return nil
	}

	var found []*element

	for _, child := range e.children {
		if name == "" || child.name == name {
			found = append(found, child)
		}
	}

	return found
}

// child returns the first child called name, or nil.
func (e *element) child(name string) *element {
	if found := e.childrenNamed(name); len(found) > 0 {
		return found[0]
	}

	return nil
}

// textOf returns the trimmed text of the child called name, or "".
func (e *element) textOf(name string) string {
	if found := e.child(name); found != nil {
		return strings.TrimSpace(found.text)
	}

	return ""
}

// descendants returns every element called name below e, in document order.
func (e *element) descendants(name string) []*element {
	var found []*element

	for _, child := range e.children {
		if child.name == name {
			found = append(found, child)
		}

		found = append(found, child.descendants(name)...)
	}

	return found
}
