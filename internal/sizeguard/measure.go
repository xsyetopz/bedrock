package sizeguard

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/xsyetopz/bedrock/internal/execx"
)

// LizardVersion is the lizard release the guard runs.
const LizardVersion = "1.26.1"

// lizardName is the program, and the package that ships it.
const lizardName = "lizard"

// ncssValue is the index of the code-line count among an item's values.
const ncssValue = 1

// errLizard reports a lizard run that failed.
var errLizard = errors.New("`lizard` failed")

// functionMeasure is one function as lizard measures it.
type functionMeasure struct {
	path   string
	name   string
	line   int
	nloc   int
	ccn    int
	params int
}

// fileMeasure is one file as lizard measures it: its code lines, without
// blanks and comments.
type fileMeasure struct {
	path string
	nloc int
}

// measures is what lizard measures under one root.
type measures struct {
	functions []functionMeasure
	files     []fileMeasure
}

// runners are the ways to run the pinned lizard, tried in order. A lizard on
// PATH counts only at that version.
func runners() [][]string {
	return [][]string{
		{lizardName},
		{"uvx", "--from", lizardName + "==" + LizardVersion, lizardName},
		{"pipx", "run", "--spec", lizardName + "==" + LizardVersion, lizardName},
	}
}

// findLizard returns the command that runs lizard LizardVersion, or nil when
// no runner is installed.
func findLizard(ctx context.Context, runner execx.Runner) ([]string, error) {
	for _, candidate := range runners() {
		if !runner.LookPath(candidate[0]) {
			continue
		}

		if len(candidate) > 1 {
			return candidate, nil
		}

		result, err := runner.Run(ctx, execx.Command{Name: candidate[0], Args: []string{"--version"}})
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(string(result.Stdout)) == LizardVersion {
			return candidate, nil
		}
	}

	return nil, nil
}

// measure runs lizard over root once for the functions and once for the
// files.
func measure(
	ctx context.Context,
	runner execx.Runner,
	lizard []string,
	root string,
	languages []string,
) (measures, error) {
	args := make([]string, 0, 2*len(languages)+len(lizard))
	for _, language := range languages {
		args = append(args, "-l", language)
	}

	// bedrock applies the bounds itself, so lizard's own thresholds must not fail the run.
	args = append(args, "--ignore_warnings", "-1", "-t", strconv.Itoa(runtime.NumCPU()))

	formats := []string{"--csv", "--xml"}
	results := make([]execx.Result, len(formats))
	errs := make([]error, len(formats))

	var wait sync.WaitGroup

	for i, format := range formats {
		wait.Go(func() {
			full := append(append(append([]string{}, lizard[1:]...), args...), format, ".")
			results[i], errs[i] = runner.Run(ctx, execx.Command{Dir: root, Name: lizard[0], Args: full})
		})
	}

	wait.Wait()

	for i, result := range results {
		if errs[i] != nil {
			return measures{}, errs[i]
		}

		if !result.OK() {
			return measures{}, fmt.Errorf("%w: %s", errLizard, strings.TrimSpace(string(result.Stderr)))
		}
	}

	functions, err := parseFunctions(results[0].Stdout)
	if err != nil {
		return measures{}, err
	}

	files, err := parseFiles(results[1].Stdout)
	if err != nil {
		return measures{}, err
	}

	return measures{functions: functions, files: files}, nil
}

// relative drops the ./ lizard puts before each path.
func relative(path string) string {
	return strings.TrimPrefix(path, "./")
}

// parseFunctions reads the functions in lizard's --csv output, whose
// columns are nloc, CCN, tokens, parameters, length, location, file, name,
// long name, start, end.
func parseFunctions(data []byte) ([]functionMeasure, error) {
	const columns = 10

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1

	var functions []functionMeasure

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return functions, nil
		}

		if err != nil {
			return nil, fmt.Errorf("read the `lizard` csv: %w", err)
		}

		if len(record) < columns {
			continue
		}

		functions = append(functions, functionMeasure{
			path:   relative(record[6]),
			name:   record[7],
			line:   number(record[9]),
			nloc:   number(record[0]),
			ccn:    number(record[1]),
			params: number(record[3]),
		})
	}
}

func number(text string) int {
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0
	}

	return value
}

// xmlReport is the part of lizard's --xml output the guard reads.
type xmlReport struct {
	Measures []struct {
		Type  string `xml:"type,attr"`
		Items []struct {
			Name   string   `xml:"name,attr"`
			Values []string `xml:"value"`
		} `xml:"item"`
	} `xml:"measure"`
}

// parseFiles reads the files in lizard's --xml output: each item of the File
// measure, whose values are its number, NCSS (code lines), CCN, and function
// count.
func parseFiles(data []byte) ([]fileMeasure, error) {
	// A run over no source files prints nothing.
	if len(bytes.TrimSpace(data)) == 0 {
		return []fileMeasure{}, nil
	}

	var report xmlReport
	if err := xml.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("read the `lizard` xml: %w", err)
	}

	var files []fileMeasure

	for _, measure := range report.Measures {
		if measure.Type != "File" {
			continue
		}

		for _, item := range measure.Items {
			if len(item.Values) <= ncssValue {
				continue
			}

			files = append(files, fileMeasure{path: relative(item.Name), nloc: number(item.Values[ncssValue])})
		}
	}

	return files, nil
}
