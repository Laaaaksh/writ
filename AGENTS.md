# Project agent memory

This file is the project's committed home for project-intrinsic agent knowledge: build, test, release, architecture, and sharp-edge notes that should travel with the code.

- Add durable project-specific notes here as they are discovered through real work.
- Module path: `github.com/Laaaaksh/writ`. Only dependencies allowed: `spf13/cobra` and `BurntSushi/toml` — do not add others without a decision.
- `internal/writ` (`writ.go`) and `internal/gate`/`internal/render` are fully implemented. `internal/drift` and `internal/evidence` remain stubs (return `"not implemented"`) with signatures matching a shared contract — do not change exported signatures without a decision.
- `.writ/current.toml` is runtime output (gitignored), never committed. Package-level constant `writPath` in `internal/writ/writ.go` is the source of truth for its location.
- `cmd/writ` commands signal a specific process exit code (0 auto-mergeable, 1 needs human, 2 no writ open) by returning the unexported `exitCodeErr{code}` from `RunE`; `main()` unwraps it via `errors.As` before falling back to the default exit-1 path. Command tests that need a real repo `os.Chdir` into a `t.TempDir()` git repo, since `repoRoot()` shells out to `git rev-parse --show-toplevel` against the process cwd rather than taking a directory argument.
- This repo is on the captain's **personal** GitHub account (`Laaaaksh`), while the shell defaults to a work `gh` profile. GitHub CLI operations need `GH_CONFIG_DIR="$HOME/.config/gh-personal"` exported first; verify with `gh api user --jq .login` (must print `Laaaaksh`). The default branch is `master`, not `main`.
- A repo commit-identity guard hook requires the git author email to match the personal class (`laksh.sadhwani07@gmail.com`) for this remote — set via local (not global) `git config user.email` if a commit is rejected.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
