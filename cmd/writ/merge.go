package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/gate"
	"github.com/Laaaaksh/writ/internal/render"
	"github.com/Laaaaksh/writ/internal/writ"
)

func newMergeCmd() *cobra.Command {
	var approve bool

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge the open writ's branch if it is auto-mergeable",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMerge(cmd, approve)
		},
	}
	cmd.Flags().BoolVar(&approve, "approve", false, "record human approval and merge even though the writ needs review")
	return cmd
}

// runMerge loads the open writ, decides mergeability the same way `writ
// status` does, and either merges w's branch into w.Base or refuses. A
// human-required decision is only overridden by --approve.
func runMerge(cmd *cobra.Command, approve bool) error {
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

	if err := requireApproval(dec, approve); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		return exitCodeErr{code: 1}
	}
	if dec.NeedsHuman && approve {
		fmt.Fprintln(cmd.OutOrStdout(), "human approval recorded via --approve; proceeding with merge")
	}

	if err := gitMerge(repoDir, w.Base); err != nil {
		return err
	}

	writPath := filepath.Join(repoDir, ".writ", "current.toml")
	if err := os.Remove(writPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", writPath, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "merged into %s\n", w.Base)
	return nil
}

// requireApproval returns an error when dec needs a human and approve was
// not passed. It is pure so the approval gate is testable without git.
func requireApproval(dec gate.Decision, approve bool) error {
	if !dec.NeedsHuman || approve {
		return nil
	}
	return errors.New("refusing to merge: a human must review this writ (pass --approve once you have)")
}

// gitMerge merges the current branch into base, in repoDir, using plain git.
// It refuses - never forcing, never discarding uncommitted work - if the
// working tree is dirty or the merge would not be a clean fast-forward or
// merge commit.
func gitMerge(repoDir, base string) error {
	dirty, err := isDirty(repoDir)
	if err != nil {
		return err
	}
	if dirty {
		return errors.New("refusing to merge: working tree is dirty; commit or stash your changes first")
	}

	current, err := currentBranch(repoDir)
	if err != nil {
		return err
	}
	if current == "" {
		return errors.New("refusing to merge: HEAD is detached; check out the branch you want to merge")
	}
	if current == base {
		return fmt.Errorf("refusing to merge: already on base branch %q", base)
	}

	if err := runGit(repoDir, "checkout", base); err != nil {
		return fmt.Errorf("checking out base branch %q: %w", base, err)
	}

	if err := runGit(repoDir, "merge", "--no-edit", current); err != nil {
		_ = runGit(repoDir, "merge", "--abort")
		_ = runGit(repoDir, "checkout", current)
		return fmt.Errorf("refusing to merge: %q into %q was not a clean fast-forward or merge: %w", current, base, err)
	}

	return nil
}

func isDirty(repoDir string) (bool, error) {
	out, err := runGitOutput(repoDir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func currentBranch(repoDir string) (string, error) {
	out, err := runGitOutput(repoDir, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func runGit(repoDir string, args ...string) error {
	_, err := runGitOutput(repoDir, args...)
	return err
}

func runGitOutput(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return out.String(), nil
}
