package jvm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

// usageError is the exit code for a command line that names nothing to do.
const usageError = 2

// gradlew runs one Gradle task line in the repository with the JDK found and
// returns its exit code.
func gradlew(ctx context.Context, c *language.Context, found jdk, args ...string) (int, error) {
	result, err := c.Runner.Run(ctx, execx.Command{
		Stdin:  nil,
		Dir:    c.Root,
		Name:   filepath.Join(c.Root, "gradlew"),
		Args:   append([]string{"--console=plain"}, args...),
		Env:    found.env(),
		Stream: true,
	})
	if err != nil {
		return 1, fmt.Errorf("run `gradlew`: %w", err)
	}

	return result.Code, nil
}

// j2k converts Java files, or directories of them, to Kotlin with IntelliJ
// IDEA's J2K, then formats the result. The first run downloads IntelliJ IDEA,
// about 1.5 GB; `BEDROCK_IDEA` points at an installed one.
func j2k(ctx context.Context, c *language.Context, args []string) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(c.Stderr, "`jvm j2k` takes the Java files or directories to convert")

		return usageError, nil
	}

	found, ok := findJdk(ctx, c.Runner, os.Getenv("JAVA_HOME"))
	if !ok {
		fmt.Fprintln(c.Stderr, jdkMissing)

		return 1, nil
	}

	tasks := []string{"j2k"}
	for _, file := range args {
		tasks = append(tasks, "--file="+file)
	}

	code, err := gradlew(ctx, c, found, tasks...)
	if err != nil || code != 0 {
		return code, err
	}

	return gradlew(ctx, c, found, "spotlessApply")
}
