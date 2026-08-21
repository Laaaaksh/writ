// Package drift computes what actually changed in a repo and compares it
// against a writ's declared scope.
package drift

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Laaaaksh/writ/internal/writ"
)

// FileChange describes a single changed file.
type FileChange struct {
	Path           string
	Added, Deleted int
}

// Report is the result of comparing actual changes against a writ's scope.
type Report struct {
	InScope []FileChange
	Drift   []FileChange
}

// Compute determines which files changed in repoDir and splits them into
// those covered by w's declared scope and those that drifted outside it.
//
// The change set is the union of: commits introduced on the current branch
// since w.Base (three-dot diff, so changes that landed on base meanwhile are
// excluded), uncommitted work (staged and unstaged) against HEAD, and
// untracked files. A file touched by more than one of those sources is
// reported once, with totals summed.
func Compute(w *writ.Writ, repoDir string) (*Report, error) {
	changes := make(map[string]*FileChange)

	if err := addNumstat(changes, repoDir, w.Base+"...HEAD"); err != nil {
		return nil, fmt.Errorf("diffing against base %q: %w", w.Base, err)
	}
	if err := addNumstat(changes, repoDir, "HEAD"); err != nil {
		return nil, fmt.Errorf("diffing working tree against HEAD: %w", err)
	}
	if err := addUntracked(changes, repoDir); err != nil {
		return nil, fmt.Errorf("listing untracked files: %w", err)
	}

	matchers := make([]*regexp.Regexp, len(w.Scope))
	for i, pattern := range w.Scope {
		matchers[i] = regexp.MustCompile(globToRegex(pattern))
	}

	paths := make([]string, 0, len(changes))
	for p := range changes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	report := &Report{}
	for _, p := range paths {
		fc := *changes[p]
		if matchesScope(matchers, p) {
			report.InScope = append(report.InScope, fc)
		} else {
			report.Drift = append(report.Drift, fc)
		}
	}
	return report, nil
}

// runGit runs git in repoDir and returns stdout, or an error carrying git's
// stderr when it exits non-zero.
func runGit(repoDir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// addNumstat runs `git diff --numstat` for rev and merges the result into
// changes, summing totals for paths already present.
//
// -z sidesteps git's path quoting entirely (spaces, unicode, and unusual
// bytes all come through verbatim), and -M forces rename detection so a
// rename always appears as one old=>new entry rather than a delete+add pair.
func addNumstat(changes map[string]*FileChange, repoDir, rev string) error {
	out, err := runGit(repoDir, "diff", "--numstat", "-M", "-z", rev)
	if err != nil {
		return err
	}
	return parseNumstatZ(out, changes)
}

// parseNumstatZ parses the NUL-delimited output of `git diff --numstat -z`.
//
// Each entry is normally one token "<added>\t<deleted>\t<path>". For a
// rename or copy, the path field is empty and is followed by two further
// NUL-terminated tokens: the old path, then the new path - git's way of
// disambiguating without needing the "old => new" text form (and its
// common-prefix-compressed "a/{old => new}/b" variant) that non-z mode uses.
func parseNumstatZ(out []byte, changes map[string]*FileChange) error {
	if len(out) == 0 {
		return nil
	}
	tokens := strings.Split(string(out), "\x00")
	if len(tokens) > 0 && tokens[len(tokens)-1] == "" {
		tokens = tokens[:len(tokens)-1]
	}

	i := 0
	for i < len(tokens) {
		fields := strings.SplitN(tokens[i], "\t", 3)
		if len(fields) != 3 {
			return fmt.Errorf("unexpected numstat entry %q", tokens[i])
		}
		addedField, deletedField, pathField := fields[0], fields[1], fields[2]

		var path string
		if pathField == "" {
			if i+2 >= len(tokens) {
				return fmt.Errorf("truncated rename entry at %q", tokens[i])
			}
			path = tokens[i+2] // new path; old path (tokens[i+1]) doesn't matter for scope matching
			i += 3
		} else {
			path = pathField
			i++
		}

		added, err := parseNumstatCount(addedField)
		if err != nil {
			return fmt.Errorf("parsing added count for %q: %w", path, err)
		}
		deleted, err := parseNumstatCount(deletedField)
		if err != nil {
			return fmt.Errorf("parsing deleted count for %q: %w", path, err)
		}

		fc := changes[path]
		if fc == nil {
			fc = &FileChange{Path: path}
			changes[path] = fc
		}
		fc.Added += added
		fc.Deleted += deleted
	}
	return nil
}

// parseNumstatCount parses one numstat count field. Binary files report "-"
// instead of a number.
func parseNumstatCount(field string) (int, error) {
	if field == "-" {
		return 0, nil
	}
	return strconv.Atoi(field)
}

// addUntracked adds every untracked file (per `git status --porcelain`) to
// changes as a pure addition; an agent creating a new file outside scope is
// real drift and must not be missed.
func addUntracked(changes map[string]*FileChange, repoDir string) error {
	out, err := runGit(repoDir, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(out) == 0 {
		return nil
	}
	tokens := strings.Split(string(out), "\x00")
	if len(tokens) > 0 && tokens[len(tokens)-1] == "" {
		tokens = tokens[:len(tokens)-1]
	}

	for _, tok := range tokens {
		if !strings.HasPrefix(tok, "??") {
			continue
		}
		path := strings.TrimPrefix(tok, "?? ")

		fc := changes[path]
		if fc == nil {
			fc = &FileChange{Path: path}
			changes[path] = fc
		}
		if added, err := countLines(filepath.Join(repoDir, path)); err == nil {
			fc.Added += added
		}
	}
	return nil
}

// countLines returns a line count for an untracked file, or 0 for a binary
// one. Line-count precision on an unusual untracked file (unreadable,
// symlink) isn't worth failing the whole report over, so callers ignore
// errors from this and leave the addition at 0.
func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return 0, nil // binary
	}
	lines := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines, nil
}

// matchesScope reports whether path matches any of the compiled scope globs.
func matchesScope(matchers []*regexp.Regexp, path string) bool {
	for _, m := range matchers {
		if m.MatchString(path) {
			return true
		}
	}
	return false
}

// globToRegex compiles a scope glob into an anchored regex pattern.
// Supported wildcards, matched against a forward-slash path relative to the
// repo root:
//
//   - "**"   any depth, including zero directories
//   - "*"    any run of characters within one path segment (not "/")
//   - "?"    a single character within one path segment (not "/")
//
// filepath.Match doesn't support "**", so this is a small hand-rolled
// translator rather than a dependency: a "**/" bounded by slashes (or start)
// becomes an optional "any segments" group so "a/**/b" also matches "a/b";
// any other "**" becomes an unrestricted ".*".
func globToRegex(glob string) string {
	var sb strings.Builder
	sb.WriteByte('^')

	n := len(glob)
	for i := 0; i < n; {
		c := glob[i]
		switch c {
		case '*':
			if i+1 < n && glob[i+1] == '*' {
				prevBoundary := i == 0 || glob[i-1] == '/'
				nextIsSlash := i+2 < n && glob[i+2] == '/'
				if prevBoundary && nextIsSlash {
					sb.WriteString("(?:.*/)?")
					i += 3
					continue
				}
				sb.WriteString(".*")
				i += 2
				continue
			}
			sb.WriteString("[^/]*")
			i++
		case '?':
			sb.WriteString("[^/]")
			i++
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
			i++
		}
	}

	sb.WriteByte('$')
	return sb.String()
}
