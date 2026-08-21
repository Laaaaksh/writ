package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepoRootOutsideRepoFails proves writ refuses to run outside a git
// repository: the old silent fallback to os.Getwd() let propose scatter
// .writ/ state into an ordinary directory and made status report a
// misleading "no writ is open".
func TestRepoRootOutsideRepoFails(t *testing.T) {
	dir := t.TempDir()
	withDir(t, dir)

	root, err := repoRoot()
	if err == nil {
		t.Fatalf("repoRoot outside any git repo = %q, want an error", root)
	}
	if !errors.Is(err, errNotInRepo) {
		t.Errorf("repoRoot error = %v, want errNotInRepo", err)
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("repoRoot error = %v, want it to name the problem for the user", err)
	}
}

func TestRepoRootInsideRepo(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot inside a git repo: %v", err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := filepath.EvalSymlinks(root); got != want {
		t.Errorf("repoRoot() = %q, want %q", got, want)
	}
}
