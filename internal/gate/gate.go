// Package gate decides whether a writ's work is mergeable given its drift
// and verification evidence.
package gate

import (
	"github.com/Laaaaksh/writ/internal/drift"
	"github.com/Laaaaksh/writ/internal/evidence"
	"github.com/Laaaaksh/writ/internal/writ"
)

// Decision is the merge/no-merge outcome for a writ.
type Decision struct {
	Mergeable  bool
	NeedsHuman bool
	Reasons    []string
}

// Decide determines whether w's work is mergeable given its drift report d
// and verification evidence e.
func Decide(w *writ.Writ, d *drift.Report, e *evidence.Report) Decision {
	panic("not implemented")
}
