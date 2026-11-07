# Security Policy

## Report a Vulnerability

Use GitHub private vulnerability reporting. Open the repository on GitHub, go to the **Security** tab, and choose **Report a vulnerability**.

Do not open a public issue for a vulnerability.

Include the affected file or command, the steps to reproduce, and the impact.

## Scope

This repo is a template. It ships configuration, git hooks, and the `bun-bedrock` package, whose `adopt` command rewrites files in a target repo. Reports about these are in scope:

- The CLI writing or deleting files outside the target repo.
- A hook or script running code you did not expect.
- A configuration that weakens a protection it claims to give.

Vulnerabilities in Bun, oxlint, or other dependencies belong to those projects.

## Supported Versions

Only the latest commit on the default branch is supported.
