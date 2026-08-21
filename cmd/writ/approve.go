package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/writ"
)

func newApproveCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Tighten and approve the proposed writ",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApprove(cmd, yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "approve as-is, skipping the editor")
	return cmd
}

// runApprove opens the proposed writ in $EDITOR so a human can tighten scope
// and criteria before agreeing to them, then re-validates and stamps
// approval. --yes skips the editor and approves the proposal as-is.
func runApprove(cmd *cobra.Command, yes bool) error {
	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	w, err := writ.Load(repoDir)
	if errors.Is(err, writ.ErrNoWrit) {
		fmt.Fprintln(cmd.ErrOrStderr(), "no writ is open in this repo; run `writ propose` to start one")
		return exitCodeErr{code: 2}
	}
	if err != nil {
		return err
	}
	if w.Approved != nil {
		return fmt.Errorf("writ is already approved (at %s)", w.Approved.At.Format(time.RFC3339))
	}

	path := filepath.Join(repoDir, ".writ", "current.toml")

	if !yes {
		if err := openInEditor(path); err != nil {
			return err
		}

		w, err = writ.Load(repoDir)
		if err != nil {
			return fmt.Errorf("reloading %s: %w", path, err)
		}
		if w.Approved != nil {
			return fmt.Errorf("writ is already approved (at %s)", w.Approved.At.Format(time.RFC3339))
		}
	}

	if err := w.Validate(); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		fmt.Fprintf(cmd.ErrOrStderr(), "\nfix the errors above and edit %s directly, then try again\n", path)
		return errSilent
	}

	w.Approved = &writ.Approval{At: time.Now().UTC()}
	if err := w.Save(repoDir); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "writ approved: %s\n", path)
	return nil
}
