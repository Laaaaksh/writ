package main

import (
	"bytes"
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
