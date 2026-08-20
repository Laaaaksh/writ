package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Laaaaksh/writ/internal/writ"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <intent>",
		Short: "Open a new writ for the given intent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOpen(cmd, args[0])
		},
	}
}

func runOpen(cmd *cobra.Command, intent string) error {
	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	if _, err := writ.Load(repoDir); !errors.Is(err, writ.ErrNoWrit) {
		if err == nil {
			return fmt.Errorf("a writ is already open in this repo (.writ/current.toml); close it before opening another")
		}
		return err
	}

	tmpl := writTemplate(intent, defaultBase(repoDir))
	path := filepath.Join(repoDir, ".writ", "current.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating .writ directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(tmpl), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	if err := openInEditor(path); err != nil {
		return err
	}

	w, err := writ.Load(repoDir)
	if err != nil {
		return fmt.Errorf("reloading %s: %w", path, err)
	}
	if err := w.Validate(); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		fmt.Fprintf(cmd.ErrOrStderr(), "\nfix the errors above and edit %s directly, then try again\n", path)
		return errSilent
	}

	fmt.Fprintf(cmd.OutOrStdout(), "writ opened: %s\n", path)
	return nil
}

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

// defaultBase returns the repo's current default branch, falling back to
// "main" if it cannot be determined.
func defaultBase(repoDir string) string {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		return strings.TrimPrefix(ref, "refs/remotes/origin/")
	}

	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoDir
	if out, err := cmd.Output(); err == nil {
		if branch := strings.TrimSpace(string(out)); branch != "" {
			return branch
		}
	}

	return "main"
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

func writTemplate(intent, base string) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# writ: %s\n", intent)
	fmt.Fprintf(&b, "# Agreed scope for this work, approved before any code exists.\n")
	fmt.Fprintf(&b, "# Fill in scope, criteria, and verify below, then save and exit.\n\n")
	fmt.Fprintf(&b, "id = %q\n", generateID())
	fmt.Fprintf(&b, "intent = %q\n", intent)
	fmt.Fprintf(&b, "base = %q\n", base)
	fmt.Fprintf(&b, "created = %s\n\n", time.Now().UTC().Format(time.RFC3339))

	b.WriteString("# Path globs the work is allowed to touch. Anything changed outside these\n")
	b.WriteString("# globs is drift. Must not cover the whole repo (e.g. \"**\", \"*\", \".\", \"/\").\n")
	b.WriteString("# Example:\n")
	b.WriteString("#   scope = [\"internal/foo/**\", \"cmd/foo/main.go\"]\n")
	b.WriteString("scope = []\n\n")

	b.WriteString("# Checkable acceptance criteria. At least one is required, each with a unique id.\n")
	b.WriteString("# Example:\n")
	b.WriteString("#   [[criteria]]\n")
	b.WriteString("#   id = \"c1\"\n")
	b.WriteString("#   text = \"the thing works\"\n\n")

	b.WriteString("# The command that verifies this work meets its criteria.\n")
	b.WriteString("[verify]\n")
	b.WriteString("# command = \"go test ./...\"\n")
	b.WriteString("command = \"\"\n")

	return b.String()
}

func generateID() string {
	return "writ-" + time.Now().UTC().Format("20060102-150405")
}
