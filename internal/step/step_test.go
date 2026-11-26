package step_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xsyetopz/bedrock/internal/step"
)

// done writes onDisk at a file and reports whether a Write of template is done.
func done(t *testing.T, onDisk, template string) bool {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gradlew.bat"), []byte(onDisk), 0o600); err != nil {
		t.Fatal(err)
	}

	ok, err := step.Done(root, step.Write{Path: "gradlew.bat", Content: []byte(template)})
	if err != nil {
		t.Fatal(err)
	}

	return ok
}

func TestAWriteIsDoneWhenGitCheckedItOutWithCRLF(t *testing.T) {
	t.Parallel()

	// Arrange: `*.bat eol=crlf` checks the LF file bedrock wrote out with CRLF.
	onDisk, template := "@echo off\r\nexit /b 0\r\n", "@echo off\nexit /b 0\n"

	// Act
	ok := done(t, onDisk, template)

	// Assert
	if !ok {
		t.Fatal("a CRLF checkout of the template is reported as drift")
	}
}

func TestAWriteWithOtherTextIsNotDone(t *testing.T) {
	t.Parallel()

	// Arrange
	onDisk, template := "@echo on\r\nexit /b 0\r\n", "@echo off\nexit /b 0\n"

	// Act
	ok := done(t, onDisk, template)

	// Assert
	if ok {
		t.Fatal("a changed file is reported as written")
	}
}

func TestABinaryWriteMustMatchByteForByte(t *testing.T) {
	t.Parallel()

	// Arrange: a NUL byte marks binary content, whose CR bytes are data.
	onDisk, template := "PK\x00\r\n", "PK\x00\n"

	// Act
	ok := done(t, onDisk, template)

	// Assert
	if ok {
		t.Fatal("a binary file that differs in a CR byte is reported as written")
	}
}
