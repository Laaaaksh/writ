package main

import (
	"errors"
	"fmt"

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

	w, dec, d, e, err := evaluate(repoDir)
	if errors.Is(err, writ.ErrNoWrit) {
		fmt.Fprintln(cmd.ErrOrStderr(), "no writ is open in this repo; run `writ propose` to start one")
		return exitCodeErr{code: 2}
	}
	if err != nil {
		return err
	}

	fmt.Fprint(cmd.OutOrStdout(), render.Status(w, d, e, dec))

	if dec.NeedsHuman {
		return exitCodeErr{code: 1}
	}
	return nil
}

// evaluate runs the full pipeline shared by `writ status` and `writ merge`:
// load the writ, compute drift, run verification, and decide.
func evaluate(repoDir string) (*writ.Writ, gate.Decision, *drift.Report, *evidence.Report, error) {
	w, err := writ.Load(repoDir)
	if err != nil {
		return nil, gate.Decision{}, nil, nil, err
	}

	d, err := drift.Compute(w, repoDir)
	if err != nil {
		return nil, gate.Decision{}, nil, nil, fmt.Errorf("computing drift: %w", err)
	}

	e, err := evidence.Run(w, repoDir)
	if err != nil {
		return nil, gate.Decision{}, nil, nil, fmt.Errorf("running verification: %w", err)
	}

	dec := gate.Decide(w, d, e)
	return w, dec, d, e, nil
}

// exitCodeErr carries a specific process exit code through cobra's error
// path without printing anything extra; main() special-cases it.
type exitCodeErr struct{ code int }

func (e exitCodeErr) Error() string { return "" }
