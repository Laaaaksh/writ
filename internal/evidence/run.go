// Package evidence runs a writ's verification command and records the result.
package evidence

import (
	"errors"

	"github.com/Laaaaksh/writ/internal/writ"
)

// Report is the outcome of running a writ's verification command.
type Report struct {
	Ran      bool
	Passed   bool
	Command  string
	ExitCode int
	Output   string
	Summary  string
}

// Run executes w's verification command in repoDir and records the outcome.
func Run(w *writ.Writ, repoDir string) (*Report, error) {
	return nil, errors.New("not implemented")
}
