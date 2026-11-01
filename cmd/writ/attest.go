package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/writ"
)

func newAttestCmd() *cobra.Command {
	var note string
	var human bool

	cmd := &cobra.Command{
		Use:   "attest <criterion-id>",
		Short: "Attest that a criterion has been met",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAttest(cmd, args[0], note, human)
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "one line: how the criterion was satisfied (required)")
	cmd.Flags().BoolVar(&human, "human", false, "attest as a human rather than the agent")
	return cmd
}

// runAttest marks a criterion met and records who claims so and how. It is a
// claim, not a fact: gate and render both keep that distinction visible.
func runAttest(cmd *cobra.Command, id, note string, human bool) error {
	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	w, err := loadOpenWrit(cmd, repoDir)
	if err != nil {
		return err
	}

	if w.Approved == nil {
		return errors.New("refusing to attest: writ is not yet approved; run `writ approve` first")
	}
	if strings.TrimSpace(note) == "" {
		return errors.New("refusing to attest: --note must not be empty")
	}

	idx, ids := findCriterion(w, id)
	if idx == -1 {
		return fmt.Errorf("unknown criterion %q; valid ids: %s", id, strings.Join(ids, ", "))
	}

	by := "agent"
	if human {
		by = "human"
	}

	met := true
	w.Criteria[idx].Met = &met
	w.Criteria[idx].Attestation = &writ.Attestation{
		By:   by,
		Note: note,
		At:   time.Now().UTC(),
	}

	if err := w.Save(repoDir); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "criterion %q attested by %s: %s\n", id, by, note)
	return nil
}

func newUnattestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unattest <criterion-id>",
		Short: "Clear a criterion's attestation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnattest(cmd, args[0])
		},
	}
}

func runUnattest(cmd *cobra.Command, id string) error {
	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	w, err := loadOpenWrit(cmd, repoDir)
	if err != nil {
		return err
	}

	idx, ids := findCriterion(w, id)
	if idx == -1 {
		return fmt.Errorf("unknown criterion %q; valid ids: %s", id, strings.Join(ids, ", "))
	}

	w.Criteria[idx].Met = nil
	w.Criteria[idx].Attestation = nil

	if err := w.Save(repoDir); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "criterion %q unattested\n", id)
	return nil
}

// findCriterion returns the index of the criterion with the given id in w,
// or -1 with the full list of valid ids if none matches.
func findCriterion(w *writ.Writ, id string) (int, []string) {
	idx := -1
	ids := make([]string, 0, len(w.Criteria))
	for i, c := range w.Criteria {
		ids = append(ids, c.ID)
		if c.ID == id {
			idx = i
		}
	}
	return idx, ids
}
