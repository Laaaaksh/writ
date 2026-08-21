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

	w, err := loadOpenWrit(cmd, repoDir)
	if err != nil {
		return err
	}

	dec, d, e, err := decide(repoDir, w)
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

// isDirty reports whether the working tree holds changes a merge must not
// disturb. Writ's own runtime state under writ.Dir never counts:
// propose/approve/attest leave .writ/current.toml untracked or modified by
// design, and that bookkeeping would otherwise block every merge on the
// documented happy path unless the user happens to gitignore it.
//
// --untracked-files=all is passed explicitly so the verdict depends only on
// repo state, not the user's `status.showUntrackedFiles` display preference,
// which would otherwise silently hide untracked files from the check (the
// same explicit flag drift.Compute already uses).
func isDirty(repoDir string) (bool, error) {
	out, err := runGitOutput(repoDir, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return false, err
	}
	for _, tok := range strings.Split(out, "\x00") {
		if tok == "" || isWritStateEntry(tok) {
			continue
		}
		return true, nil
	}
	return false, nil
}

// isWritStateEntry reports whether one porcelain -z token refers to a path
// under writ's own state directory. Entry tokens are "XY PATH"; for renames
// the old path follows as a separate bare token, which this conservatively
// counts as user work.
func isWritStateEntry(tok string) bool {
	if len(tok) < 4 || tok[2] != ' ' {
		return false
	}
	path := tok[3:]
	return path == writ.Dir || strings.HasPrefix(path, writ.Dir+"/")
}

// writStateTracked reports whether git knows about writ's own state file,
// .writ/current.toml - committed or merely staged, since ls-files reads the
// index and a staged copy poisons checkout exactly like a committed one. A
// tracked state file breaks the merge writ performs: later saves leave it
// locally modified, so checking out base dies on git's raw "local changes
// would be overwritten" output, while a copy committed on the branch merges
// back onto base as stale state and leaves a phantom deletion dirtying the
// tree after every merge.
func writStateTracked(repoDir string) (bool, error) {
	out, err := runGitOutput(repoDir, "ls-files", "-z", "--", writ.Dir+"/current.toml")
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
