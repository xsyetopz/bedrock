package typescript

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// errJSONC wraps every syntax error the reader reports.
var errJSONC = errors.New("invalid JSON")

// parseJSONC reads JSON with comments and trailing commas, the dialect
// tsconfig.json and the lint configs use, into objects that keep key order.
func parseJSONC(text string) (any, error) {
	p := &reader{text: strings.TrimPrefix(text, "\uFEFF"), pos: 0}

	value, err := p.value()
	if err != nil {
		return nil, err
	}

	if skipErr := p.skip(); skipErr != nil {
		return nil, skipErr
	}

	if p.pos < len(p.text) {
		return nil, p.fail("unexpected text after the value")
	}

	return value, nil
}

type reader struct {
	text string
	pos  int
}

func (p *reader) fail(message string) error {
	line := 1 + strings.Count(p.text[:min(p.pos, len(p.text))], "\n")

	return fmt.Errorf("%w: %s at line %d", errJSONC, message, line)
}

// skip passes whitespace and comments.
func (p *reader) skip() error {
	for p.pos < len(p.text) {
		rest := p.text[p.pos:]

		switch {
		case strings.IndexByte(" \t\r\n", rest[0]) >= 0:
			p.pos++
		case strings.HasPrefix(rest, "//"):
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				end = len(rest)
			}

			p.pos += end
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				return p.fail("unterminated comment")
			}

			p.pos += end + len("/**/")
		default:
			return nil
		}
	}

	return nil
}

func (p *reader) value() (any, error) {
	if skipErr := p.skip(); skipErr != nil {
		return nil, skipErr
	}

	if p.pos >= len(p.text) {
		return nil, p.fail("no value")
	}

	switch p.text[p.pos] {
	case '{':
		return p.object()
	case '[':
		return p.array()
	case '"':
		return p.str()
	default:
		return p.scalar()
	}
}

func (p *reader) scalar() (any, error) {
	start := p.pos
	for p.pos < len(p.text) && strings.IndexByte(",]} \t\r\n/", p.text[p.pos]) < 0 {
		p.pos++
	}

	word := p.text[start:p.pos]
	switch word {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil //nolint:nilnil // JSON null is a value, and nil stands for it.
	}

	var parsed float64
	if err := json.Unmarshal([]byte(word), &parsed); err != nil {
		p.pos = start

		return nil, p.fail("unexpected `" + word + "`")
	}

	return number(word), nil
}

func (p *reader) str() (any, error) {
	start := p.pos
	p.pos++

	for p.pos < len(p.text) && p.text[p.pos] != '"' {
		if p.text[p.pos] == '\\' {
			p.pos++
		}

		p.pos++
	}

	if p.pos >= len(p.text) {
		return nil, p.fail("unterminated string")
	}

	p.pos++

	var out string
	if err := json.Unmarshal([]byte(p.text[start:p.pos]), &out); err != nil {
		return nil, p.fail("bad string")
	}

	return out, nil
}

// closes consumes the closing bracket when it is next, so a trailing comma
// before it is accepted.
func (p *reader) closes(bracket byte) (bool, error) {
	if skipErr := p.skip(); skipErr != nil {
		return false, skipErr
	}

	if p.pos < len(p.text) && p.text[p.pos] == bracket {
		p.pos++

		return true, nil
	}

	return false, nil
}

// separator consumes a comma or the closing bracket and reports whether the
// list continues.
func (p *reader) separator(bracket byte) (bool, error) {
	if skipErr := p.skip(); skipErr != nil {
		return false, skipErr
	}

	if p.pos >= len(p.text) {
		return false, p.fail("unterminated value")
	}

	switch p.text[p.pos] {
	case ',':
		p.pos++

		return true, nil
	case bracket:
		p.pos++

		return false, nil
	default:
		return false, p.fail("expected `,` or `" + string(bracket) + "`")
	}
}

func (p *reader) array() (any, error) {
	p.pos++

	items := []any{}

	if done, err := p.closes(']'); done || err != nil {
		return items, err
	}

	for {
		item, err := p.value()
		if err != nil {
			return nil, err
		}

		items = append(items, item)

		more, err := p.separator(']')
		if err != nil || !more {
			return items, err
		}

		if done, closeErr := p.closes(']'); done || closeErr != nil {
			return items, closeErr
		}
	}
}

func (p *reader) member() (string, any, error) {
	if skipErr := p.skip(); skipErr != nil {
		return "", nil, skipErr
	}

	if p.pos >= len(p.text) || p.text[p.pos] != '"' {
		return "", nil, p.fail("expected a key")
	}

	key, err := p.str()
	if err != nil {
		return "", nil, err
	}

	if err = p.skip(); err != nil {
		return "", nil, err
	}

	if p.pos >= len(p.text) || p.text[p.pos] != ':' {
		return "", nil, p.fail("expected `:`")
	}

	p.pos++

	item, err := p.value()

	text, _ := key.(string)

	return text, item, err
}

func (p *reader) object() (any, error) {
	p.pos++

	out := newObject()

	if done, err := p.closes('}'); done || err != nil {
		return out, err
	}

	for {
		key, item, err := p.member()
		if err != nil {
			return nil, err
		}

		out.set(key, item)

		more, err := p.separator('}')
		if err != nil || !more {
			return out, err
		}

		if done, closeErr := p.closes('}'); done || closeErr != nil {
			return out, closeErr
		}
	}
}
