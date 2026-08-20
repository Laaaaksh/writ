# writ

Today an agent writes code, opens a PR, and a human reads a large diff to decide whether to
merge. That does not scale: the diff is too big to read and arrives stripped of the context
that produced it.

writ replaces that. A writ is the agreed scope of a piece of work, approved before any code
exists: an intent, a set of checkable acceptance criteria, a declared file scope, and a
verification command. The agent implements against it. writ then computes what actually
changed, compares it against the declared scope, runs the verification, and shows the human
only what fell outside the writ - the drift. Zero drift, plus met criteria, plus a green
verification, merges with no human at all.

A writ that declares its scope as the whole repository makes drift meaningless, so writ
refuses to open one.

## Install

```
go install github.com/Laaaaksh/writ/cmd/writ@latest
```

## Usage

```
$ writ open "add a retry to the webhook sender"
```

This creates `.writ/current.toml` in the repo, pre-filled with the intent and the repo's
current default branch, and opens it in `$EDITOR` so you can fill in scope, criteria, and a
verification command. On save and exit, writ validates the file; if it's invalid, writ prints
the problems and leaves the file in place so you can fix it.

```
$ writ version
```

Prints the writ CLI version.

More commands - computing drift, running verification, and rendering a merge decision - land
in later slices.
