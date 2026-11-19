package cpp

import "maps"

// target is one build target, as a build system's own query reports it.
type target struct {
	id string
	// sources it compiles, relative to the repository root.
	sources []string
	// deps are the ids of the targets it links or depends on.
	deps []string
	// test is whether a test the build system registers runs this target.
	test bool
	// installed is whether the build installs it, which makes it product code.
	installed bool
}

// split is the sources only test-only targets compile, and those some other
// target compiles.
type split struct {
	test  map[string]bool
	other map[string]bool
}

// splitSources splits the sources of targets by the build's own view of
// tests: a target a test runs is test-only, and so is an uninstalled one that
// only test-only targets depend on, such as a fixture library. A source
// compiled by any other target is product code.
func splitSources(targets []target) split {
	dependents := map[string][]string{}

	for _, t := range targets {
		for _, dep := range t.deps {
			dependents[dep] = append(dependents[dep], t.id)
		}
	}

	byID := map[string]target{}
	for _, t := range targets {
		byID[t.id] = t
	}

	classifier := testOnly{byID: byID, dependents: dependents, memo: map[string]bool{}}
	test, other := map[string]bool{}, map[string]bool{}

	for _, t := range targets {
		into := other
		if classifier.of(t.id, map[string]bool{}) {
			into = test
		}

		for _, source := range t.sources {
			into[source] = true
		}
	}

	for source := range test {
		if other[source] {
			delete(test, source)
		}
	}

	return split{test: test, other: other}
}

// testOnly decides which targets only tests use.
type testOnly struct {
	byID       map[string]target
	dependents map[string][]string
	memo       map[string]bool
}

// of reports whether the target id is test-only; seen are the targets on the
// path being followed, so a dependency cycle ends without marking it.
func (c testOnly) of(id string, seen map[string]bool) bool {
	t, ok := c.byID[id]
	if !ok || seen[id] {
		return false
	}

	if t.test {
		return true
	}

	if known, memoized := c.memo[id]; memoized {
		return known
	}

	users := c.dependents[id]
	next := maps.Clone(seen)
	next[id] = true

	result := !t.installed && len(users) > 0

	for _, user := range users {
		result = result && c.of(user, next)
	}

	c.memo[id] = result

	return result
}
