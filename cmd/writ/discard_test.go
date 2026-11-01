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

func TestRunDiscardRemovesOpenWrit(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)
	proposeValidWrit(t)

	cmd := newDiscardCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("discard: %v (stderr: %s)", err, errOut.String())
	}
	if !strings.Contains(out.String(), "do the thing") {
		t.Errorf("discard output = %q, want it to name the discarded writ's intent", out.String())
	}

	if _, err := writ.Load(dir); !errors.Is(err, writ.ErrNoWrit) {
		t.Fatalf("after discard, Load = %v, want ErrNoWrit", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".writ")); !os.IsNotExist(err) {
		t.Errorf("expected the empty .writ/ directory removed after discard, got %v", err)
	}

	// Discarding exists to unblock the loop: a fresh proposal must be
	// accepted again without touching writ state by hand.
	proposeValidWrit(t)
}

func TestRunDiscardApprovedWrit(t *testing.T) {
	dir := setupApprovedWrit(t)

	cmd := newDiscardCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("discarding an approved writ: %v", err)
	}
	if _, err := writ.Load(dir); !errors.Is(err, writ.ErrNoWrit) {
		t.Fatalf("after discarding an approved writ, Load = %v, want ErrNoWrit", err)
	}
}

func TestRunDiscardCorruptStateFile(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	stateDir := filepath.Join(dir, ".writ")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, "current.toml")
	if err := os.WriteFile(path, []byte("not [ valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newDiscardCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("discarding corrupt state: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected corrupt current.toml removed after discard, got %v", err)
	}
}

func TestRunDiscardNoWritOpen(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newDiscardCmd()
	cmd.SetOut(&bytes.Buffer{})
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, nil)
	ec, ok := err.(exitCodeErr)
	if !ok {
		t.Fatalf("discard with no open writ: got err %v, want exitCodeErr", err)
	}
	if ec.code != 2 {
		t.Errorf("discard with no open writ: exit code = %d, want 2", ec.code)
	}
	if !strings.Contains(errOut.String(), "no writ is open") {
		t.Errorf("stderr = %q, want a helpful message", errOut.String())
	}
}
