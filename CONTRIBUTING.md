# Contributing to writ

Thanks for wanting to contribute. writ verifies that coding agents stay inside an agreed
scope: a human-approved writ declares an intent, checkable acceptance criteria, a declared
file scope, and a verification command, and writ shows the human only what fell outside
that scope - the drift. It is a single Go CLI; its only runtime requirements are `git` and
a POSIX `sh`, so building and testing it works on any machine with a Go toolchain.

## Contribution workflow

The default branch `master` is protected: every change lands through a pull request,
required status checks must pass, and protection is enforced for everyone - including the
maintainer. There are no direct pushes to `master`.

1. Fork the repo on GitHub, then clone your fork:
   ```bash
   git clone https://github.com/<your-username>/writ.git
   cd writ
   ```
2. Create a descriptively named feature branch off `master`.
3. Make your changes as small, focused commits.
4. Verify locally - all three must pass:
   ```bash
   go build ./...
   go vet ./...
   go test ./...
   ```
5. Push the branch to your fork.
6. Open a pull request against `master` here.

A PR can merge only when the `Test` check passes and all conversation threads are resolved.
CI runs the same build, vet, and test steps on every pull request and on pushes to `master`.

## Scope and dependencies

- The command surface follows a fixed lifecycle: `propose` -> `approve` -> implement ->
  `attest`/`unattest` -> `status`/`merge`, with `discard` removing an unwanted open writ.
  Changes that alter this flow, the exit-code contract, or any exported signature need
  agreement in an issue before they land.
- `status` and `merge` are meant to run from the writ's own feature branch, never from
  `base`, and `merge` refuses a dirty working tree. Keep these guarantees intact in any
  change to those commands.

- The module path is `github.com/Laaaaksh/writ`. Only `spf13/cobra` and `BurntSushi/toml`
  are allowed as dependencies; do not add others without prior agreement.
- If you are a coding agent working in this repo, read [AGENTS.md](AGENTS.md) first. It
  documents the lifecycle commands and invariants your changes must respect, including
  that `.writ/current.toml` is runtime state and is never committed.

## Reporting issues

Please open a GitHub issue before starting large changes or proposing new features, so the
design can be discussed before code is written. Bug reports should include:
- Your OS and how you installed writ (Homebrew tap or `go install`)
- `writ version` output
- Steps to reproduce, ideally including the `.writ/current.toml` involved
  (redact anything private)
- What you expected vs what happened

For security-sensitive behavior, see [SECURITY.md](SECURITY.md).
