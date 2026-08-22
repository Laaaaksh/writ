# Changelog

All notable changes to writ are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- FAQ answering what AI-coding-agent users actually ask: writ needs no AI subscription or API key, works with any agent that can pipe TOML and run shell commands, allows one open writ per repo (parallel agents need separate git worktrees), and each worktree should use its own provider key or subscription because sharing one invites rate-limit retry storms.
- Real rendered `$ writ status` example output and a pasteable agent-discipline snippet for your repo's `AGENTS.md` / `CLAUDE.md` driving the propose → implement → attest loop.
- Feature-request issue template alongside the bug template.
- CI runs a gofmt gate inside the required `Test` check and a dedicated macOS job covering the darwin/arm64 release target.
- This changelog.

## [0.2.4] - 2026-08-22

### Security

- Released binaries now build with a supported Go toolchain (`go.mod`'s `go` directive bumped off EOL Go 1.22); both workflows resolve their compiler from that directive.
- CodeQL code scanning added as an advanced-setup workflow covering Go and GitHub Actions.

### Fixed

- Generated Homebrew formula now passes `brew style` and `brew audit --strict`.

### Changed

- Documented the fork-and-PR contribution flow under branch protection in `CONTRIBUTING.md`.
- Pinned `goreleaser-action` to `~> v2` instead of the deprecated floating latest.

## [0.2.3] - 2026-08-22

### Fixed

- `writ merge` from a linked git worktree printed an Auto-mergeable verdict and then died on git's raw "'main' is already used by worktree" fatal; it now refuses up front with detach-or-remove instructions.

### Security

- Added `SECURITY.md` with a private vulnerability-disclosure channel; enabled Dependabot vulnerability alerts, automated security fixes, and private vulnerability reporting.
- Weekly dependency-update PRs via `.github/dependabot.yml` (`gomod` + `github-actions`).

### Changed

- Bumped `goreleaser-action` from v6 to v7.

## [0.2.2] - 2026-08-22

### Fixed

- Clear failure message when git itself is missing from PATH (previously misreported as "not inside a git repository").
- README corrected: writ requires git and POSIX sh and ships for macOS/Linux only.

## [0.2.1] - 2026-08-22

### Security

- CI workflow hardened to least-privilege `permissions: contents: read`.

### Fixed

- Install docs cover the explicit trust step current Homebrew requires before using an untapped third-party formula.

## [0.2.0] - 2026-08-22

Public-release hardening of the core shipped in 0.1.x.

### Added

- `writ discard` to throw away an unwanted open writ (rejected proposal, abandoned work, corrupt state file) so `propose` is unblocked without hand-deleting state.
- `--version` / `-v` root flag.
- Strict TOML intake: author-written drafts (stdin, `--file`, `$EDITOR`) reject keys naming no writ field, so typos are reported instead of silently dropped.
- Shell-completion install instructions for bash/zsh/fish using an idiom that works on macOS's stock bash 3.2, plus documentation of color behavior when output is scripted.
- The exit-code contract (0 auto-mergeable, 1 needs human, 2 no writ open) documented across all six deciding commands.
- `WRIT_VERIFY_TIMEOUT` knob documented (10-minute default) and guarded against a zero-value footgun.

### Fixed

- `.writ/current.toml` state file can no longer be accidentally committed: seeded into the repo-local git exclude at propose time (working inside linked worktrees too), with a refusal and untrack guidance if a tracked copy appears at decide time.
- Corrupt state files now point every reading command at `writ discard` instead of surfacing raw errors.
- Commands outside a git repository fail loudly instead of silently treating the current directory as the repo.
- status/merge handle brand-new repos with zero commits (unborn HEAD).
- `approve` works when `EDITOR` contains arguments (e.g. `code -w`).
- Bare interactive `writ propose` refuses up front with actionable remedies instead of hanging silently.
- Dirty-tree merge guard no longer weakened by git's `status.showUntrackedFiles=no`.
- Vacuous verdicts closed: deleted criteria no longer count as "all met"; criteria with empty ids or text are rejected; whole-repo-scoped tampered writs are refused; unresolvable declared bases are refused; driving a writ from its own base branch is refused.
- Agents cannot self-bless acceptance claims: intake rejects criteria arriving pre-assessed (`met`/`attestation` set).
- README documents creating a feature branch off base during implement, matching writ's own refusals.

## [0.1.1] - 2026-08-21

### Fixed

- Generated Homebrew formula writes into the tap's `Formula/` directory so it cannot be shadowed by a root-level formula later.

## [0.1.0] - 2026-08-21

### Added

- Initial public release of `writ`, a git-native contract between you and your coding agent.
- Lifecycle commands: `propose`, `approve`, `attest`/`unattest <criterion-id>`, `status`, `merge`, `version`.
- Drift detection diffing committed work against the writ's declared scope.
- Evidence-based verification running each criterion's verify command and gating the merge decision on results.
- Human attestation kept visually distinct from machine claims and from agent attestations.
- Cross-platform releases (darwin/linux × amd64/arm64) with Homebrew tap publishing.

[Unreleased]: https://github.com/Laaaaksh/writ/compare/v0.2.4...HEAD
[0.2.4]: https://github.com/Laaaaksh/writ/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/Laaaaksh/writ/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/Laaaaksh/writ/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/Laaaaksh/writ/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Laaaaksh/writ/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/Laaaaksh/writ/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Laaaaksh/writ/releases/tag/v0.1.0
