// Package gate decides whether a writ's work is mergeable given its drift
// and verification evidence.
package gate

import (
	"fmt"
	"strings"

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
// and verification evidence e. It is drift-only review: auto-mergeable only
// when there is zero drift, verification ran and passed, and every criterion
// is explicitly met. Any ambiguity - a nil report, an unassessed criterion,
// verification that never ran - sends it to a human instead of guessing.
func Decide(w *writ.Writ, d *drift.Report, e *evidence.Report) Decision {
	var reasons []string

	switch {
	case d == nil:
		reasons = append(reasons, "drift could not be determined")
	case len(d.Drift) > 0:
		reasons = append(reasons, fmt.Sprintf("%d file(s) drifted outside the declared scope", len(d.Drift)))
	}

	switch {
	case strings.TrimSpace(w.Verify.Command) == "":
		reasons = append(reasons, "no verification command is configured")
	case e == nil:
		reasons = append(reasons, "verification did not run")
	case !e.Ran:
		reasons = append(reasons, "verification did not run")
	case !e.Passed:
		reasons = append(reasons, fmt.Sprintf("verification failed: %s", e.Command))
	}

	for _, c := range w.Criteria {
		switch {
		case c.Met == nil:
			reasons = append(reasons, fmt.Sprintf("criterion %q not yet assessed", c.ID))
		case !*c.Met:
			reasons = append(reasons, fmt.Sprintf("criterion %q not met", c.ID))
		}
	}

	if len(reasons) == 0 {
		return Decision{Mergeable: true, NeedsHuman: false}
	}
	return Decision{Mergeable: false, NeedsHuman: true, Reasons: reasons}
}
