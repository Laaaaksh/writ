// Package drift computes what actually changed in a repo and compares it
// against a writ's declared scope.
package drift

import (
	"errors"

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
func Compute(w *writ.Writ, repoDir string) (*Report, error) {
	return nil, errors.New("not implemented")
}
