package typescript

import (
	"regexp"
	"slices"
	"strings"
)

func assignment() *regexp.Regexp { return regexp.MustCompile(`^[A-Za-z_]\w*=`) }

func separator() *regexp.Regexp { return regexp.MustCompile(`\s*(?:&&|\|\||[;|])\s*`) }

func lowerStart() *regexp.Regexp { return regexp.MustCompile(`^[a-z]`) }

// rewriteScript moves each command of a package.json script onto Bun and
// bedrock's tools, keeping the separators between the commands.
func rewriteScript(command string) string {
	var out strings.Builder

	last := 0

	for _, at := range separator().FindAllStringIndex(command, -1) {
		out.WriteString(rewritePart(command[last:at[0]]))
		out.WriteString(command[at[0]:at[1]])

		last = at[1]
	}

	out.WriteString(rewritePart(command[last:]))

	return out.String()
}

// rewritePart returns part unchanged unless a known tool starts it.
func rewritePart(part string) string {
	tokens := strings.Fields(part)
	if rewritten, changed := rewriteCommand(tokens); changed {
		return strings.Join(rewritten, " ")
	}

	return part
}

// rewriteCommand rewrites the tool after any leading `NAME=value` words.
func rewriteCommand(tokens []string) ([]string, bool) {
	start := slices.IndexFunc(tokens, func(token string) bool { return !assignment().MatchString(token) })
	if start < 0 {
		return tokens, false
	}

	handled, ok := rewriteTool(tokens[start], tokens[start+1:])
	if !ok {
		return tokens, false
	}

	return append(slices.Clone(tokens[:start]), handled...), true
}

func withFix(args []string) []string {
	if slices.Contains(args, "--fix") || slices.Contains(args, "--write") {
		return []string{"--fix"}
	}

	return nil
}

func runOnBun(args []string) []string {
	if len(args) > 0 && args[0] == "watch" {
		return append([]string{nameBun, "--watch"}, args[1:]...)
	}

	return append([]string{nameBun}, args...)
}

// head splits a tool's arguments into the subcommand and the rest.
func head(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}

	return args[0], args[1:]
}

// rewriteTool maps one tool and its arguments onto the Bun command, and
// reports false for a tool bedrock leaves alone.
func rewriteTool(tool string, args []string) ([]string, bool) {
	if handler, ok := simpleTools()[tool]; ok {
		return handler(args), true
	}

	switch tool {
	case "biome":
		sub, rest := head(args)
		if sub == "format" {
			return append([]string{nameBun, nameFmt}, formatCheck(!slices.Contains(rest, "--write"))...), true
		}

		return append([]string{nameOxlint}, withFix(rest)...), true
	case "cross-env":
		inner, _ := rewriteCommand(args)

		return inner, true
	case nameNpm, namePnpm, "yarn":
		return packageManager(tool, args), true
	case "npx", "pnpx":
		return execute(args), true
	case "prettier":
		return prettier(args), true
	case "tsc":
		return tsc(args), true
	case "typescript-bedrock":
		sub, rest := head(args)
		if sub == "guard" {
			return []string{"just", "typescript::guard"}, true
		}

		return append([]string{"just", "bedrock", sub}, rest...), true
	case "vitest":
		if len(args) > 0 && args[0] == nameRun {
			args = args[1:]
		}

		return append([]string{nameBun, nameTest}, args...), true
	default:
		return nil, false
	}
}

// simpleTools are the tools that map to a fixed command plus their arguments.
func simpleTools() map[string]func(args []string) []string {
	return map[string]func(args []string) []string{
		"esno":        runOnBun,
		"eslint":      func(args []string) []string { return append([]string{nameOxlint}, withFix(args)...) },
		nameJest:      func(args []string) []string { return append([]string{nameBun, nameTest}, args...) },
		"node":        runOnBun,
		"oxfmt":       func(args []string) []string { return append([]string{nameBun, nameFmt}, args...) },
		"rimraf":      func(args []string) []string { return append([]string{"rm", "-rf"}, args...) },
		"ts-node":     runOnBun,
		"ts-node-dev": runOnBun,
		"tsx":         runOnBun,
	}
}

func formatCheck(check bool) []string {
	if check {
		return []string{"--check"}
	}

	return nil
}

func prettier(args []string) []string {
	checks := []string{"--check", "-c", "--list-different", "-l"}
	dropped := append([]string{"--write", "-w", "."}, checks...)
	out := []string{nameBun, nameFmt}
	out = append(
		out,
		formatCheck(slices.ContainsFunc(args, func(a string) bool { return slices.Contains(checks, a) }))...)

	for _, arg := range args {
		if !slices.Contains(dropped, arg) {
			out = append(out, arg)
		}
	}

	return out
}

func tsc(args []string) []string {
	if !slices.Contains(args, "--noEmit") {
		return append([]string{"tsc"}, args...)
	}

	out := []string{nameBun, "check"}

	for _, arg := range args {
		if arg != "--noEmit" {
			out = append(out, arg)
		}
	}

	return out
}

// execute rewrites `npx` and `dlx`: a known tool runs as itself, anything
// else through bunx.
func execute(args []string) []string {
	rest := slices.DeleteFunc(slices.Clone(args), func(arg string) bool { return arg == "--yes" || arg == "-y" })
	if len(rest) > 0 {
		if rewritten, ok := rewriteTool(rest[0], rest[1:]); ok {
			return rewritten
		}
	}

	return append([]string{"bunx"}, rest...)
}

func packageManager(tool string, args []string) []string {
	sub, args := head(args)

	switch {
	case slices.Contains([]string{"ci", "i", nameInstall}, sub):
		frozen := sub == "ci" || slices.Contains(args, "--frozen-lockfile") || slices.Contains(args, "--immutable")
		if frozen {
			return []string{nameBun, nameInstall, "--frozen-lockfile"}
		}

		return []string{nameBun, nameInstall}
	case slices.Contains([]string{"add", "publish", "remove"}, sub):
		return append([]string{nameBun, sub}, args...)
	case slices.Contains([]string{nameRun, "run-script"}, sub):
		return append([]string{nameBun, nameRun}, args...)
	case slices.Contains([]string{"t", nameTest}, sub):
		return append([]string{nameBun, nameRun, nameTest}, args...)
	case slices.Contains([]string{"dlx", "exec"}, sub):
		return execute(args)
	case tool != nameNpm && lowerStart().MatchString(sub):
		// `pnpm build` and `yarn build` run scripts; npm has no such shorthand.
		return append([]string{nameBun, nameRun, sub}, args...)
	default:
		return append([]string{tool, sub}, args...)
	}
}
