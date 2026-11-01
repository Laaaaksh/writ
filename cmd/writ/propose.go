package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/writ"
)

func newProposeCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "propose",
		Short: "Propose an agent-drafted writ, read as TOML from stdin or --file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPropose(cmd, file)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "read the writ TOML from this file instead of stdin")
	return cmd
}

// runPropose reads a complete writ as TOML, validates it, and writes it to
// .writ/current.toml unapproved. This is the command an agent runs: it
// drafts the writ so a human only has to tighten and approve it, rather
// than author one from a blank file.
func runPropose(cmd *cobra.Command, file string) error {
	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	if _, err := writ.Load(repoDir); !errors.Is(err, writ.ErrNoWrit) {
		if err == nil {
			return fmt.Errorf("a writ is already open in this repo (.writ/current.toml); run `writ discard` to close it before proposing another")
		}
		return fmt.Errorf("%w; run `writ discard` to clear the broken state, then propose again", err)
	}

	var data []byte
	if file != "" {
		data, err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("reading %s: %w", file, err)
		}
	} else {
		if stdinIsInteractive(cmd.InOrStdin()) {
			return errors.New("propose reads the writ as TOML from stdin; pipe one in or pass --file <path>")
		}
		data, err = io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("reading writ from stdin: %w", err)
		}
	}

	// Parse, not a lenient decode: the draft is authored outside writ, so
	// a typo'd key must be named here rather than silently dropped and
	// resurfacing as an empty-field validation problem.
	w, err := writ.Parse(data)
	if err != nil {
		return fmt.Errorf("parsing writ: %w", err)
	}
	// A proposal is never pre-approved, regardless of what the input carried.
	w.Approved = nil

	if err := w.ValidateProposal(); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		return errSilent
	}

	if err := w.Save(repoDir); err != nil {
		return err
	}
	ensureWritExcluded(cmd, repoDir)

	fmt.Fprintf(cmd.OutOrStdout(), "writ proposed: %s\n", w.Intent)
	fmt.Fprintf(cmd.OutOrStdout(), "  scope:    %s\n", strings.Join(w.Scope, ", "))
	fmt.Fprintf(cmd.OutOrStdout(), "  criteria: %d\n", len(w.Criteria))
	fmt.Fprintln(cmd.OutOrStdout(), "run `writ approve` to review and approve it")
	return nil
}

// stdinIsInteractive reports whether r is a terminal, i.e. a human ran
// `writ propose` bare in a shell. Reading a writ from an interactive
// terminal blocks forever with no prompt and no hint, so propose refuses
// up front; piped input (the agent path) and file redirection are not
// character devices and read normally.
func stdinIsInteractive(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// ensureWritExcluded makes a best effort to keep writ's runtime state out
// of the user's commits. Without an ignore rule covering .writ/, a blanket
// `git add -A` during implement tracks .writ/current.toml, and a tracked
// state file breaks writ: later saves leave it locally modified, so merge's
// checkout of base dies on raw git output, while copies committed on the
// branch come back onto base as stale state after merging. Unless some
// ignore rule already covers the state file, it seeds .writ/ into the
// repo-local .git/info/exclude - local to this clone, never pushed or
// committed. Any failure is only a warning, never a propose failure:
// decide() still refuses genuinely tracked state at status/merge time.
//
// The rule goes into the COMMON git dir (--git-common-dir), not whatever
// --absolute-git-dir reports: git reads info/exclude only from the common
// dir, so seeding the per-worktree gitdir of a linked worktree would
// silently have no effect there.
func ensureWritExcluded(cmd *cobra.Command, repoDir string) {
	stateRel := writ.Dir + "/current.toml"

	if ignored, err := gitPathIgnored(repoDir, stateRel); err == nil && ignored {
		return
	}

	gitDir, err := runGitOutput(repoDir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		warnExcludeSeedFailed(cmd, err)
		return
	}

	excludeDir := filepath.Join(strings.TrimSpace(gitDir), "info")
	path := filepath.Join(excludeDir, "exclude")
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == writ.Dir+"/" {
				return // already seeded; nothing to add
			}
		}
	}

	if err := os.MkdirAll(excludeDir, 0o755); err != nil {
		warnExcludeSeedFailed(cmd, err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		warnExcludeSeedFailed(cmd, err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "\n# Added by writ so its runtime state stays out of your commits.\n%s/\n", writ.Dir)
}

// warnExcludeSeedFailed reports an exclude-seeding failure without failing
// the propose that triggered it.
func warnExcludeSeedFailed(cmd *cobra.Command, cause error) {
	fmt.Fprintf(cmd.ErrOrStderr(),
		"warning: could not keep .writ/ out of git commits (%v); avoid committing .writ/current.toml\n", cause)
}

// gitPathIgnored reports whether git would exclude p (repo-relative) from
// `git add` via any ignore mechanism (.gitignore, .git/info/exclude,
// core.excludesFile). check-ignore signals the verdict through its exit
// code: 0 for ignored, 1 for not ignored, anything else a real failure.
func gitPathIgnored(repoDir, p string) (bool, error) {
	cmd := exec.Command("git", "check-ignore", "-q", p)
	cmd.Dir = repoDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git check-ignore -q %s: %w: %s", p, err, strings.TrimSpace(out.String()))
}
