package cpp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// swiftDescribe is the part of `swift package describe` the entries read.
type swiftDescribe struct {
	CxxLanguageStandard string `json:"cxx_language_standard"`
	Targets             []struct {
		ModuleType string `json:"module_type"`
		Path       string `json:"path"`
		Sources    strs   `json:"sources"`
	} `json:"targets"`
}

// swiftInfo is the part of `swift -print-target-info` the entries read.
type swiftInfo struct {
	Paths struct {
		RuntimeResourcePath string `json:"runtimeResourcePath"`
	} `json:"paths"`
}

// languageFlags returns the compiler and language flags for a source's
// extension.
func languageFlags(extension, standard string) []string {
	switch extension {
	case ".c":
		return []string{"clang", "-std=c17"}
	case ".m":
		return []string{"clang", "-fobjc-arc"}
	case ".mm":
		return []string{"clang++", "-std=" + standard, "-fobjc-arc"}
	default:
		return []string{"clang++", "-std=" + standard}
	}
}

// swiftEntries returns one entry per C, C++, and Objective-C source in each
// Clang target, with the include paths and system roots a build would pass.
func swiftEntries(root string, describe swiftDescribe, info swiftInfo, sdk string) []map[string]any {
	// <swift/bridging> lives in the toolchain's usr/include, beside usr/lib/swift.
	toolchainInclude := filepath.Join(info.Paths.RuntimeResourcePath, "..", "..", "include")

	standard := describe.CxxLanguageStandard
	if standard == "" {
		standard = "c++20"
	}

	var common []string
	if sdk != "" {
		common = append(common, "-isysroot", sdk)
	}

	common = append(common, "-isystem", toolchainInclude, "-mmacosx-version-min=13.0")

	for _, t := range describe.Targets {
		if t.ModuleType == "ClangTarget" {
			common = append(common, "-I", root+"/"+t.Path+"/include")
		}
	}

	entries := []map[string]any{}

	for _, t := range describe.Targets {
		if t.ModuleType != "ClangTarget" {
			continue
		}

		for _, source := range t.Sources {
			file := root + "/" + t.Path + "/" + source
			arguments := append(languageFlags(filepath.Ext(source), standard), common...)
			arguments = append(arguments, "-c", file)
			entries = append(entries, map[string]any{"arguments": arguments, "directory": root, "file": file})
		}
	}

	return entries
}

// swiftJSON runs a Swift command and decodes its JSON output.
func swiftJSON(ctx context.Context, c *language.Context, into any, args ...string) error {
	result := run(ctx, c, c.Root, args[0], args[1:]...)
	if result.code != 0 {
		return toolFailed("%s failed: %s", strings.Join(args, " "), firstLine(result.stderr))
	}

	return json.Unmarshal([]byte(result.stdout), into)
}

// packageEntries returns the entries of the Clang targets of the SwiftPM
// package at the root, if any.
func packageEntries(ctx context.Context, c *language.Context) ([]map[string]any, error) {
	if !fsutil.Exists(filepath.Join(c.Root, "Package.swift")) {
		return nil, nil
	}

	var (
		describe swiftDescribe
		info     swiftInfo
	)

	if err := swiftJSON(
		ctx,
		c,
		&describe,
		"swift",
		"package",
		"--disable-sandbox",
		"describe",
		"--type",
		"json",
	); err != nil {
		return nil, err
	}

	if err := swiftJSON(ctx, c, &info, "swift", "-print-target-info"); err != nil {
		return nil, err
	}

	sdk := ""
	if result := run(ctx, c, c.Root, "xcrun", "--show-sdk-path"); result.code == 0 {
		sdk = strings.TrimSpace(result.stdout)
	}

	return swiftEntries(c.Root, describe, info, sdk), nil
}
