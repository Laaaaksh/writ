# writ

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
refuses to accept one.

## Install

```
brew install Laaaaksh/writ/writ
```

The binary has no runtime dependencies. Alternatively, with a Go toolchain:

```
go install github.com/Laaaaksh/writ/cmd/writ@latest
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
to `.writ/current.toml`, unapproved. Drafting a concrete proposal - not hand-authoring path
globs from a blank file - is the agent's job, because a lazy scope like `app/**` makes drift
meaningless.

**2. `writ approve`** - a human reviews the proposal, opening it in `$EDITOR` to tighten scope
or criteria before agreeing to them. On save and exit, writ re-validates; if invalid, it
prints the problems and leaves the file in place. `--yes` approves the proposal as-is,
skipping the editor.

**3. Implement** - the agent does the work described by the writ.

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

When a proposal is rejected or abandoned, `writ discard` removes the open writ so `propose`
can start fresh; it touches only `.writ/current.toml`, never branches or commits.

```
$ writ status
```

```
$ writ version
```

Prints the writ CLI version.
