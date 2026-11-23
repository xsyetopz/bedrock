---
name: adopt
description: Move this repo onto bedrock's setup for its languages, then fix what the new checks report. Use when the user types /adopt, or asks to adopt bedrock in a repo.
argument-hint: "[<lang>...] [--yes]"
allowed-tools: mcp__bedrock__run
---

# Adopt

Arguments: `$ARGUMENTS`

Follow the `bedrock` skill with the command line `adopt $ARGUMENTS`, adding no language or option the user did not name: run it through `mcp__bedrock__run`, summarize the plan, apply it with `--yes` when the arguments had it or the user agrees, then run `just setup`, `just verify`, and `guard`, and fix what they report.
