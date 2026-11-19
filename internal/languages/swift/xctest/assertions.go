package xctest

import (
	"strings"
)

// comparisonOperands is the count of operands a comparison assertion takes
// before its message.
const comparisonOperands = 2

type assertionKind int

const (
	kindTruth assertionKind = iota
	kindCompare
	kindNilCheck
	kindClosure
	kindUnwrap
	kindFail
)

// assertion is how one XCTest call converts.
type assertion struct {
	// op is the operator of a comparison or nil check.
	op string
	// throws is the error type a closure assertion expects.
	throws  string
	kind    assertionKind
	negated bool
}

func assertionOf(name string) (assertion, bool) {
	switch name {
	case "XCTAssert", "XCTAssertTrue":
		return assertion{kind: kindTruth}, true
	case "XCTAssertFalse":
		return assertion{kind: kindTruth, negated: true}, true
	case "XCTAssertEqual":
		return compare("=="), true
	case "XCTAssertNotEqual":
		return compare("!="), true
	case "XCTAssertGreaterThan":
		return compare(">"), true
	case "XCTAssertGreaterThanOrEqual":
		return compare(">="), true
	case "XCTAssertLessThan":
		return compare("<"), true
	case "XCTAssertLessThanOrEqual":
		return compare("<="), true
	case "XCTAssertIdentical":
		return compare("==="), true
	case "XCTAssertNotIdentical":
		return compare("!=="), true
	case "XCTAssertNil":
		return assertion{kind: kindNilCheck, op: "=="}, true
	case "XCTAssertNotNil":
		return assertion{kind: kindNilCheck, op: "!="}, true
	case "XCTUnwrap":
		return assertion{kind: kindUnwrap}, true
	case "XCTFail":
		return assertion{kind: kindFail}, true
	case "XCTAssertThrowsError":
		return assertion{kind: kindClosure, throws: "(any Error).self"}, true
	case "XCTAssertNoThrow":
		return assertion{kind: kindClosure, throws: "Never.self"}, true
	default:
		return assertion{}, false
	}
}

func compare(op string) assertion { return assertion{kind: kindCompare, op: op} }

// replacement is the text a call is replaced by, and the index after the call.
type replacement struct {
	text string
	end  int
}

func leadingEffect(text string) string {
	for _, effect := range []string{kwTry, kwAwait} {
		if strings.HasPrefix(text, effect+" ") {
			return effect
		}
	}

	return ""
}

// builder builds one replacement call, tracking the `try` and `await` its
// operands carried.
type builder struct {
	effects     map[string]bool
	accuracy    string
	positional  []string
	hasAccuracy bool
}

func (m *builder) arg(index int) string {
	if index < len(m.positional) {
		return m.positional[index]
	}

	return ""
}

func (m *builder) operand(index int, wrapped bool) string {
	text := m.arg(index)

	for word := leadingEffect(text); word != ""; word = leadingEffect(text) {
		m.effects[word] = true
		text = strings.TrimSpace(text[len(word):])
	}

	if wrapped {
		return wrap(text)
	}

	return text
}

func (m *builder) message(index int) string {
	text := m.arg(index)
	b := toScalars(text)

	if end, ok := b.trivia(0); ok && end == len(b) && b[0] != '/' {
		return text
	}

	return "Comment(rawValue: " + text + ")"
}

func (m *builder) call(head string, count int, first func() string, tail string) (string, bool) {
	if len(m.positional) != count && len(m.positional) != count+1 {
		return "", false
	}

	body := first()

	var lead strings.Builder

	for _, effect := range []string{kwTry, kwAwait} {
		if m.effects[effect] {
			lead.WriteString(effect + " ")
		}
	}

	var parts []string
	if body != "" {
		parts = append(parts, body)
	}

	if len(m.positional) > count {
		if message := m.message(count); message != "" {
			parts = append(parts, message)
		}
	}

	return head + lead.String() + strings.Join(parts, ", ") + tail, true
}

func (m *builder) comparison(op string) (string, bool) {
	if m.hasAccuracy && op != "==" && op != "!=" {
		return "", false
	}

	return m.call("#expect(", comparisonOperands, func() string {
		left := m.operand(0, true)
		right := m.operand(1, true)

		if !m.hasAccuracy {
			return left + " " + op + " " + right
		}

		limit := ">"
		if op == "==" {
			limit = "<="
		}

		return "abs(" + left + " - " + right + ") " + limit + " " + wrap(m.accuracy)
	}, ")")
}

func (m *builder) build(kind assertion) (string, bool) {
	switch kind.kind {
	case kindTruth:
		return m.call("#expect(", 1, func() string {
			if kind.negated {
				return "!" + m.operand(0, true)
			}

			return m.operand(0, false)
		}, ")")
	case kindNilCheck:
		return m.call("#expect(", 1, func() string { return m.operand(0, true) + " " + kind.op + " nil" }, ")")
	case kindUnwrap:
		return m.call("#require(", 1, func() string { return m.arg(0) }, ")")
	case kindFail:
		return m.call("Issue.record(", 0, func() string { return "" }, ")")
	case kindClosure:
		return m.call("#expect(", 1, func() string { return "throws: " + kind.throws }, ") { "+m.arg(0)+" }")
	case kindCompare:
		return m.comparison(kind.op)
	}

	return "", false
}

func convertCall(kind assertion, b scalars, open int) (replacement, bool) {
	closing, ok := b.matchClose(open)
	if !ok {
		return replacement{}, false
	}

	parts, ok := splitArguments(b[open+1 : closing])
	if !ok {
		return replacement{}, false
	}

	// A trailing closure is an error handler; those calls stay with a person.
	for _, c := range b[closing+1:] {
		if !isSpace(c) {
			if c == '{' {
				return replacement{}, false
			}

			break
		}
	}

	parsed := parseArguments(parts, rewriteAssertions)
	m := &builder{
		effects:     map[string]bool{},
		positional:  parsed.positional,
		accuracy:    parsed.accuracy,
		hasAccuracy: parsed.hasAccuracy,
	}

	text, ok := m.build(kind)
	if !ok {
		return replacement{}, false
	}

	return replacement{text: text, end: closing + 1}, true
}

// callAfter converts the call whose `(` is the first non-blank scalar from from.
func callAfter(kind assertion, b scalars, from int) (replacement, bool) {
	open := from
	for open < len(b) && isSpace(b[open]) {
		open++
	}

	if b.at(open) != '(' || open >= len(b) {
		return replacement{}, false
	}

	return convertCall(kind, b, open)
}

// rewriteAssertions rewrites each XCTAssert* call, XCTUnwrap, and XCTFail
// into Swift Testing.
func rewriteAssertions(source string) string {
	b := toScalars(source)

	var out strings.Builder

	index := 0
	for index < len(b) {
		end, isTrivia := b.trivia(index)
		if !isTrivia {
			end = index
		}

		if end == index && isIdent(b[index]) {
			for end < len(b) && isIdent(b[end]) {
				end++
			}
		}

		var (
			next replacement
			done bool
		)

		if kind, known := assertionOf(string(b[index:end])); end > index && known {
			next, done = callAfter(kind, b, end)
		}

		if !done {
			end = max(end, index+1)
			out.WriteString(string(b[index:end]))
			index = end

			continue
		}

		out.WriteString(next.text)

		index = next.end
	}

	return out.String()
}
