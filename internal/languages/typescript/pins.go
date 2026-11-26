package typescript

import (
	"slices"
	"strconv"
	"strings"
)

// releaseParts counts MAJOR, MINOR, and PATCH.
const releaseParts = 3

// withNewerPins returns the template's pins with each one the repo raised
// kept, so `bedrock update` does not undo the bumps Dependabot makes between
// bedrock releases. `typescript` keeps the template's pin, the TypeScript Bun
// bundles, since the guard requires the two to match.
func withNewerPins(repo, template *object) *object {
	next := template.clone()

	for _, name := range template.keys {
		mine, _ := repo.get(name).(string)
		shipped, _ := template.get(name).(string)

		if name != nameTypescript && newerRelease(mine, shipped) {
			next.set(name, mine)
		}
	}

	return next
}

// newerRelease reports whether a and b are both plain MAJOR.MINOR.PATCH
// versions and a is the later one. A range, tag, or prerelease is never
// newer, so the template's pin replaces it.
func newerRelease(a, b string) bool {
	left, okLeft := release(a)
	right, okRight := release(b)

	return okLeft && okRight && slices.Compare(left, right) > 0
}

// release splits a MAJOR.MINOR.PATCH version into its numbers.
func release(version string) ([]int, bool) {
	parts := strings.Split(version, ".")
	if len(parts) != releaseParts {
		return nil, false
	}

	numbers := make([]int, 0, len(parts))

	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || strings.HasPrefix(part, "+") {
			return nil, false
		}

		numbers = append(numbers, n)
	}

	return numbers, true
}
