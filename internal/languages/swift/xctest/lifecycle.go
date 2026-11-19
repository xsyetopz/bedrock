package xctest

import (
	"strings"
)

// container is a class or extension body being rewritten, with the
// lifecycle members it has converted.
type container struct {
	bodyDepth int
	seenBody  bool
	hasInit   bool
	hasDeinit bool
}

func isIndent(c rune) bool { return c == ' ' || c == '\t' }

func isIndentOnly(text string) bool {
	return prefixWhile(text, isIndent) == text
}

// lifecycle is an `override func setUp…()` or `tearDown…()` declaration.
type lifecycle struct {
	name  string
	rest  string
	at    int
	setUp bool
}

const overrideFunc = "override func "

// lifecycleOf finds a lifecycle declaration in the line.
func lifecycleOf(text string) (lifecycle, bool) {
	at := strings.Index(text, overrideFunc)
	if at == -1 {
		return lifecycle{}, false
	}

	rest := text[at+len(overrideFunc):]
	name := prefixWhile(rest, isWord)
	setUp := name == "setUp" || name == "setUpWithError"
	tearDown := name == "tearDown" || name == "tearDownWithError"

	if (!setUp && !tearDown) || !strings.HasPrefix(rest[len(name):], "()") {
		return lifecycle{}, false
	}

	return lifecycle{at: at, name: name, setUp: setUp, rest: rest}, true
}

// canBeDeinit reports whether the declaration can become deinit, which can
// neither throw nor be async and takes no attributes.
func canBeDeinit(found lifecycle, text string) bool {
	effects, _, _ := strings.Cut(found.rest[len(found.name)+2:], "{")

	return !strings.Contains(effects, "async") &&
		!strings.Contains(effects, "throws") &&
		isIndentOnly(text[:found.at])
}

// rewriteLifecycle turns `override func setUp…()` into `init()` and
// `tearDown…()` into `deinit`.
func rewriteLifecycle(text string, c *container) (string, bool) {
	found, ok := lifecycleOf(text)
	if !ok {
		return "", false
	}

	if found.setUp && c.hasInit || !found.setUp && c.hasDeinit {
		return "", false
	}

	if !found.setUp && !canBeDeinit(found, text) {
		return "", false
	}

	keyword := "deinit"
	end := found.at + len(overrideFunc) + len(found.name+"()")

	if found.setUp {
		c.hasInit = true
		keyword = "init"
		end -= 2
	} else {
		c.hasDeinit = true
	}

	return text[:found.at] + keyword + text[end:], true
}

// isSuperCall reports whether the line only calls `super.setUp…()` or
// `super.tearDown…()`.
func isSuperCall(text string) bool {
	parts := strings.FieldsFunc(strings.Trim(text, "; \t\r"), func(c rune) bool { return c == ' ' })

	for len(parts) > 0 && (parts[0] == kwTry || parts[0] == kwAwait) {
		parts = parts[1:]
	}

	if len(parts) != 1 {
		return false
	}

	switch parts[0] {
	case "super.setUp()", "super.setUpWithError()", "super.tearDown()", "super.tearDownWithError()":
		return true
	default:
		return false
	}
}
