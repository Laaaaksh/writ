package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRootCmdRegistersDocumentedCommands locks the public CLI surface: every
// command and flag the README documents must be registered on the root, so a
// refactor that drops or renames one fails CI rather than shipping.
func TestRootCmdRegistersDocumentedCommands(t *testing.T) {
	root := newRootCmd()

	registered := map[string]bool{}
	for _, c := range root.Commands() {
		registered[c.Name()] = true
	}
	for _, name := range []string{"version", "propose", "approve", "attest", "unattest", "status", "merge", "discard"} {
		if !registered[name] {
			t.Errorf("root command is missing documented subcommand %q", name)
		}
	}

	wantFlags := map[string][]string{
		"propose": {"file"},
		"approve": {"yes"},
		"attest":  {"note", "human"},
		"merge":   {"approve"},
	}
	for name, flags := range wantFlags {
		sub, _, err := root.Find([]string{name})
		if err != nil || sub.Name() != name {
			t.Fatalf("root.Find(%q): sub=%v err=%v", name, sub, err)
		}
		for _, flag := range flags {
			if sub.Flags().Lookup(flag) == nil {
				t.Errorf("writ %s is missing documented flag --%s", name, flag)
			}
		}
	}
}

// TestRootExecuteVersion drives `writ version` through the real cobra
// dispatch path (SetArgs -> Execute), not just RunE.
func TestRootExecuteVersion(t *testing.T) {
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("writ version: unexpected error %v", err)
	}
	if got := out.String(); !strings.Contains(got, "writ version") {
		t.Errorf("writ version output = %q, want it to name the version", got)
	}
}

// TestRootExecuteBareInvocationShowsHelp proves bare `writ` prints help and
// succeeds - a first-run user's most likely keystroke.
func TestRootExecuteBareInvocationShowsHelp(t *testing.T) {
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{})

	if err := root.Execute(); err != nil {
		t.Fatalf("bare writ: unexpected error %v", err)
	}
	if combined := out.String() + errOut.String(); !strings.Contains(combined, "Usage:") {
		t.Errorf("bare writ printed no usage/help, got stdout %q stderr %q", out.String(), errOut.String())
	}
}

// TestRootExecuteUnknownCommandErrors proves an unknown subcommand fails
// loudly (main() turns the returned error into exit 1 with the message).
func TestRootExecuteUnknownCommandErrors(t *testing.T) {
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"definitely-not-a-command"})

	err := root.Execute()
	if err == nil {
		t.Fatal("unknown command: expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "definitely-not-a-command") {
		t.Errorf("unknown command error = %v, want it to name the problem", err)
	}
}

// TestRootExecuteNoWritStatusExits2 proves cobra's Execute propagates
// exitCodeErr intact to main(): `writ status` with no writ open must yield
// exactly exit code 2 through the real dispatch path.
func TestRootExecuteNoWritStatusExits2(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"status"})

	err := root.Execute()
	ec, ok := err.(exitCodeErr)
	if !ok {
		t.Fatalf("writ status with no writ open: got err %v, want exitCodeErr", err)
	}
	if ec.code != 2 {
		t.Errorf("writ status with no writ open: exit code = %d, want 2", ec.code)
	}
}

// TestRootExecuteDocumentedLifecycle drives propose --file -> approve --yes
// -> attest <id> --note -> status -> merge through the root command with
// real argument and flag parsing, ending merged onto base with state cleared.
func TestRootExecuteDocumentedLifecycle(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	// The draft lives outside the repo so its bytes never count as drift.
	draftDir := t.TempDir()
	draft := filepath.Join(draftDir, "writ.toml")
	if err := os.WriteFile(draft, []byte(`
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
`), 0o644); err != nil {
		t.Fatal(err)
	}

	root := func(args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
		r := newRootCmd()
		var out, errOut bytes.Buffer
		r.SetOut(&out)
		r.SetErr(&errOut)
		r.SetArgs(args)
		return &out, &errOut, r.Execute()
	}

	if _, errOut, err := root("propose", "--file", draft); err != nil {
		t.Fatalf("propose --file: %v (stderr: %s)", err, errOut.String())
	}
	if _, errOut, err := root("approve", "--yes"); err != nil {
		t.Fatalf("approve --yes: %v (stderr: %s)", err, errOut.String())
	}
	out, errOut, err := root("attest", "c1", "--note", "covered by tests")
	if err != nil {
		t.Fatalf("attest c1 --note: %v (stderr: %s)", err, errOut.String())
	}
	if !strings.Contains(out.String(), `"c1" attested`) {
		t.Errorf("attest output = %q, want it to confirm criterion c1", out.String())
	}

	out, errOut, err = root("status")
	if err != nil {
		t.Fatalf("status after full attestation should be auto-mergeable (exit 0): %v\n%s%s", err, out.String(), errOut.String())
	}
	if !strings.Contains(out.String()+errOut.String(), "Auto-mergeable") {
		t.Errorf("status output = %q, want it to report Auto-mergeable", out.String()+errOut.String())
	}

	out, errOut, err = root("merge")
	if err != nil {
		t.Fatalf("merge: %v\n%s%s", err, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "merged into base") {
		t.Errorf("merge output = %q, want it to report merging into base", out.String())
	}

	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "base" {
		t.Errorf("expected to be on base after merge, got %q", branch)
	}
	if _, err := os.Stat(filepath.Join(dir, ".writ", "current.toml")); !os.IsNotExist(err) {
		t.Errorf("expected .writ/current.toml to be cleared after merge, got stat err %v", err)
	}
}
