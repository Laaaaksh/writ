package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Laaaaksh/writ/internal/writ"
)

// proposeValidWrit runs `writ propose` with validWritTOML on stdin against
// the repo at dir, which must already be the current working directory.
func proposeValidWrit(t *testing.T) {
	t.Helper()
	cmd := newProposeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(validWritTOML))
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("propose: %v", err)
	}
}

func TestRunApproveYes(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	cmd := newApproveCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("approve --yes: %v (stderr: %s)", err, errOut.String())
	}

	w, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if w.Approved == nil {
		t.Fatal("expected writ to be approved")
	}
}

func TestRunApproveRefusesWhenAlreadyApproved(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	first := newApproveCmd()
	first.SetOut(&bytes.Buffer{})
	first.SetErr(&bytes.Buffer{})
	if err := first.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	if err := first.RunE(first, nil); err != nil {
		t.Fatalf("first approve: %v", err)
	}

	second := newApproveCmd()
	second.SetOut(&bytes.Buffer{})
	second.SetErr(&bytes.Buffer{})
	if err := second.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	if err := second.RunE(second, nil); err == nil {
		t.Fatal("second approve: expected an error since the writ is already approved")
	}
}

func TestRunApproveNoWritOpen(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newApproveCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	ec, ok := err.(exitCodeErr)
	if !ok || ec.code != 2 {
		t.Fatalf("approve with no open writ: got err %v, want exitCodeErr{2}", err)
	}
	if !strings.Contains(errOut.String(), "no writ is open") {
		t.Errorf("approve with no open writ: stderr = %q, want a helpful message", errOut.String())
	}
}

// editorScript writes an executable /bin/sh script at a temp path whose
// body rewrites the target file it receives as $1, sets $EDITOR to it, and
// returns the script path.
func editorScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor.sh")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", path)
	return path
}

// TestRunApproveViaEditor covers the documented human flow: `writ approve`
// opens the proposal in $EDITOR, and on save-and-exit the tightened writ is
// what gets approved.
func TestRunApproveViaEditor(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	editorScript(t, `cat > "$1" <<'TOML'
id = "w1"
intent = "do the thing"
base = "base"
created = 2026-01-01T00:00:00Z
scope = ["internal/foo/one.go", "internal/foo/two.go"]

[[criteria]]
id = "c1"
text = "the thing works"

[verify]
command = "go test ./..."
TOML`)

	cmd := newApproveCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("approve via editor: %v (stderr: %s)", err, errOut.String())
	}

	w, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if w.Approved == nil {
		t.Fatal("expected the edited writ to be approved")
	}
	want := []string{"internal/foo/one.go", "internal/foo/two.go"}
	if len(w.Scope) != len(want) || w.Scope[0] != want[0] || w.Scope[1] != want[1] {
		t.Errorf("approved scope = %v, want the editor's tightened scope %v", w.Scope, want)
	}
}

// TestRunApproveViaEditorInvalidEdit proves the README promise: when the
// edited writ fails validation, approve prints the problems and leaves the
// file in place, still unapproved.
func TestRunApproveViaEditorInvalidEdit(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	editorScript(t, `cat > "$1" <<'TOML'
id = "w1"
intent = ""
base = "base"
created = 2026-01-01T00:00:00Z
scope = ["internal/foo/**"]

[[criteria]]
id = "c1"
text = "the thing works"

[verify]
command = "go test ./..."
TOML`)

	cmd := newApproveCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, nil)
	if err == nil || !errors.Is(err, errSilent) {
		t.Fatalf("approve after invalid edit: got err %v, want errSilent", err)
	}
	for _, want := range []string{"invalid writ", "intent must not be empty"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("stderr = %q, want it to contain %q", errOut.String(), want)
		}
	}
	// git reports the symlink-resolved repo root, so resolve dir before
	// comparing paths.
	resolved, symErr := filepath.EvalSymlinks(dir)
	if symErr != nil {
		t.Fatal(symErr)
	}
	if !strings.Contains(errOut.String(), filepath.Join(resolved, ".writ", "current.toml")) {
		t.Errorf("stderr = %q, want it to name the file to fix", errOut.String())
	}

	w, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("Load after failed approve: %v", err)
	}
	if w.Approved != nil {
		t.Error("a failed approval must not stamp approval onto the writ")
	}
}

// TestRunApproveViaEditorBrokenTOML proves that even a syntactically broken
// save leaves the file in place rather than losing the proposal entirely.
func TestRunApproveViaEditorBrokenTOML(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	editorScript(t, `printf 'not toml at all' > "$1"`)

	cmd := newApproveCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, nil)
	if err == nil || errors.Is(err, errSilent) {
		t.Fatalf("approve after broken TOML: got err %v, want a plain error naming the file", err)
	}
	// git reports the symlink-resolved repo root, so resolve dir before
	// comparing paths.
	resolved, symErr := filepath.EvalSymlinks(dir)
	if symErr != nil {
		t.Fatal(symErr)
	}
	if !strings.Contains(err.Error(), filepath.Join(resolved, ".writ", "current.toml")) {
		t.Errorf("error = %v, want it to name the state file", err)
	}
	if _, loadErr := writ.Load(dir); loadErr == nil {
		t.Error("the broken file should still be on disk (unparseable), not deleted")
	}
}

// TestRunApproveRefusesPreAssessedCriteria locks in the intake rule on the
// approve side: a still-unapproved writ whose criteria already carry met or
// an attestation (smuggled in via the draft or a hand edit) is refused by
// approve --yes, stays unapproved on disk, and recovers through `writ
// unattest` followed by approval.
func TestRunApproveRefusesPreAssessedCriteria(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	w, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	met := true
	w.Criteria[0].Met = &met
	w.Criteria[0].Attestation = &writ.Attestation{By: "agent", Note: "self-blessed"}
	if err := w.Save(dir); err != nil {
		t.Fatalf("save tampered state: %v", err)
	}

	cmd := newApproveCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatal("approve --yes: expected refusal of pre-assessed criteria, got nil")
	}
	if !strings.Contains(errOut.String(), `criterion "c1" arrives already assessed`) {
		t.Errorf("refusal should name the offending criterion, got stderr: %s", errOut.String())
	}
	if strings.Contains(out.String(), "writ approved") {
		t.Errorf("refused writ must not be approved, output: %s", out.String())
	}

	still, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if still.Approved != nil {
		t.Error("refused writ must remain unapproved")
	}
	if still.Criteria[0].Attestation == nil {
		t.Error("state must be left in place for the user to clean up")
	}

	unattest := newUnattestCmd()
	unattest.SetOut(&bytes.Buffer{})
	unattest.SetErr(&bytes.Buffer{})
	if err := unattest.RunE(unattest, []string{"c1"}); err != nil {
		t.Fatalf("unattest c1: %v", err)
	}

	after := newApproveCmd()
	after.SetOut(&bytes.Buffer{})
	after.SetErr(&bytes.Buffer{})
	if err := after.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	if err := after.RunE(after, nil); err != nil {
		t.Fatalf("approve --yes after clearing the smuggled claim: %v", err)
	}

	final, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("final load: %v", err)
	}
	if final.Approved == nil {
		t.Error("writ should be approved after cleanup")
	}
	if final.Criteria[0].Met != nil || final.Criteria[0].Attestation != nil {
		t.Errorf("criterion c1 should carry no assessment, got %+v", final.Criteria[0])
	}
}
