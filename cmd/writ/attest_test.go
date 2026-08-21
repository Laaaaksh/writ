package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Laaaaksh/writ/internal/writ"
)

// setupApprovedWrit proposes and approves validWritTOML in a fresh test
// repo, chdirs into it, and returns its path.
func setupApprovedWrit(t *testing.T) string {
	t.Helper()
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	cmd := newApproveCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("approve: %v", err)
	}
	return dir
}

func TestRunAttestUnknownCriterion(t *testing.T) {
	setupApprovedWrit(t)

	cmd := newAttestCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Flags().Set("note", "done"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"nope"})
	if err == nil {
		t.Fatal("expected an error for an unknown criterion id")
	}
	if !strings.Contains(err.Error(), "unknown criterion") || !strings.Contains(err.Error(), "c1") {
		t.Errorf("error = %v, want it to name the unknown id and list valid ones", err)
	}
}

func TestRunAttestEmptyNote(t *testing.T) {
	setupApprovedWrit(t)

	cmd := newAttestCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.RunE(cmd, []string{"c1"}); err == nil {
		t.Fatal("expected an error for an empty note")
	}
}

func TestRunAttestUnapprovedWrit(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	cmd := newAttestCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Flags().Set("note", "done"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"c1"})
	if err == nil {
		t.Fatal("expected an error since the writ is not yet approved")
	}
	if !strings.Contains(err.Error(), "not yet approved") {
		t.Errorf("error = %v, want it to mention approval", err)
	}
}

func TestRunAttestAndUnattest(t *testing.T) {
	dir := setupApprovedWrit(t)

	acmd := newAttestCmd()
	acmd.SetOut(&bytes.Buffer{})
	acmd.SetErr(&bytes.Buffer{})
	if err := acmd.Flags().Set("note", "covered by tests"); err != nil {
		t.Fatal(err)
	}
	if err := acmd.RunE(acmd, []string{"c1"}); err != nil {
		t.Fatalf("attest: %v", err)
	}

	w, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if w.Criteria[0].Met == nil || !*w.Criteria[0].Met {
		t.Fatal("expected c1.Met to be true after attest")
	}
	if w.Criteria[0].Attestation == nil || w.Criteria[0].Attestation.By != "agent" {
		t.Fatalf("expected agent attestation, got %+v", w.Criteria[0].Attestation)
	}

	ucmd := newUnattestCmd()
	ucmd.SetOut(&bytes.Buffer{})
	ucmd.SetErr(&bytes.Buffer{})
	if err := ucmd.RunE(ucmd, []string{"c1"}); err != nil {
		t.Fatalf("unattest: %v", err)
	}

	w2, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if w2.Criteria[0].Met != nil || w2.Criteria[0].Attestation != nil {
		t.Fatalf("expected c1 cleared after unattest, got %+v", w2.Criteria[0])
	}
}

func TestRunAttestHumanFlag(t *testing.T) {
	dir := setupApprovedWrit(t)

	cmd := newAttestCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Flags().Set("note", "confirmed manually"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("human", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"c1"}); err != nil {
		t.Fatalf("attest --human: %v", err)
	}

	w, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if w.Criteria[0].Attestation == nil || w.Criteria[0].Attestation.By != "human" {
		t.Fatalf("expected human attestation, got %+v", w.Criteria[0].Attestation)
	}
}
