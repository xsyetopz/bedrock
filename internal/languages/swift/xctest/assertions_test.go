package xctest_test

import (
	"strings"
	"testing"
)

func TestAssertionsConvert(t *testing.T) {
	t.Parallel()

	cases := [][2]string{
		{"XCTAssert(x)", "#expect(x)"},
		{"XCTAssertTrue(x)", "#expect(x)"},
		{`XCTAssertTrue(x, "why")`, `#expect(x, "why")`},
		{"XCTAssertTrue(a == b)", "#expect(a == b)"},
		{"XCTAssertFalse(ok)", "#expect(!ok)"},
		{"XCTAssertFalse(a.isEmpty)", "#expect(!a.isEmpty)"},
		{"XCTAssertFalse(a && b)", "#expect(!(a && b))"},
		{"XCTAssertEqual(a, b)", "#expect(a == b)"},
		{"XCTAssertNotEqual(a, b)", "#expect(a != b)"},
		{"XCTAssertGreaterThan(a, b)", "#expect(a > b)"},
		{"XCTAssertGreaterThanOrEqual(a, b)", "#expect(a >= b)"},
		{"XCTAssertLessThan(a, b)", "#expect(a < b)"},
		{"XCTAssertLessThanOrEqual(a, b)", "#expect(a <= b)"},
		{"XCTAssertIdentical(a, b)", "#expect(a === b)"},
		{"XCTAssertNotIdentical(a, b)", "#expect(a !== b)"},
		{"XCTAssertNil(a)", "#expect(a == nil)"},
		{"XCTAssertNotNil(a)", "#expect(a != nil)"},
		{"XCTAssertNil(a ?? b)", "#expect((a ?? b) == nil)"},
		{"XCTAssertEqual(a, b, accuracy: 0.1)", "#expect(abs(a - b) <= 0.1)"},
		{"XCTAssertNotEqual(a, b, accuracy: e)", "#expect(abs(a - b) > e)"},
		{"XCTAssertEqual(a + 1, b, accuracy: 0.1)", "#expect(abs((a + 1) - b) <= 0.1)"},
		{"XCTAssertEqual(a + 1, b ?? 2)", "#expect((a + 1) == (b ?? 2))"},
		{`XCTAssertEqual(x?.name, "a b")`, `#expect(x?.name == "a b")`},
		{"XCTAssertEqual(x, -1)", "#expect(x == -1)"},
		{`XCTAssertEqual(a, b, "m", file: f, line: l)`, `#expect(a == b, "m")`},
		{"XCTAssertEqual(a, b, file: f, line: l)", "#expect(a == b)"},
		{"XCTAssertTrue(x, message)", "#expect(x, Comment(rawValue: message))"},
		{"XCTAssertEqual(1, try f())", "#expect(try 1 == f())"},
		{"XCTAssertTrue(try await f())", "#expect(try await f())"},
		{"XCTAssertEqual(await a, await b)", "#expect(await a == b)"},
		{"let v = try XCTUnwrap(x)", "let v = try #require(x)"},
		{`let v = try XCTUnwrap(x, "missing")`, `let v = try #require(x, "missing")`},
		{`XCTFail("boom")`, `Issue.record("boom")`},
		{"XCTFail()", "Issue.record()"},
		{`XCTFail("boom \(x)", file: f, line: l)`, `Issue.record("boom \(x)")`},
		{"XCTAssertThrowsError(try f())", "#expect(throws: (any Error).self) { try f() }"},
		{`XCTAssertThrowsError(try f(), "m")`, `#expect(throws: (any Error).self, "m") { try f() }`},
		{"XCTAssertNoThrow(try f())", "#expect(throws: Never.self) { try f() }"},
		{"XCTAssertNoThrow(try await f())", "#expect(throws: Never.self) { try await f() }"},
	}

	for _, tc := range cases {
		t.Run(tc[0], func(t *testing.T) {
			t.Parallel()

			// Arrange / Act
			got := statement(t, tc[0])

			// Assert
			if got != tc[1] {
				t.Errorf("got %q, want %q", got, tc[1])
			}
		})
	}
}

func TestAssertionsLeaveStringsAndCommentsAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	source := strings.Join([]string{
		`let a = "XCTAssertEqual(a, b)"`,
		`let b = #"XCTAssertTrue("x")"#`,
		"// XCTAssertEqual(a, b)",
		"/* XCTAssertTrue(x) /* nested */ XCTAssertFalse(y) */",
		`let c = """`,
		"XCTAssertEqual(c, d)",
		`"""`,
	}, "\n")

	// Act
	got := statement(t, source)

	// Assert
	if got != source {
		t.Errorf("got %q, want the source unchanged", got)
	}
}

func TestAssertionsReadPastInterpolationAndEscapes(t *testing.T) {
	t.Parallel()

	// Arrange
	code := `XCTAssertEqual("a\")), \(f("x,)"))", g(1, 2))`

	// Act
	got := statement(t, code)

	// Assert
	if want := `#expect("a\")), \(f("x,)"))" == g(1, 2))`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAssertionsKeepCommasInNestedArguments(t *testing.T) {
	t.Parallel()

	cases := [][2]string{
		{"XCTAssertEqual(max(1, 2), 2)", "#expect(max(1, 2) == 2)"},
		{
			`XCTAssertEqual([1, 2], [a: 1, b: 2]["a", default: 0])`,
			`#expect([1, 2] == [a: 1, b: 2]["a", default: 0])`,
		},
		{"XCTAssertEqual(xs.map { $0 + 1 }, [2, 3])", "#expect((xs.map { $0 + 1 }) == [2, 3])"},
		{
			"XCTAssertEqual(xs.reduce(0) { a, b in a + b }, 3)",
			"#expect((xs.reduce(0) { a, b in a + b }) == 3)",
		},
		{"XCTAssertEqual(Dictionary<String, Int>(), [:])", "#expect(Dictionary<String, Int>() == [:])"},
	}

	for _, tc := range cases {
		if got := statement(t, tc[0]); got != tc[1] {
			t.Errorf("%s: got %q, want %q", tc[0], got, tc[1])
		}
	}
}

func TestAssertionsRewriteMultiLineCalls(t *testing.T) {
	t.Parallel()

	// Arrange
	code := "XCTAssertEqual(\n  foo(1,\n      2),\n  bar,\n  \"message\"\n)"

	// Act
	got := statement(t, code)

	// Assert
	if want := "#expect(foo(1,\n      2) == bar, \"message\")"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAssertionsRewriteNestedInClosures(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := "XCTAssertThrowsError(try f()) { XCTAssertEqual($0 as? E, .bad) }"

	// Act
	plain := statement(t, "items.forEach { XCTAssertNotNil($0) }")
	text := converted(t, "import XCTest\n"+handler+"\n").Text

	// Assert
	if plain != "items.forEach { #expect($0 != nil) }" {
		t.Errorf("closure assertion: got %q", plain)
	}

	for _, want := range []string{"{ #expect(($0 as? E) == .bad) }", "XCTAssertThrowsError(try f())"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
}

func TestAssertionsWithCommentsStayForAPerson(t *testing.T) {
	t.Parallel()

	// Arrange
	code := "XCTAssertEqual(a, // why\n  b)"

	// Act
	result := converted(t, "import XCTest\n"+code+"\n")

	// Assert
	if !strings.Contains(result.Text, code) {
		t.Errorf("the call changed: %q", result.Text)
	}

	if len(result.Notes) == 0 {
		t.Error("expected a note for the call left as is")
	}
}
