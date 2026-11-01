# Security policy

## Supported versions

Only the [latest tagged release](https://github.com/Laaaaksh/writ/releases) is
supported with security fixes. If you are running an older version, upgrade first
and see whether the problem still reproduces.

## Reporting a vulnerability

Please do **not** open a public issue for anything you believe is security-relevant.

Use GitHub's private vulnerability reporting:

https://github.com/Laaaaksh/writ/security/advisories/new

Please include what you can of:

- the writ version or commit you tested against (`writ version`)
- the steps to reproduce, ideally starting from a fresh `git init` repo
- your assessment of the impact

## Scope notes

writ is a single static binary that shells out to `git` (every command) and POSIX
`sh` (verification commands and `$EDITOR` launches). Anything where a repository,
a writ file, a verification command, or an agent-proposed draft causes writ to
behave dangerously beyond what the human approved is in scope - including cases
where writ's own state under `.writ/` is used to defeat its approval or drift
checks.
