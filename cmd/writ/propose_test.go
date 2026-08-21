package main

import (
	"bytes"
	"io"
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

// A corrupt state file must not strand the agent behind an inscrutable TOML
// error: propose should name `writ discard`, the command that exists exactly
// to clear broken state.
func TestRunProposeCorruptStateNamesDiscard(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	stateDir := filepath.Join(dir, ".writ")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "current.toml"), []byte("not [ valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newProposeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(validWritTOML))

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected an error when the open state file is corrupt")
	}
	if !strings.Contains(err.Error(), "discard") {
		t.Errorf("error = %v, want it to point at `writ discard`", err)
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

// A syntactically broken proposal must fail with a parse error naming the
// problem, and must not write any state file.
func TestRunProposeMalformedTOML(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newProposeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader("id = [ not toml"))

	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected an error for malformed TOML input")
	} else if !strings.Contains(err.Error(), "parsing writ") {
		t.Errorf("error = %v, want it to name the parse failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".writ", "current.toml")); !os.IsNotExist(statErr) {
		t.Errorf("malformed TOML must not create state (stat error: %v)", statErr)
	}
}

// A --file path that cannot be read must fail with an error naming the path.
func TestRunProposeFileReadError(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	missing := filepath.Join(t.TempDir(), "no-such-writ.toml")

	cmd := newProposeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Flags().Set("file", missing); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected an error for a missing --file path")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error = %v, want it to name the unreadable path %s", err, missing)
	}
}

// A bare `writ propose` in a real terminal would block forever on stdin
// with no prompt and no hint. The guard must refuse that case up front
// while leaving piped and redirected input untouched.
func TestRunProposeRefusesInteractiveStdin(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("cannot open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

	cmd := newProposeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(devNull)

	err = cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected propose to refuse an interactive terminal")
	}
	if !strings.Contains(err.Error(), "stdin") || !strings.Contains(err.Error(), "--file") {
		t.Errorf("error = %v, want it to name stdin and --file", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".writ", "current.toml")); !os.IsNotExist(statErr) {
		t.Errorf(".writ/current.toml should not exist after the refusal (stat error: %v)", statErr)
	}
}

func TestStdinIsInteractive(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("cannot open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()
	fi, err := devNull.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		t.Skipf("%s is not a character device here; cannot use it as a terminal stand-in", os.DevNull)
	}

	file, err := os.CreateTemp(t.TempDir(), "regular")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	tests := []struct {
		name string
		in   io.Reader
		want bool
	}{
		{"character device acts as a terminal", devNull, true},
		{"pipe-like reader does not", strings.NewReader(validWritTOML), false},
		{"redirected regular file does not", file, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stdinIsInteractive(tc.in); got != tc.want {
				t.Errorf("stdinIsInteractive(%T) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
