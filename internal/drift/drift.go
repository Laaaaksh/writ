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

// emptyTreeHash is git's well-known empty tree object, which exists in every
// repository without being created or referenced by any commit.
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

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

	// A brand-new repo has no commits yet ("unborn HEAD"): neither base nor
	// HEAD resolves, and there is no committed history to compare anything
	// against. Skipping the base comparison and diffing staged work against
	// the empty tree keeps every file in the change set instead of failing,
	// while behaving identically to HEAD (everything is new) in that state.
	worktreeRev := emptyTreeHash
	if headExists(repoDir) {
		// The base must resolve before any diff runs against it. A base that
		// names nothing (a typo at propose time, or a branch deleted after
		// approval) would otherwise surface as raw git plumbing output, and a
		// blanked one is worse: git treats an empty left side of "base...HEAD"
		// as empty diff output rather than an error, so every committed change
		// would silently vanish from the report and the writ could pass as
		// zero-drift. Like the whole-repo scope and empty-criteria refusals,
		// this defends parseable-but-invalid state reachable by editing
		// .writ/current.toml after approval.
		if !revExists(repoDir, w.Base) {
			return nil, fmt.Errorf("writ names base %q, which does not exist in this repo; run `writ discard` and propose again with the correct base", w.Base)
		}
		if err := addNumstat(changes, repoDir, w.Base+"...HEAD"); err != nil {
			return nil, fmt.Errorf("diffing against base %q: %w", w.Base, err)
		}
		worktreeRev = "HEAD"
	}
	if err := addNumstat(changes, repoDir, worktreeRev); err != nil {
		return nil, fmt.Errorf("diffing working tree against %s: %w", worktreeRev, err)
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
		if isWritOwned(p) {
			continue
		}
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

// headExists reports whether repoDir has at least one commit. A repo before
// its first commit has an "unborn" HEAD, which git reports as a quiet
// rev-parse failure; any other git failure is indistinguishable here, but
// the diff that follows fails loudly on a real problem anyway.
func headExists(repoDir string) bool {
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", "-q", "HEAD")
	return cmd.Run() == nil
}

// revExists reports whether rev resolves to a commit in repoDir. Git ref
// names cannot contain whitespace, so a blank or whitespace-only base is
// refused here without consulting git at all.
func revExists(repoDir, rev string) bool {
	if strings.TrimSpace(rev) == "" {
		return false
	}
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", "-q", rev+"^{commit}")
	return cmd.Run() == nil
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

// isWritOwned reports whether path falls under writ's own bookkeeping
// directory (writ.Dir), which is excluded from both InScope and Drift
// entirely rather than classified into either. A plain prefix match on the
// string would also catch unrelated paths like ".writings/notes.md" or
// ".writ-backup", so this checks the directory component exactly.
func isWritOwned(path string) bool {
	return path == writ.Dir || strings.HasPrefix(path, writ.Dir+"/")
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
