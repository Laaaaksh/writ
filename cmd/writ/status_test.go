package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunStatusNoWritOpen(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newStatusCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, nil)

	ec, ok := err.(exitCodeErr)
	if !ok {
		t.Fatalf("runStatus with no open writ: got err %v, want exitCodeErr", err)
	}
	if ec.code != 2 {
		t.Errorf("runStatus with no open writ: exit code = %d, want 2", ec.code)
	}
	if !strings.Contains(errOut.String(), "no writ is open") {
		t.Errorf("runStatus with no open writ: stderr = %q, want a helpful message", errOut.String())
	}
}

// TestRunStatusExitCodesThroughPipeline drives the real pipeline - drift
// compute, verification run, gate decision - through runStatus and proves
// both documented exit codes: 1 while a criterion is unattested, 0 once it
// is attested and nothing else blocks.
func TestRunStatusExitCodesThroughPipeline(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	proposeWritTOML(t, `
id = "w1"
intent = "add a feature"
base = "base"
created = 2026-01-01T00:00:00Z
scope = ["feature.txt"]

[[criteria]]
id = "c1"
text = "the feature works"

[verify]
command = "true"
`)
	approveYes(t)

	newCmd := func() *cobra.Command {
		cmd := newStatusCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		return cmd
	}

	if err := newCmd().RunE(newCmd(), nil); err == nil {
		t.Fatal("status before attesting: expected an error (exit 1), got nil")
	} else if ec, ok := err.(exitCodeErr); !ok || ec.code != 1 {
		t.Errorf("status before attesting: err = %v, want exitCodeErr{1}", err)
	}

	attestCriterion(t, "c1")

	if err := newCmd().RunE(newCmd(), nil); err != nil {
		t.Fatalf("status after attesting: %v, want nil (exit 0)", err)
	}
}

// A .writ/current.toml that exists but cannot be parsed must not strand a
// user behind an inscrutable TOML error in any command: every reader points
// at `writ discard`, which by design succeeds even on broken state.
func TestLoadOpenWritCorruptStateNamesDiscard(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	statePath := filepath.Join(dir, ".writ", "current.toml")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("not [ valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}

	newCmds := []struct {
		name string
		cmd  func() *cobra.Command
		args []string
	}{
		{"approve", newApproveCmd, nil},
		{"attest", func() *cobra.Command {
			c := newAttestCmd()
			if err := c.Flags().Set("note", "how"); err != nil {
				t.Fatal(err)
			}
			return c
		}, []string{"c1"}},
		{"unattest", newUnattestCmd, []string{"c1"}},
		{"status", newStatusCmd, nil},
		{"merge", newMergeCmd, nil},
	}

	for _, tt := range newCmds {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmd()
			var out, errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)

			err := cmd.RunE(cmd, tt.args)
			if err == nil {
				t.Fatal("expected an error when the open state file is corrupt")
			}
			if _, ok := err.(exitCodeErr); ok {
				t.Errorf("corrupt state is not 'no writ open': got exit-code error %v", err)
			}
			if !strings.Contains(err.Error(), "discard") {
				t.Errorf("error = %v, want it to point at `writ discard`", err)
			}
			if _, statErr := os.Stat(statePath); statErr != nil {
				t.Errorf("a refused command must leave the state file for discard to clear: %v", statErr)
			}
		})
	}
}
