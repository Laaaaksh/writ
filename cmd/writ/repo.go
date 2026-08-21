package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// errSilent signals that an error was already printed and main should exit
// non-zero without cobra printing anything further.
var errSilent = errors.New("")

// errNotInRepo is returned when the working directory is not inside a git
// repository. writ's whole model - drift against a base branch, merging the
// feature branch - is built on git, so there is no useful fallback: commands
// fail loudly here rather than silently treating an ordinary directory as a
// repo and scattering .writ/ state into it.
var errNotInRepo = errors.New("not inside a git repository; writ tracks work per repo - cd into one or run `git init` first")

// repoRoot finds the root of the git repository containing the working
// directory.
func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	return "", errNotInRepo
}

// openInEditor opens path in $EDITOR, falling back to $VISUAL, then vi.
// The editor value is launched through the shell exactly the way git
// launches GIT_EDITOR: `<editor> "$@"`. A bare command name behaves
// identically, while a value carrying arguments (EDITOR="code -w") works
// instead of dying with a confusing fork/exec "file not found".
func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command("sh", "-c", editor+` "$@"`, editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("running editor %q: %w", editor, err)
	}
	return nil
}
