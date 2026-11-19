package dotnet

import (
	"context"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// foreignConfigs would override what `.editorconfig` gives the formatters: a
// `.csharpierrc` makes CSharpier skip `.editorconfig` altogether, and
// Fantomas dropped `.fantomasrc` for `.editorconfig` in 4.0.
func foreignConfigs() []string {
	return []string{".csharpierrc", ".csharpierrc.json", ".csharpierrc.yaml", ".csharpierrc.yml", ".fantomasrc"}
}

// suppressions finds suppressions that give no reason.
type suppressions struct {
	pragma     *regexp.Regexp
	suppress   *regexp.Regexp
	nowarn     *regexp.Regexp
	hasComment *regexp.Regexp
}

func newSuppressions() suppressions {
	return suppressions{
		pragma:     regexp.MustCompile(`^\s*#pragma\s+warning\s+disable\b`),
		suppress:   regexp.MustCompile(`\bSuppressMessage\s*\(`),
		nowarn:     regexp.MustCompile(`^\s*#nowarn\b`),
		hasComment: regexp.MustCompile(`//\s*\S`),
	}
}

// unexplained reports whether rest, the line after a directive, gives no
// reason in a trailing comment. RE2 has no look-ahead, so the directive is
// matched first and the rest checked here.
func (s suppressions) unexplained(line string, directive *regexp.Regexp) bool {
	loc := directive.FindStringIndex(line)

	return loc != nil && !s.hasComment.MatchString(line[loc[1]:])
}

// csharp is the message for a C# line with a suppression that gives no
// reason: a `#pragma warning disable` without a trailing comment, or a
// `SuppressMessage` attribute without `Justification`. It is empty when the
// line is fine.
func (s suppressions) csharp(line string) string {
	switch {
	case s.unexplained(line, s.pragma):
		return "`#pragma warning disable` without a reason; add `// <reason>` after the ids"
	case s.suppress.MatchString(line) && !strings.Contains(line, "Justification"):
		return "`SuppressMessage` without `Justification`; give the reason in `Justification = \"...\"`"
	default:
		return ""
	}
}

// fsharp is the message for an F# `#nowarn` that gives no reason in a
// trailing comment, or empty.
func (s suppressions) fsharp(line string) string {
	if s.unexplained(line, s.nowarn) {
		return "`#nowarn` without a reason; add `// <reason>` after the ids"
	}

	return ""
}

// findings are the findings of one source file, as
// `path:line: message`, one per unexplained suppression.
func (s suppressions) findings(file, text string) []language.Finding {
	check := s.fsharp
	if strings.HasSuffix(file, ".cs") || strings.HasSuffix(file, ".csx") {
		check = s.csharp
	}

	var found []language.Finding

	for index, line := range strings.Split(text, "\n") {
		if message := check(line); message != "" {
			found = append(found, file+":"+strconv.Itoa(index+1)+": "+message)
		}
	}

	return found
}

// Guard reports what the SDK analyzers and FSharpLint cannot: a suppression
// that gives no reason, a formatter config that would shadow `.editorconfig`,
// and a second tool manifest, which `dotnet tool restore` would read beside
// the first.
func (Dotnet) Guard(ctx context.Context, c *language.Context, _ bool) ([]language.Finding, error) {
	sources, err := listMatching(ctx, c, "**/*.{cs,csx,fs,fsi,fsx}")
	if err != nil {
		return nil, err
	}

	all, err := listMatching(ctx, c, "**")
	if err != nil {
		return nil, err
	}

	rules := newSuppressions()

	var findings []language.Finding

	for _, source := range sources {
		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(source)))
		if readErr != nil {
			return nil, readErr
		}

		findings = append(findings, rules.findings(source, text)...)
	}

	for _, file := range all {
		if slices.Contains(foreignConfigs(), path.Base(file)) {
			findings = append(findings, file+
				": the formatters read their layout from `.editorconfig`; move the settings there and delete this file")
		}
	}

	manifests := slices.DeleteFunc(
		toolManifests(),
		func(manifest string) bool { return !slices.Contains(all, manifest) },
	)
	if len(manifests) > 1 {
		findings = append(findings, strings.Join(manifests, ", ")+": two tool manifests; keep the tools in `"+
			manifests[0]+"` and delete the other")
	}

	return findings, nil
}
