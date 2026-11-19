// Package execx runs external programs, such as git, go, or bun, behind one
// small interface, so every caller bounds a run by its context and tells a
// program that failed from one that could not start.
package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// Command describes one program run.
type Command struct {
	// Stdin feeds the program; nil gives it no input.
	Stdin io.Reader
	// Dir is the working directory; "" means the current one.
	Dir string
	// Name is the program, looked up on PATH.
	Name string
	// Args are the arguments after the program name.
	Args []string
	// Env adds KEY=value pairs to the inherited environment.
	Env []string
	// Unset names inherited variables the program does not see, such as
	// those HookGitVars returns.
	Unset []string
	// Stream sends the program's output to the terminal instead of
	// capturing it, for installs a person watches, such as bun install.
	Stream bool
}

// Result is what a finished program left.
type Result struct {
	Stdout []byte
	Stderr []byte
	// Code is the exit status; 0 means success.
	Code int
}

// OK reports whether the program exited with status 0.
func (r Result) OK() bool {
	return r.Code == 0
}

// Runner starts programs. System runs real ones; tests pass the same type,
// since bedrock's checks exercise the real toolchain.
type Runner interface {
	// Run waits for the program. A non-zero exit is a Result, not an error;
	// the error reports a program that could not start or was cancelled.
	Run(ctx context.Context, command Command) (Result, error)
	// LookPath reports whether a program is on PATH.
	LookPath(name string) bool
}

// System runs programs on this machine.
type System struct {
	// Stdout and Stderr receive streamed output; nil means the process's own.
	Stdout io.Writer
	Stderr io.Writer
}

// Run starts the program and waits for it to exit.
func (s System) Run(ctx context.Context, command Command) (Result, error) {
	//nolint:gosec // G204: bedrock runs fixed toolchain programs with arguments it builds itself.
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	cmd.Stdin = command.Stdin

	if len(command.Env) > 0 || len(command.Unset) > 0 {
		cmd.Env = environ(command.Unset, command.Env)
	}

	var stdout, stderr bytes.Buffer
	if command.Stream {
		cmd.Stdout = writerOr(s.Stdout, os.Stdout)
		cmd.Stderr = writerOr(s.Stderr, os.Stderr)
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Code: 0}

	var exit *exec.ExitError
	if errors.As(err, &exit) && ctx.Err() == nil {
		result.Code = exit.ExitCode()

		return result, nil
	}

	if err != nil {
		return result, fmt.Errorf("run `%s`: %w", command.Name, err)
	}

	return result, nil
}

// LookPath reports whether name is an executable on PATH.
func (System) LookPath(name string) bool {
	_, err := exec.LookPath(name)

	return err == nil
}

// FailedError reports a program that exited with a non-zero status.
type FailedError struct {
	Name   string
	Args   []string
	Stderr string
	Code   int
}

func (e *FailedError) Error() string {
	message := fmt.Sprintf("%s %s exited with %d", e.Name, strings.Join(e.Args, " "), e.Code)
	if e.Stderr != "" {
		message += ": " + e.Stderr
	}

	return message
}

// Output runs the program and returns its stdout, or an error when it fails
// to start or exits non-zero.
func Output(ctx context.Context, runner Runner, command Command) (string, error) {
	result, err := runner.Run(ctx, command)
	if err != nil {
		return "", err
	}

	if !result.OK() {
		return "", &FailedError{
			Name:   command.Name,
			Args:   command.Args,
			Stderr: strings.TrimSpace(string(result.Stderr)),
			Code:   result.Code,
		}
	}

	return string(result.Stdout), nil
}

// HookGitVars returns the variables git exports to a hook that pin every git
// run to the hook's repository. A hook in a worktree sets them, so a program
// that runs git in another directory, such as SwiftPM fetching a package or
// a listing of a nested project, unsets them.
func HookGitVars() []string {
	return []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"}
}

// environ returns the inherited environment without the unset names, plus extra.
func environ(unset, extra []string) []string {
	env := slices.DeleteFunc(os.Environ(), func(pair string) bool {
		name, _, _ := strings.Cut(pair, "=")

		return slices.Contains(unset, name)
	})

	return append(env, extra...)
}

func writerOr(writer, fallback io.Writer) io.Writer {
	if writer != nil {
		return writer
	}

	return fallback
}
