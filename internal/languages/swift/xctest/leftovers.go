package xctest

import (
	"strings"
)

const (
	confirmation = "use `confirmation(...)`"
	skipAdvice   = "use the `.enabled(if:)` or `.disabled` trait"
)

// leftover is a construct left for a person, with its 1-based line.
type leftover struct {
	message     string
	line        int
	needsXCTest bool
}

// manualEntry describes an XCTest construct with no direct Swift Testing form.
type manualEntry struct {
	label  string
	advice string
}

// manualOf returns what to tell a person about an XCTest identifier that
// has no direct Swift Testing form.
func manualOf(name string) (manualEntry, bool) {
	switch name {
	case "XCTSkip", "XCTSkipIf", "XCTSkipUnless":
		return manualEntry{label: "`" + name + "`", advice: skipAdvice}, true
	case "XCTestExpectation":
		return manualEntry{label: "`XCTestExpectation`", advice: confirmation}, true
	case "expectation":
		return manualEntry{label: "`expectation(description:)`", advice: confirmation}, true
	case kwWait:
		return manualEntry{label: "`wait(for:)`", advice: confirmation}, true
	case "waitForExpectations":
		return manualEntry{label: "`waitForExpectations`", advice: confirmation}, true
	case kwFulfill:
		return manualEntry{label: "`fulfill()`", advice: confirmation}, true
	case kwMeasure:
		return manualEntry{label: "`measure`", advice: "time the work with `ContinuousClock` by hand"}, true
	case "XCUIApplication":
		return manualEntry{label: "`XCUIApplication`", advice: "keep UI tests in an XCTest target"}, true
	case "addTeardownBlock":
		return manualEntry{label: "`addTeardownBlock`", advice: "use `defer` or `deinit`"}, true
	case "continueAfterFailure":
		return manualEntry{label: "`continueAfterFailure`", advice: "use `#require` to stop at the first failure"}, true
	case "XCTAssertThrowsError":
		return manualEntry{
			label:  "`XCTAssertThrowsError` with an error handler",
			advice: "use `#expect(throws:)` and inspect the error",
		}, true
	default:
		return manualEntry{}, false
	}
}

func callOnly(name string) bool {
	switch name {
	case "expectation", kwWait, kwMeasure, kwFulfill:
		return true
	default:
		return false
	}
}

// isXCTestCall reports whether a call-only identifier really is the XCTest
// call: `fulfill` is a method on an expectation, the others are bare calls.
func isXCTestCall(name, following string, bare bool) bool {
	prefix := "("
	if name == kwWait {
		prefix = "(for:"
	}

	isCall := strings.HasPrefix(following, prefix) || (name == kwMeasure && strings.HasPrefix(following, "{"))

	return isCall && bare == (name != kwFulfill)
}

func classify(name, previous string, entry manualEntry, hasEntry bool) (leftover, bool) {
	switch {
	case hasEntry:
		return leftover{
			message:     entry.label + " has no direct Swift Testing form; " + entry.advice,
			needsXCTest: true,
		}, true
	case strings.HasPrefix(name, "XCT") && previous != kwImport:
		return leftover{message: "`" + name + "` could not be converted; left as is", needsXCTest: true}, true
	case previous == kwFunc && (strings.HasPrefix(name, "setUp") || strings.HasPrefix(name, "tearDown")):
		return leftover{
			message: "`" + name + "` left as is; `init` and `deinit` appear once, " +
				"and `deinit` cannot throw or be async",
		}, true
	default:
		return leftover{}, false
	}
}

func isNoted(found []leftover, line int, message string) bool {
	for _, other := range found {
		if other.line == line && other.message == message {
			return true
		}
	}

	return false
}

// leftovers finds constructs still left for a person in the converted text.
func leftovers(source string) []leftover {
	b := toScalars(source)
	tokens := b.identifiers()

	var found []leftover

	line := 1
	position := 0

	for k, tok := range tokens {
		for at := position; at < tok.start; at++ {
			if b[at] == '\n' {
				line++
			}
		}

		position = tok.start

		previous := ""
		if k > 0 {
			previous = tokens[k-1].name
		}

		last, hasLast := b.behind(tok.start)
		bare := !hasLast || last != '.' || previous == "self"
		skipped := callOnly(tok.name) && !isXCTestCall(tok.name, b.ahead(tok.end), bare)

		entry, hasEntry := manualOf(tok.name)
		hit, ok := classify(tok.name, previous, entry, hasEntry && !skipped)

		if ok && !isNoted(found, line, hit.message) {
			hit.line = line
			found = append(found, hit)
		}
	}

	return found
}
