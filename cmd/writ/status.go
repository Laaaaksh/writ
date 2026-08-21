package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/drift"
	"github.com/Laaaaksh/writ/internal/evidence"
	"github.com/Laaaaksh/writ/internal/gate"
	"github.com/Laaaaksh/writ/internal/render"
	"github.com/Laaaaksh/writ/internal/writ"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the drift-only review surface for the open writ",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd)
		},
	}
}

// runStatus loads the open writ, computes drift and evidence, decides
// mergeability, and prints the review surface. It exits 0 when
// auto-mergeable, 1 when a human is needed, 2 when no writ is open.
func runStatus(cmd *cobra.Command) error {
	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	w, err := loadOpenWrit(cmd, repoDir)
	if err != nil {
		return err
	}

	dec, d, e, err := decide(repoDir, w)
	if err != nil {
		return err
	}

	fmt.Fprint(cmd.OutOrStdout(), render.Status(w, d, e, dec))

	if dec.NeedsHuman {
		return exitCodeErr{code: 1}
	}
	return nil
}

// loadOpenWrit loads .writ/current.toml relative to repoDir on behalf of
// every command that needs the open writ, translating the two failure modes
// into their shared user-facing contract: nothing open prints the propose
// hint and exits 2, while a file that exists but cannot be read or parsed
// points at `writ discard`, which by design succeeds even on broken state -
// without that pointer a stranded user sees only an inscrutable TOML error.
func loadOpenWrit(cmd *cobra.Command, repoDir string) (*writ.Writ, error) {
	w, err := writ.Load(repoDir)
	if errors.Is(err, writ.ErrNoWrit) {
		fmt.Fprintln(cmd.ErrOrStderr(), "no writ is open in this repo; run `writ propose` to start one")
		return nil, exitCodeErr{code: 2}
	}
	if err != nil {
		return nil, fmt.Errorf("%w; run `writ discard` to clear the broken state", err)
	}
	return w, nil
}

// decide runs drift computation, verification, and the mergeability gate for
// an already-loaded writ - the pipeline shared by `writ status` and
// `writ merge`. It refuses a writ whose declared scope covers the whole
// repo: intake (propose/approve) already rejects one, so reaching this point
// means the state file was hand-edited or broken after approval, and drift
// against such a scope is vacuously zero - the writ would auto-merge with
// arbitrary unreviewed changes.
func decide(repoDir string, w *writ.Writ) (gate.Decision, *drift.Report, *evidence.Report, error) {
	var covered []string
	for _, s := range w.Scope {
		if writ.IsWholeRepoScope(s) {
			covered = append(covered, fmt.Sprintf("%q", s))
		}
	}
	if len(covered) > 0 {
		return gate.Decision{}, nil, nil, fmt.Errorf(
			"invalid writ: scope entry(s) %s cover the whole repo, which defeats drift detection; run `writ discard` and propose again",
			strings.Join(covered, ", "))
	}

	d, err := drift.Compute(w, repoDir)
	if err != nil {
		return gate.Decision{}, nil, nil, fmt.Errorf("computing drift: %w", err)
	}

	e, err := evidence.Run(w, repoDir)
	if err != nil {
		return gate.Decision{}, nil, nil, fmt.Errorf("running verification: %w", err)
	}

	return gate.Decide(w, d, e), d, e, nil
}

// exitCodeErr carries a specific process exit code through cobra's error
// path without printing anything extra; main() special-cases it.
type exitCodeErr struct{ code int }

func (e exitCodeErr) Error() string { return "" }
