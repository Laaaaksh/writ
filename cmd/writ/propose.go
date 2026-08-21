package main

import (
	"errors"
	"fmt"
	"io"
	"os"
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
