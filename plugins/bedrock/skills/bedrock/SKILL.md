---
name: bedrock
description: Run bedrock in this repo and finish the job it starts. Adopts a repo bedrock does not manage yet, updates one it does, and fixes what `just verify` and `bedrock guard` report. Use when the user types /bedrock, or asks to set up, update, or guard a repo with bedrock.
argument-hint: "[adopt|init|update|guard|clean|languages] [args]"
allowed-tools: mcp__bedrock__run
---

# Bedrock

Arguments: `$ARGUMENTS`

The `mcp__bedrock__run` tool runs the bedrock CLI in the session's folder. Its `args` is the command line without `bedrock`, such as `adopt ts rust` or `guard`. `adopt`, `init`, and `update` print their plan and change nothing until the line has `--yes`.

## Pick the Command

- With arguments, run that command line.
- Without arguments, read `.editorconfig`. A `bedrock_version` in it means bedrock manages the repo: run `update`. Without one, run `adopt`.

## Apply a Plan

A plan lists the files bedrock would write or remove.

1. Summarize the plan for the user in a few lines: the languages, the files that change, the tools it removes, and the notes it prints.
2. Apply it with the same line plus `--yes` only when the arguments already had `--yes`, or the user agrees after reading the summary.
3. A plan with no steps needs nothing applied: say the repo is current and go on to the checks.

Adopt refuses a file with uncommitted work. Show the user the files it names and ask before passing `--allow-dirty`.

## Check the Result

After an applied plan, and for a run without arguments, run `just setup` and then `just verify` in the repo, and `guard` through the tool.

- Fix each error at its cause in the code.
- Keep the shared configs as bedrock wrote them, since the next `update` rewrites them.
- Move a step a removed Makefile or hook manager ran into `project.just`.
- When a recipe reports a missing tool that `just setup` cannot install, tell the user which tool and how to install it.

Finish with what changed, what you fixed, and what still fails.
