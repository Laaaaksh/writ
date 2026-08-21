package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validWritTOML = `
id = "w1"
intent = "do the thing"
base = "base"
created = 2026-01-01T00:00:00Z
scope = ["internal/foo/**"]

[[criteria]]
id = "c1"
text = "the thing works"

[verify]
command = "go test ./..."
`

func TestRunProposeFromStdin(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newProposeCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(validWritTOML))

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runPropose: %v (stderr: %s)", err, errOut.String())
	}
	if !strings.Contains(out.String(), "writ proposed") {
		t.Errorf("propose output = %q, want mention of 'writ proposed'", out.String())
	}

	path := filepath.Join(dir, ".writ", "current.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if strings.Contains(string(data), "[approved]") {
		t.Errorf("proposed writ must not be approved:\n%s", data)
	}
}

func TestRunProposeRefusesWhenAlreadyOpen(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	first := newProposeCmd()
	first.SetOut(&bytes.Buffer{})
	first.SetErr(&bytes.Buffer{})
	first.SetIn(strings.NewReader(validWritTOML))
	if err := first.RunE(first, nil); err != nil {
		t.Fatalf("first propose: %v", err)
	}

	second := newProposeCmd()
	second.SetOut(&bytes.Buffer{})
	second.SetErr(&bytes.Buffer{})
	second.SetIn(strings.NewReader(validWritTOML))
	if err := second.RunE(second, nil); err == nil {
		t.Fatal("second propose: expected an error since a writ is already open")
	}
}

func TestRunProposeInvalidWrit(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newProposeCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader("id = \"\"\n"))

	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected an error for an invalid writ")
	}
	if !strings.Contains(errOut.String(), "invalid writ") {
		t.Errorf("stderr = %q, want validation errors", errOut.String())
	}
}
