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

// TestRunProposeRefusesPreAssessedDraft locks in the intake rule that a
// proposal cannot arrive with criteria already marked met or attested:
// claims are recorded via `writ attest` only after a human approves.
func TestRunProposeRefusesPreAssessedDraft(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newProposeCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(`
id = "w1"
intent = "do the thing"
base = "base"
created = 2026-01-01T00:00:00Z
scope = ["internal/foo/**"]

[[criteria]]
id = "c1"
text = "the thing works"
met = true

[criteria.attestation]
by = "agent"
note = "self-blessed"

[[criteria]]
id = "c2"
text = "tests pass"

[verify]
command = "go test ./..."
`))

	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatal("runPropose: expected refusal of a pre-assessed draft, got nil")
	}
	if !strings.Contains(errOut.String(), `criterion "c1" arrives already assessed`) {
		t.Errorf("refusal should name the offending criterion, got stderr: %s", errOut.String())
	}
	if strings.Contains(errOut.String(), `criterion "c2"`) {
		t.Errorf("clean criterion c2 must not be flagged, got stderr: %s", errOut.String())
	}

	if _, err := os.Stat(filepath.Join(dir, ".writ", "current.toml")); !os.IsNotExist(err) {
		t.Errorf("refused proposal must not create state, stat err %v", err)
	}
}

// A typo'd key such as "titel" must be named outright: the old lenient
// decode silently dropped it, so the draft failed validation later with the
// misleading "id must not be empty".
func TestRunProposeNamesUnknownKeys(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	draft := strings.Replace(validWritTOML, `id = "w1"`, `titel = "w1"`, 1)

	cmd := newProposeCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(draft))

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected propose to refuse a draft with a typo'd key")
	}
	for _, want := range []string{"parsing writ", `"titel"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to contain %s", err, want)
		}
	}
	if strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("error = %v; the typo must be named, not surfaced as an empty field", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".writ", "current.toml")); !os.IsNotExist(statErr) {
		t.Errorf("refused proposal must not create state (stat error: %v)", statErr)
	}
}

// TestRunProposeSeedsLocalExclude proves propose proactively protects the
// documented implement step: in a repo with no ignore rule covering .writ/,
// propose seeds the repo-local .git/info/exclude so a wholesale `git add -A`
// can never track .writ/current.toml - the exact tracked-state breakage the
// decide() guard would otherwise have to refuse at status/merge time.
// Seeding must also be idempotent across re-proposals.
func TestRunProposeSeedsLocalExclude(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	proposeWritTOML(t, validWritTOML)

	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("propose did not seed .git/info/exclude: %v", err)
	}
	if n := strings.Count(string(data), ".writ/"); n != 1 {
		t.Errorf("seeded exclude mentions .writ/ %d times, want exactly 1:\n%s", n, data)
	}

	// git itself must agree the state file is now excluded, so a blanket
	// add during implement cannot track writ's state behind the user's back.
	if out := mustRunGit(t, dir, "check-ignore", ".writ/current.toml"); strings.TrimSpace(out) == "" {
		t.Error("check-ignore reports .writ/current.toml not excluded after seeding")
	}
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-q", "-m", "implement")
	if out := mustRunGit(t, dir, "ls-files", ".writ"); strings.TrimSpace(out) != "" {
		t.Errorf("blanket `git add -A` tracked writ state after seeding: %q", out)
	}

	// Re-proposing after a discard must not duplicate the seeded line.
	discardCmd := newDiscardCmd()
	discardCmd.SetOut(&bytes.Buffer{})
	discardCmd.SetErr(&bytes.Buffer{})
	if err := discardCmd.RunE(discardCmd, nil); err != nil {
		t.Fatalf("discard: %v", err)
	}
	proposeWritTOML(t, validWritTOML)

	data, err = os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), ".writ/"); n != 1 {
		t.Errorf("re-propose duplicated the seeded line (%d occurrences of .writ/):\n%s", n, data)
	}
}

// TestRunProposeSkipsExcludeSeedWhenAlreadyIgnored proves seeding stays out
// of the way when the user already ignores .writ/ through their own
// committed .gitignore - no redundant lines land in .git/info/exclude.
func TestRunProposeSkipsExcludeSeedWhenAlreadyIgnored(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	gitignore := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte(".writ/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	proposeWritTOML(t, validWritTOML)

	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	if data, err := os.ReadFile(excludePath); err == nil && strings.Contains(string(data), ".writ/") {
		t.Errorf("propose seeded info/exclude although .gitignore already covers .writ/:\n%s", data)
	}
}
