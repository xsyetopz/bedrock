package rust

// suppression is a lint suppression that skips the rule.
type suppression struct {
	message string
	line    int
}

// attribute is the body of `#[...]` or `#![...]`.
type attribute struct {
	inner []token
	line  int
	// next is the index of the token after the closing bracket.
	next int
}

func attributeAt(all []token, index int) (attribute, bool) {
	hash := tokenAt(all, index)
	if hash.text != "#" {
		return attribute{}, false
	}

	open := index + 1
	if tokenAt(all, open).text == "!" {
		open++
	}

	if tokenAt(all, open).text != "[" {
		return attribute{}, false
	}

	end, ok := closing(all, open)
	if !ok {
		return attribute{}, false
	}

	return attribute{line: hash.line, inner: all[open+1 : end], next: end + 1}, true
}

// hasReason reports whether the arguments have a `reason = ...`.
func hasReason(inner []token) bool {
	for index, tok := range inner {
		if tok.text == "reason" && tokenAt(inner, index+1).text == "=" {
			return true
		}
	}

	return false
}

// problemOf returns the message for one `allow` or `expect` that breaks the
// rule.
func problemOf(attr attribute) (string, bool) {
	switch tokenAt(attr.inner, 0).text {
	case "allow":
		return "`#[allow]` hides a lint without saying why; use `#[expect(lint, reason = \"…\")]`", true
	case "expect":
		if !hasReason(attr.inner) {
			return "`#[expect]` without a reason; add `reason = \"…\"`", true
		}
	}

	return "", false
}

// suppressionProblems returns the lint suppressions in source that skip the
// rule: `#[allow]` anywhere, and `#[expect]` without a `reason`.
func suppressionProblems(source string) []suppression {
	all := tokens(source)

	var found []suppression

	for index := 0; index < len(all); {
		attr, ok := attributeAt(all, index)
		if !ok {
			index++

			continue
		}

		if message, bad := problemOf(attr); bad {
			found = append(found, suppression{line: attr.line, message: message})
		}

		index = attr.next
	}

	return found
}
