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

// repoRoot finds the root of the current git repository, falling back to the
// current working directory if git is unavailable.
func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	return os.Getwd()
}

// openInEditor opens path in $EDITOR, falling back to $VISUAL, then vi.
func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("running editor %q: %w", editor, err)
	}
	return nil
}
