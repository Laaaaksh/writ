package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/writ"
)

func newDiscardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discard",
		Short: "Discard the open writ",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiscard(cmd)
		},
	}
}

// runDiscard removes .writ/current.toml so the repo has no open writ and
// `writ propose` is unblocked. It is the way out of a rejected or abandoned
// proposal; it touches only writ's own state, never branches or commits.
func runDiscard(cmd *cobra.Command) error {
	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	w, err := writ.Load(repoDir)
	if errors.Is(err, writ.ErrNoWrit) {
		fmt.Fprintln(cmd.ErrOrStderr(), "no writ is open in this repo; run `writ propose` to start one")
		return exitCodeErr{code: 2}
	}

	path := filepath.Join(repoDir, ".writ", "current.toml")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	_ = os.Remove(filepath.Join(repoDir, ".writ"))

	if err != nil || w == nil {
		// The file existed but could not be read or parsed. Removing it is
		// exactly the recovery wanted here, so report success rather than
		// leave the user stuck with state no command accepts.
		fmt.Fprintln(cmd.OutOrStdout(), "discarded unreadable writ state (.writ/current.toml)")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "discarded writ %s: %s\n", w.ID, w.Intent)
	return nil
}
