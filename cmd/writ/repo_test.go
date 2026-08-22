package main

import (
	"errors"
	"fmt"
	"os"
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

// TestRepoRootNamesMissingGit proves that when the git binary itself is
// absent from PATH, writ names that instead of claiming "not inside a git
// repository" and advising `git init`, which cannot succeed without git.
func TestRepoRootNamesMissingGit(t *testing.T) {
	empty := t.TempDir()
	dir := newTestRepo(t)
	withDir(t, dir)
	t.Setenv("PATH", empty)

	root, err := repoRoot()
	if err == nil {
		t.Fatalf("repoRoot without git in PATH = %q, want an error", root)
	}
	if errors.Is(err, errNotInRepo) {
		t.Errorf("repoRoot error = %v, want a missing-git message rather than errNotInRepo", err)
	}
	for _, want := range []string{"git was not found in PATH", "install it first"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("repoRoot error %q does not mention %q", err, want)
		}
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

// TestOpenInEditorSupportsEditorWithArguments proves EDITOR values that
// carry arguments (e.g. "code -w") launch correctly: the editor is run
// through the shell the same way git runs GIT_EDITOR.
func TestOpenInEditorSupportsEditorWithArguments(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "editor.sh")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n", argsFile)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", script+" -w")

	target := filepath.Join(dir, "target.toml")
	if err := openInEditor(target); err != nil {
		t.Fatalf("openInEditor with arguments in EDITOR: %v", err)
	}

	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("editor script never ran: %v", err)
	}
	want := "-w\n" + target + "\n"
	if string(got) != want {
		t.Errorf("editor received args %q, want %q", got, want)
	}
}
