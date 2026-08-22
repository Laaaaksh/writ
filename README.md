# writ

[![CI](https://github.com/Laaaaksh/writ/actions/workflows/ci.yml/badge.svg)](https://github.com/Laaaaksh/writ/actions/workflows/ci.yml)

Today an agent writes code, opens a PR, and a human reads a large diff to decide whether to
merge. That does not scale: the diff is too big to read and arrives stripped of the context
that produced it.

writ replaces that. A writ is the agreed scope of a piece of work, approved before any code
exists: an intent, a set of checkable acceptance criteria, a declared file scope, and a
verification command. The agent implements against it. writ then computes what actually
changed, compares it against the declared scope, runs the verification, and shows the human
only what fell outside the writ - the drift. Zero drift, plus every criterion met, plus a
green verification, merges with no human at all.

A writ that declares its scope as the whole repository makes drift meaningless, so writ
refuses to accept one - at intake and again before any status or merge decision.

## Install

```
brew install Laaaaksh/writ/writ
```

The binary has no runtime dependencies. Alternatively, with a Go toolchain:

```
go install github.com/Laaaaksh/writ/cmd/writ@latest
```

## Shell completions

writ generates tab-completion scripts for bash, zsh, fish, and PowerShell;
`writ completion <shell> --help` prints full per-shell instructions.

```
# zsh - current session; append the line to your .zshrc to keep it
source <(writ completion zsh)

# bash - current session; append the line to your .bashrc to keep it.
# eval is used because macOS's default bash (3.2) cannot source from
# <(...); results require the bash-completion package.
eval "$(writ completion bash)"

# fish
writ completion fish > ~/.config/fish/completions/writ.fish
```

## The loop: propose -> approve -> implement -> attest -> merge

**1. `writ propose`** - the agent drafts a complete writ and proposes it, before writing any
code:

```
$ writ propose <<'EOF'
id = "writ-1"
intent = "add a retry to the webhook sender"
base = "main"
created = 2026-01-01T00:00:00Z
scope = ["internal/webhook/**"]

[[criteria]]
id = "retries-on-5xx"
text = "a 5xx response is retried with backoff"

[verify]
command = "go test ./..."
EOF
```

`--file <path>` reads the writ from a file instead of stdin. writ validates it and writes it
to `.writ/current.toml`, unapproved. Criteria must arrive unassessed: a draft carrying `met`
or an `attestation` is refused, because claims are recorded with `writ attest` only after a
human approves. Drafting a concrete proposal - not hand-authoring path globs from a blank
file - is the agent's job, because a lazy scope like `app/**` makes drift meaningless.

propose also keeps writ's bookkeeping out of your history: when no ignore rule covers
`.writ/`, it seeds the repo-local `.git/info/exclude`, so committing your work wholesale
(`git add -A`) can never track the state file - and if it does end up tracked anyway,
`status` and `merge` refuse with instructions to untrack it.

**2. `writ approve`** - a human reviews the proposal, opening it in `$EDITOR` to tighten scope
or criteria before agreeing to them. On save and exit, writ re-validates; if invalid, it
prints the problems and leaves the file in place. `--yes` approves the proposal as-is,
skipping the editor.

**3. Implement** - the agent creates a branch off `base` (`git checkout -b <branch>`) and
does the work described by the writ there, committing as it goes: `merge` refuses a dirty
working tree, and commits made on `base` itself are invisible to drift.

**4. `writ attest <criterion-id> --note "<how>"`** - the agent (or a human, with `--human`)
claims that a criterion is met and records how. This is a claim, not a fact: `writ status`
renders it as "claimed by agent", visibly distinct from criteria a human confirmed or
evidence that ran and passed. Attesting requires an approved writ - claiming against a
contract nobody agreed to is meaningless. `writ unattest <criterion-id>` clears it.

**5. `writ status` / `writ merge`** - status computes drift, runs the verification command,
and renders the decision: zero drift, a green verification, an approved writ, and every
criterion attested auto-merges with no human; anything else names the reasons a human is
needed. `merge` does the same and, if mergeable (or given `--approve`), merges the writ's
branch into `base` and clears `.writ/current.toml`.

Run both from the writ's own feature branch, not from `base`: writ refuses them while HEAD
is on the base branch itself, where nothing can be merged and commits on base would be
invisible to drift.

The verification command runs through `sh -c` in the repo root; its exit code alone decides
pass or fail. It is killed after 10 minutes by default - set `WRIT_VERIFY_TIMEOUT` (a duration
like `45s` or `30m`) to change that; invalid or non-positive values fall back to the default.

When a proposal is rejected or abandoned, `writ discard` removes the open writ so `propose`
can start fresh; it touches only `.writ/current.toml`, never branches or commits.

```
$ writ status
```

```
$ writ version
```

Prints the writ CLI version; `writ --version` (or `-v`) prints the same line.

## Exit codes

For scripting and agents, `writ` signals its decision on the process exit code:

- `0` - success; for `status`, the writ is auto-mergeable, and for `merge`, it merged
- `1` - a human is needed (always from `status`; from `merge` unless `--approve` was given), or any other error occurred
- `2` - no writ is open (from any command that reads one: `approve`, `attest`, `unattest`, `status`, `merge`, `discard`)

Output stays script-friendly: ANSI color appears only when stdout is a terminal,
and setting `NO_COLOR` (to any non-empty value) turns it off even then, so piped
or captured output is always plain text.
