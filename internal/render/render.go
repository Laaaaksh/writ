// Package render formats a writ's status for display to a human.
package render

import (
	"github.com/Laaaaksh/writ/internal/drift"
	"github.com/Laaaaksh/writ/internal/evidence"
	"github.com/Laaaaksh/writ/internal/gate"
	"github.com/Laaaaksh/writ/internal/writ"
)

// Status renders a human-readable summary of w's drift, verification
// evidence, and merge decision.
func Status(w *writ.Writ, d *drift.Report, e *evidence.Report, dec gate.Decision) string {
	panic("not implemented")
}
