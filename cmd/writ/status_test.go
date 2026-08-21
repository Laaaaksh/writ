package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/writ"
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

// TestRunStatusUnapprovedWritNeedsHuman covers the README loop's state
// between steps 1 and 2: right after propose the writ exists but nobody has
// agreed to it, so status must exit 1 naming the missing approval - while
// still computing and rendering drift and evidence so the human reviewing
// the proposal sees what they are being asked to approve.
func TestRunStatusUnapprovedWritNeedsHuman(t *testing.T) {
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

	cmd := newStatusCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, nil)

	ec, ok := err.(exitCodeErr)
	if !ok || ec.code != 1 {
		t.Fatalf("status on an unapproved writ: err = %v, want exitCodeErr{1}", err)
	}
	got := out.String()
	if !strings.Contains(got, "writ has not been approved") {
		t.Errorf("status output = %q, want the verdict to name the missing approval", got)
	}
	if !strings.Contains(got, "not assessed") {
		t.Errorf("status output = %q, want unattested criteria shown as not assessed", got)
	}
	if strings.Contains(got, "Auto-mergeable") {
		t.Errorf("status output = %q must not claim auto-mergeable before approval", got)
	}

	w, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("reloading state after status: %v", err)
	}
	if w.Approved != nil {
		t.Error("status must not mutate approval state")
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

// TestDecideRefusesWholeRepoScope locks the scope axis of the vacuous-drift
// defense into CI: intake (propose/approve) already refuses a whole-repo
// scope, so one on disk means the approved state file was edited or broken
// afterwards - and drift against such a scope is vacuously zero, which would
// let an arbitrary unreviewed change set report as auto-mergeable. status
// must error loudly instead of rendering a green verdict, and merge must
// refuse even with --approve, the strongest override it has.
func TestDecideRefusesWholeRepoScope(t *testing.T) {
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
	attestCriterion(t, "c1")

	// Simulate post-approval tampering: swap the saved writ's scope line for
	// a whole-repo glob, leaving everything else (approval, attestation)
	// exactly as the tool wrote it.
	statePath := filepath.Join(dir, ".writ", "current.toml")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	tampered := false
	for i, line := range lines {
		if strings.HasPrefix(line, "scope =") {
			lines[i] = `scope = ["**"]`
			tampered = true
		}
	}
	if !tampered {
		t.Fatalf("no scope line found in saved state: %s", data)
	}
	if err := os.WriteFile(statePath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(cmd *cobra.Command, args []string) (*bytes.Buffer, *bytes.Buffer, error) {
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		return &out, &errOut, cmd.RunE(cmd, args)
	}

	statusCmd := newStatusCmd()
	out, _, err := run(statusCmd, nil)
	if err == nil {
		t.Fatal("status over a whole-repo-scope writ: expected an error")
	}
	if ec, ok := err.(exitCodeErr); ok && ec.code == 2 {
		t.Errorf("status error = %v, want a plain error (exit 1), not no-writ-open", err)
	}
	if !strings.Contains(err.Error(), "whole repo") || !strings.Contains(err.Error(), "discard") {
		t.Errorf("status error = %v, want it to name the whole-repo scope and point at discard", err)
	}
	if strings.Contains(out.String(), "Auto-mergeable") {
		t.Errorf("status output %q must not render an auto-mergeable verdict for a defeated scope", out.String())
	}

	mergeCmd := newMergeCmd()
	if err := mergeCmd.Flags().Set("approve", "true"); err != nil {
		t.Fatal(err)
	}
	outM, _, err := run(mergeCmd, nil)
	if err == nil {
		t.Fatal("merge --approve over a whole-repo-scope writ: expected a refusal")
	}
	if !strings.Contains(err.Error(), "whole repo") {
		t.Errorf("merge --approve error = %v, want it to name the whole-repo scope", err)
	}
	if strings.Contains(outM.String(), "merged into") {
		t.Errorf("merge --approve output %q must not report a merge", outM.String())
	}
	if branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current")); branch == "base" {
		t.Error("a refused merge must not check out base")
	}
	if _, statErr := os.Stat(statePath); statErr != nil {
		t.Errorf("a refused merge must leave the state file in place: %v", statErr)
	}
}
