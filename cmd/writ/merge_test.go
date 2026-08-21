package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/gate"
	"github.com/Laaaaksh/writ/internal/writ"
)

func mustRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out.String())
	}
	return out.String()
}

// newTestRepo creates a git repo at dir with a "base" branch holding one
// commit, checked out on a "feature" branch with a second commit ahead of
// it - a clean, mergeable setup for gitMerge to work with.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	mustRunGit(t, dir, "init", "-q", "-b", "base")
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test")
	// Pin the configs a contributor machine may set globally and that would
	// otherwise break or alter commits made inside these throwaway repos
	// (e.g. signing without a usable key).
	mustRunGit(t, dir, "config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, dir, "add", "base.txt")
	mustRunGit(t, dir, "commit", "-q", "-m", "base commit")

	mustRunGit(t, dir, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, dir, "add", "feature.txt")
	mustRunGit(t, dir, "commit", "-q", "-m", "feature commit")

	return dir
}

func TestRequireApproval(t *testing.T) {
	tests := []struct {
		name    string
		dec     gate.Decision
		approve bool
		wantErr bool
	}{
		{"mergeable, no approve needed", gate.Decision{Mergeable: true, NeedsHuman: false}, false, false},
		{"needs human without approve refuses", gate.Decision{Mergeable: false, NeedsHuman: true, Reasons: []string{"drift"}}, false, true},
		{"needs human with approve proceeds", gate.Decision{Mergeable: false, NeedsHuman: true, Reasons: []string{"drift"}}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requireApproval(tt.dec, tt.approve)
			if (err != nil) != tt.wantErr {
				t.Errorf("requireApproval(%+v, %v) = %v, wantErr %v", tt.dec, tt.approve, err, tt.wantErr)
			}
		})
	}
}

func TestGitMergeRefusesOnDirtyTree(t *testing.T) {
	dir := newTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := gitMerge(dir, "base")
	if err == nil {
		t.Fatal("gitMerge: expected an error for a dirty working tree, got nil")
	}
	if !strings.Contains(err.Error(), "dirty") {
		t.Errorf("gitMerge error = %v, want it to mention the dirty tree", err)
	}

	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "feature" {
		t.Errorf("gitMerge on a dirty tree must not switch branches, got %q", branch)
	}
	if _, err := os.Stat(filepath.Join(dir, "uncommitted.txt")); err != nil {
		t.Errorf("gitMerge on a dirty tree must not discard uncommitted work: %v", err)
	}
}

// TestGitMergeDirtyTreeIgnoresStatusConfig proves the dirty-tree verdict
// depends on repo state alone: even when the local git config hides
// untracked files from `git status` (status.showUntrackedFiles=no), an
// untracked file must still make isDirty report true.
func TestGitMergeDirtyTreeIgnoresStatusShowUntrackedFilesConfig(t *testing.T) {
	dir := newTestRepo(t)
	mustRunGit(t, dir, "config", "status.showUntrackedFiles", "no")

	if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty, err := isDirty(dir)
	if err != nil {
		t.Fatalf("isDirty: %v", err)
	}
	if !dirty {
		t.Error("isDirty = false for an untracked file with status.showUntrackedFiles=no, want true")
	}

	err = gitMerge(dir, "base")
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Errorf("gitMerge error = %v, want it to refuse on the untracked-file dirty tree", err)
	}
}

// TestGitMergeIgnoresWritState proves the documented happy path works: writ
// leaves .writ/current.toml behind as untracked or modified bookkeeping, and
// that alone must not make the tree dirty enough to refuse a merge.
func TestGitMergeIgnoresWritState(t *testing.T) {
	dir := newTestRepo(t)

	stateDir := filepath.Join(dir, ".writ")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "current.toml"), []byte("id = \"w1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty, err := isDirty(dir)
	if err != nil {
		t.Fatalf("isDirty: %v", err)
	}
	if dirty {
		t.Error("isDirty = true with only .writ/current.toml present, want false")
	}

	if err := gitMerge(dir, "base"); err != nil {
		t.Fatalf("gitMerge with only writ state in the tree: %v", err)
	}
	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "base" {
		t.Errorf("expected to end up on base, got %q", branch)
	}
}

func TestGitMergeSuccessCleanMerge(t *testing.T) {
	dir := newTestRepo(t)

	if err := gitMerge(dir, "base"); err != nil {
		t.Fatalf("gitMerge: %v", err)
	}

	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "base" {
		t.Fatalf("expected to end up on base, got %q", branch)
	}
	if _, err := os.Stat(filepath.Join(dir, "feature.txt")); err != nil {
		t.Errorf("expected feature.txt to be merged into base: %v", err)
	}
}

func TestGitMergeRefusesOnConflict(t *testing.T) {
	dir := newTestRepo(t)

	// Diverge base so merging feature in produces a conflict on base.txt.
	mustRunGit(t, dir, "checkout", "-q", "base")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base changed on base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, dir, "add", "base.txt")
	mustRunGit(t, dir, "commit", "-q", "-m", "diverge base")

	mustRunGit(t, dir, "checkout", "-q", "feature")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base changed on feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, dir, "add", "base.txt")
	mustRunGit(t, dir, "commit", "-q", "-m", "diverge feature")

	err := gitMerge(dir, "base")
	if err == nil {
		t.Fatal("gitMerge: expected an error for a conflicting merge, got nil")
	}

	status := mustRunGit(t, dir, "status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Errorf("gitMerge must leave a clean tree after aborting a conflicted merge, got status:\n%s", status)
	}
	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "feature" {
		t.Errorf("gitMerge must restore the original branch after aborting, got %q", branch)
	}
}

func TestGitMergeRefusesWhenAlreadyOnBase(t *testing.T) {
	dir := newTestRepo(t)
	mustRunGit(t, dir, "checkout", "-q", "base")

	err := gitMerge(dir, "base")
	if err == nil {
		t.Fatal("gitMerge while already on base: expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "already on base") {
		t.Errorf("gitMerge error = %v, want it to name being on base", err)
	}

	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "base" {
		t.Errorf("expected to still be on base, got %q", branch)
	}
}

func TestGitMergeRefusesDetachedHead(t *testing.T) {
	dir := newTestRepo(t)
	mustRunGit(t, dir, "checkout", "-q", "--detach", "HEAD")

	err := gitMerge(dir, "base")
	if err == nil {
		t.Fatal("gitMerge on a detached HEAD: expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "detached") {
		t.Errorf("gitMerge error = %v, want it to mention the detached HEAD", err)
	}

	head := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if head != "" {
		t.Errorf("expected HEAD to still be detached, got branch %q", head)
	}
}

// TestRunMergeHappyPath drives the documented end-to-end flow through
// runMerge itself: propose -> approve -> attest -> merge merges the feature
// branch into base, clears .writ/current.toml, and reports success.
func TestRunMergeHappyPath(t *testing.T) {
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

	cmd := newMergeCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runMerge: %v (stderr: %s)", err, out.String()+errOut.String())
	}
	if !strings.Contains(out.String(), "merged into base") {
		t.Errorf("runMerge output = %q, want it to report the merge into base", out.String())
	}

	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "base" {
		t.Errorf("expected to be on base after merge, got %q", branch)
	}
	if _, err := os.Stat(filepath.Join(dir, ".writ", "current.toml")); !os.IsNotExist(err) {
		t.Errorf("expected .writ/current.toml to be cleared after merge, got stat err %v", err)
	}
}

// proposeWritTOML proposes the given TOML in the current test repo.
func proposeWritTOML(t *testing.T, tomlSrc string) {
	t.Helper()
	cmd := newProposeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(tomlSrc))
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("propose: %v", err)
	}
}

// approveYes approves the open writ as-is, skipping the editor.
func approveYes(t *testing.T) {
	t.Helper()
	cmd := newApproveCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("approve: %v", err)
	}
}

// attestCriterion attests id with --note on the open writ.
func attestCriterion(t *testing.T, id string) {
	t.Helper()
	cmd := newAttestCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Flags().Set("note", "covered by tests"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{id}); err != nil {
		t.Fatalf("attest %s: %v", id, err)
	}
}

// TestRunMergeNeedsHumanRefusalThenApproveOverride locks in the README's
// merge contract end to end at Go level: a writ whose verification fails is
// needs-human, so plain merge refuses with exit 1 and names --approve, and
// merge --approve records the human approval and merges anyway.
func TestRunMergeNeedsHumanRefusalThenApproveOverride(t *testing.T) {
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
command = "false"
`)
	approveYes(t)
	attestCriterion(t, "c1")

	newCmd := func(approve bool) *cobra.Command {
		cmd := newMergeCmd()
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		if approve {
			if err := cmd.Flags().Set("approve", "true"); err != nil {
				t.Fatal(err)
			}
		}
		return cmd
	}

	err := newCmd(false).RunE(newCmd(false), nil)
	if ec, ok := err.(exitCodeErr); !ok || ec.code != 1 {
		t.Fatalf("merge needing human review without --approve: err = %v, want exitCodeErr{1}", err)
	}

	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "feature" {
		t.Errorf("a refused merge must not change branches, got %q", branch)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".writ", "current.toml")); statErr != nil {
		t.Errorf("a refused merge must keep the open writ: %v", statErr)
	}

	override := newCmd(true)
	if err := override.RunE(override, nil); err != nil {
		t.Fatalf("merge --approve: %v", err)
	}

	branch = strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "base" {
		t.Errorf("expected merge --approve to land on base, got %q", branch)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".writ", "current.toml")); !os.IsNotExist(statErr) {
		t.Errorf("expected .writ/current.toml cleared after merge --approve, got stat err %v", statErr)
	}
}

// TestRunMergeUnapprovedWritRefusalThenApproveOverride covers the other
// needs-human axis: a writ nobody has approved yet. Plain merge refuses with
// exit 1 without touching branches or state, and merge --approve - whose
// flag help promises to "record human approval and merge" - is the one way
// through, landing on base and clearing the state file.
func TestRunMergeUnapprovedWritRefusalThenApproveOverride(t *testing.T) {
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

	newCmd := func(approve bool) *cobra.Command {
		cmd := newMergeCmd()
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		if approve {
			if err := cmd.Flags().Set("approve", "true"); err != nil {
				t.Fatal(err)
			}
		}
		return cmd
	}

	err := newCmd(false).RunE(newCmd(false), nil)
	if ec, ok := err.(exitCodeErr); !ok || ec.code != 1 {
		t.Fatalf("merge of an unapproved writ without --approve: err = %v, want exitCodeErr{1}", err)
	}

	branch := strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "feature" {
		t.Errorf("a refused merge must not change branches, got %q", branch)
	}
	w, err := writ.Load(dir)
	if err != nil {
		t.Fatalf("a refused merge must keep the open writ: %v", err)
	}
	if w.Approved != nil {
		t.Error("a refused merge must leave the writ unapproved")
	}

	override := newCmd(true)
	var out bytes.Buffer
	override.SetOut(&out)
	if err := override.RunE(override, nil); err != nil {
		t.Fatalf("merge --approve of an unapproved writ: %v", err)
	}
	if !strings.Contains(out.String(), "human approval recorded via --approve") {
		t.Errorf("merge --approve output = %q, want it to record the human approval", out.String())
	}

	branch = strings.TrimSpace(mustRunGit(t, dir, "branch", "--show-current"))
	if branch != "base" {
		t.Errorf("expected merge --approve to land on base, got %q", branch)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".writ", "current.toml")); !os.IsNotExist(statErr) {
		t.Errorf("expected .writ/current.toml cleared after merge --approve, got stat err %v", statErr)
	}
}

func TestRunMergeNoWritOpen(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newMergeCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, nil)

	ec, ok := err.(exitCodeErr)
	if !ok {
		t.Fatalf("runMerge with no open writ: got err %v, want exitCodeErr", err)
	}
	if ec.code != 2 {
		t.Errorf("runMerge with no open writ: exit code = %d, want 2", ec.code)
	}
	if !strings.Contains(errOut.String(), "no writ is open") {
		t.Errorf("runMerge with no open writ: stderr = %q, want a helpful message", errOut.String())
	}
}

// withDir chdirs into dir for the duration of the test and restores the
// original working directory afterward.
func withDir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	})
}
