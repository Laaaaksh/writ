// Package writ defines the Writ type: the agreed scope of a piece of work,
// approved before any code exists.
package writ

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ErrNoWrit is returned by Load when no writ is open in the repo.
var ErrNoWrit = errors.New("no writ open")

// Dir is the directory, relative to a repo root, that holds writ's own
// runtime state. It is writ's bookkeeping, not the agent's work, so
// consumers such as drift detection must exclude it rather than treat it
// as in-scope or out-of-scope repo content.
const Dir = ".writ"

// writPath is the path, relative to a repo root, where the open writ lives.
const writPath = Dir + "/current.toml"

// wholeRepoScopes are scope entries that would make drift meaningless because
// they cover the entire repository.
var wholeRepoScopes = map[string]bool{
	"**":   true,
	"**/*": true,
	"*":    true,
	".":    true,
	"/":    true,
	"./**": true,
}

// Writ is the agreed scope of a piece of work: an intent, a set of checkable
// acceptance criteria, a declared file scope, and a verification command.
type Writ struct {
	ID       string      `toml:"id"`
	Intent   string      `toml:"intent"`
	Base     string      `toml:"base"` // branch to merge into, e.g. "main"
	Created  time.Time   `toml:"created"`
	Scope    []string    `toml:"scope"` // path globs the work may touch
	Criteria []Criterion `toml:"criteria"`
	Verify   VerifySpec  `toml:"verify"`
	Approved *Approval   `toml:"approved,omitempty"` // nil = proposed, not yet approved
}

// Approval records that a human has agreed to a writ's scope and criteria.
type Approval struct {
	At time.Time `toml:"at"`
}

// Criterion is a single checkable acceptance criterion.
type Criterion struct {
	ID          string       `toml:"id"`
	Text        string       `toml:"text"`
	Met         *bool        `toml:"met,omitempty"`         // nil = not yet assessed
	Attestation *Attestation `toml:"attestation,omitempty"` // who claims Met and how
}

// Attestation records who claims a criterion is met and how. It is a claim,
// not a fact: an agent attestation is unverified and must be rendered as
// such, distinct from machine-verified evidence.
type Attestation struct {
	By   string    `toml:"by"` // "agent" or "human"
	Note string    `toml:"note"`
	At   time.Time `toml:"at"`
}

// VerifySpec is the command that verifies the work meets its criteria.
type VerifySpec struct {
	Command string `toml:"command"`
}

// Load reads .writ/current.toml relative to repoDir.
// Returns ErrNoWrit if the file does not exist.
func Load(repoDir string) (*Writ, error) {
	path := filepath.Join(repoDir, writPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoWrit
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var w Writ
	if err := toml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &w, nil
}

// Save writes .writ/current.toml relative to repoDir, creating .writ/ if needed.
func (w *Writ) Save(repoDir string) error {
	dir := filepath.Join(repoDir, ".writ")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	path := filepath.Join(repoDir, writPath)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(w); err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	return nil
}

// Validate returns a non-nil error describing every problem with this writ.
func (w *Writ) Validate() error {
	var problems []string

	if strings.TrimSpace(w.ID) == "" {
		problems = append(problems, "id must not be empty")
	}
	if strings.TrimSpace(w.Intent) == "" {
		problems = append(problems, "intent must not be empty")
	}
	if strings.TrimSpace(w.Base) == "" {
		problems = append(problems, "base must not be empty")
	}

	if len(w.Scope) == 0 {
		problems = append(problems, "scope must not be empty")
	}
	for _, s := range w.Scope {
		if wholeRepoScopes[strings.TrimSpace(s)] {
			problems = append(problems, fmt.Sprintf("scope entry %q covers the whole repo, which defeats drift detection", s))
		}
	}

	if len(w.Criteria) < 1 {
		problems = append(problems, "at least one criterion is required")
	}
	seen := make(map[string]bool, len(w.Criteria))
	for _, c := range w.Criteria {
		if seen[c.ID] {
			problems = append(problems, fmt.Sprintf("duplicate criterion id %q", c.ID))
		}
		seen[c.ID] = true

		if c.Attestation != nil {
			if c.Attestation.By != "agent" && c.Attestation.By != "human" {
				problems = append(problems, fmt.Sprintf("criterion %q: attestation.by must be \"agent\" or \"human\", got %q", c.ID, c.Attestation.By))
			}
			if c.Met == nil || !*c.Met {
				problems = append(problems, fmt.Sprintf("criterion %q: attestation present but met is not true", c.ID))
			}
		}
	}

	if strings.TrimSpace(w.Verify.Command) == "" {
		problems = append(problems, "verify.command must not be empty")
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("invalid writ:\n  - %s", strings.Join(problems, "\n  - "))
}
