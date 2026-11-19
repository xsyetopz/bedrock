package sizeguard

import (
	"fmt"
	"strconv"
)

// Report is what the size guard found: errors fail the guard, and warnings
// only print.
type Report struct {
	Errors   []string
	Warnings []string
}

// hit is one measured value with the bound that applies to it.
type hit struct {
	bound Bound
	at    string
	what  string
	value int
}

// what words a metric in a finding.
func what(metric Metric) string {
	switch metric {
	case FileCodeLines:
		return "file code lines"
	case TestFileCodeLines:
		return "test file code lines"
	case FunctionCodeLines:
		return "code lines"
	case FunctionComplexity:
		return "cyclomatic complexity"
	case FunctionParameters:
		return "parameters"
	default:
		return string(metric)
	}
}

// report sorts each crossed bound into errors or warnings, worded
// `path:line: <what> is <value>, over <limit>`.
func report(hits []hit) Report {
	var result Report

	for _, h := range hits {
		level, limit, over := crossed(h.value, h.bound)
		if !over {
			continue
		}

		finding := fmt.Sprintf("%s: %s is %d, over %d", h.at, h.what, h.value, limit)
		if level == errorLevel {
			result.Errors = append(result.Errors, finding)
		} else {
			result.Warnings = append(result.Warnings, finding)
		}
	}

	return result
}

// judge returns the bounds measured crosses, each path bounded by the
// .editorconfig properties propertiesOf gives it. A file isTest picks takes
// the test file bound in place of the file one.
func judge(
	measured measures,
	propertiesOf func(path string) map[string]string,
	isTest func(path string) bool,
) Report {
	cache := map[string]map[Metric]Bound{}
	boundsAt := func(path string, metric Metric) Bound {
		bounds, ok := cache[path]
		if !ok {
			bounds = boundsOf(propertiesOf(path))
			cache[path] = bounds
		}

		return bounds[metric]
	}

	hits := make([]hit, 0, len(measured.files)+3*len(measured.functions))

	for _, file := range measured.files {
		metric := FileCodeLines
		if isTest(file.path) {
			metric = TestFileCodeLines
		}

		hits = append(
			hits,
			hit{at: file.path, what: what(metric), value: file.nloc, bound: boundsAt(file.path, metric)},
		)
	}

	for _, fn := range measured.functions {
		at := fn.path + ":" + strconv.Itoa(fn.line)
		named := func(metric Metric, value int) hit {
			words := what(metric)
			if fn.name != "" {
				words = "`" + fn.name + "` " + words
			}

			return hit{
				at:    at,
				what:  words,
				value: value,
				bound: boundsAt(fn.path, metric),
			}
		}

		hits = append(hits,
			named(FunctionCodeLines, fn.nloc),
			named(FunctionComplexity, fn.ccn),
			named(FunctionParameters, fn.params))
	}

	return report(hits)
}
