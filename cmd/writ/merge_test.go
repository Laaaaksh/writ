package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Laaaaksh/writ/internal/gate"
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
