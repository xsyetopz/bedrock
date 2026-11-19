package sizeguard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/editorconfig"
)

// Metric is one size the guard bounds, read from .editorconfig as
// bedrock_<metric>_warning and bedrock_<metric>_error. The language blocks
// set the defaults: functions over 56 code lines went with failed LLM
// changes (Chong et al., EASE 2026); 15 is the cyclomatic complexity lizard
// and the TypeScript config use; 7 parameters is Clippy's too_many_arguments.
// File length only warns at 603: no study shows a file-length optimum, and
// splitting files has measured costs (Doc2Harness, 2026). A test file, which
// a language that tells its tests apart by their code names, takes
// test_file_code_lines in place of file_code_lines.
type Metric string

// The metrics the guard bounds.
const (
	FileCodeLines      Metric = "file_code_lines"
	TestFileCodeLines  Metric = "test_file_code_lines"
	FunctionCodeLines  Metric = "function_code_lines"
	FunctionComplexity Metric = "function_complexity"
	FunctionParameters Metric = "function_parameters"
)

const (
	warningLevel   = "warning"
	errorLevel     = "error"
	boundKeyPrefix = "bedrock_"
)

// metrics lists every bounded metric, in the order the guard names them.
func metrics() []Metric {
	return []Metric{FileCodeLines, TestFileCodeLines, FunctionCodeLines, FunctionComplexity, FunctionParameters}
}

// levels are the two bounds of a metric.
func levels() []string {
	return []string{warningLevel, errorLevel}
}

// Bound is a limit on one metric: crossing Error fails the guard, crossing
// Warning only reports. Zero means no limit, since a bound is a positive integer.
type Bound struct {
	Warning int
	Error   int
}

// boundKey returns the metric and level a bedrock_<metric>_<level> key names.
func boundKey(key string) (Metric, string, bool) {
	for _, metric := range metrics() {
		for _, level := range levels() {
			if key == boundKeyPrefix+string(metric)+"_"+level {
				return metric, level, true
			}
		}
	}

	return "", "", false
}

// boundsOf returns the bounds the bedrock_* properties of one file set.
func boundsOf(properties map[string]string) map[Metric]Bound {
	bounds := make(map[Metric]Bound)

	for key, value := range properties {
		metric, level, ok := boundKey(key)
		limit, err := strconv.Atoi(value)

		if !ok || err != nil || limit <= 0 {
			continue
		}

		bound := bounds[metric]
		if level == errorLevel {
			bound.Error = limit
		} else {
			bound.Warning = limit
		}

		bounds[metric] = bound
	}

	return bounds
}

// invalidBounds returns what is wrong with the size-bound pairs of an
// .editorconfig: a bound that is not a positive integer or unset, or a
// bedrock_*_warning or bedrock_*_error key that names no metric; each worded
// from file.
func invalidBounds(pairs []editorconfig.Pair, file string) []string {
	var problems []string

	for _, pair := range pairs {
		if _, _, ok := boundKey(pair.Key); ok {
			if !positiveOrUnset(pair.Value) {
				problems = append(problems, fmt.Sprintf(
					"%s: `%s` = %s; a bound is a positive integer or `unset`", file, pair.Key, pair.Value))
			}

			continue
		}

		if namesBound(pair.Key) {
			names := make([]string, 0, len(metrics()))
			for _, metric := range metrics() {
				names = append(names, "`"+string(metric)+"`")
			}

			problems = append(problems, fmt.Sprintf(
				"%s: `%s` names no bound; bedrock bounds %s, each with `_warning` or `_error`",
				file, pair.Key, strings.Join(names, ", ")))
		}
	}

	return problems
}

// positiveOrUnset reports whether value is a positive integer without a
// sign or leading zero, or unset in any case.
func positiveOrUnset(value string) bool {
	if strings.EqualFold(value, "unset") {
		return true
	}

	if value == "" || value[0] == '0' {
		return false
	}

	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}

// namesBound reports whether key has the shape of a bound, bedrock_<name>_warning
// or bedrock_<name>_error, whatever the name.
func namesBound(key string) bool {
	rest, ok := strings.CutPrefix(key, boundKeyPrefix)
	if !ok {
		return false
	}

	for _, level := range levels() {
		if name, found := strings.CutSuffix(rest, "_"+level); found && name != "" {
			return true
		}
	}

	return false
}

// crossed returns which bound value crosses, if any; an error outranks a warning.
func crossed(value int, bound Bound) (string, int, bool) {
	if bound.Error > 0 && value > bound.Error {
		return errorLevel, bound.Error, true
	}

	if bound.Warning > 0 && value > bound.Warning {
		return warningLevel, bound.Warning, true
	}

	return "", 0, false
}
