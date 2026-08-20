// Package render formats a writ's status for display to a human.
package render

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Laaaaksh/writ/internal/drift"
	"github.com/Laaaaksh/writ/internal/evidence"
	"github.com/Laaaaksh/writ/internal/gate"
	"github.com/Laaaaksh/writ/internal/writ"
)

// maxDriftLines is how many drifted files are listed individually before
// the rest are collapsed into a single "... and N more" line. The whole
// point of drift-only review is that a human does not read the full diff,
// so this list stays short even when drift is large.
const maxDriftLines = 20

const (
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

// Status renders a human-readable summary of w's drift, verification
// evidence, and merge decision.
func Status(w *writ.Writ, d *drift.Report, e *evidence.Report, dec gate.Decision) string {
	color := useColor(isTerminal(os.Stdout), os.Getenv("NO_COLOR"))

	var b strings.Builder

	fmt.Fprintf(&b, "  writ    %s\n\n", w.Intent)
	fmt.Fprintf(&b, "  %-13s%s\n", "CONTRACT", contractLine(w))
	for _, line := range criterionLines(w) {
		fmt.Fprintf(&b, "                 %s\n", line)
	}
	fmt.Fprintf(&b, "  %-13s%s\n", "EVIDENCE", evidenceLine(w, e))
	fmt.Fprintf(&b, "  %-13s%s\n", "IN SCOPE", inScopeLine(d))
	fmt.Fprintf(&b, "  %-13s%s\n", "DRIFT", driftLine(d))
	for _, line := range driftFileLines(d) {
		fmt.Fprintf(&b, "                 %s\n", line)
	}

	fmt.Fprintf(&b, "\n  %s\n", verdictLine(dec, color))

	return b.String()
}

func contractLine(w *writ.Writ) string {
	met := 0
	for _, c := range w.Criteria {
		if c.Met != nil && *c.Met {
			met++
		}
	}
	return fmt.Sprintf("%d/%d criteria", met, len(w.Criteria))
}

// criterionLines renders one line per criterion showing its provenance,
// visibly distinct: a machine cannot tell agent claims from human
// confirmation apart from evidence, so a reader must never have to either.
func criterionLines(w *writ.Writ) []string {
	if len(w.Criteria) == 0 {
		return nil
	}

	idWidth := 0
	for _, c := range w.Criteria {
		if len(c.ID) > idWidth {
			idWidth = len(c.ID)
		}
	}

	lines := make([]string, 0, len(w.Criteria))
	for _, c := range w.Criteria {
		lines = append(lines, fmt.Sprintf("%-*s   %s", idWidth, c.ID, provenance(c)))
	}
	return lines
}

func provenance(c writ.Criterion) string {
	if c.Attestation == nil {
		return "not assessed"
	}
	switch c.Attestation.By {
	case "human":
		return "confirmed by you"
	default: // "agent"
		return fmt.Sprintf("claimed by agent   %q", c.Attestation.Note)
	}
}

func evidenceLine(w *writ.Writ, e *evidence.Report) string {
	if strings.TrimSpace(w.Verify.Command) == "" {
		return "not configured"
	}
	if e == nil || !e.Ran {
		return "did not run"
	}
	if e.Summary != "" {
		return e.Summary
	}
	if e.Passed {
		return fmt.Sprintf("passed (exit %d)", e.ExitCode)
	}
	return fmt.Sprintf("failed (exit %d)", e.ExitCode)
}

func inScopeLine(d *drift.Report) string {
	if d == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d files", len(d.InScope))
}

func driftLine(d *drift.Report) string {
	if d == nil {
		return "unknown (drift not computed)"
	}
	n := len(d.Drift)
	if n == 0 {
		return "none"
	}
	if n == 1 {
		return "1 file outside declared scope"
	}
	return fmt.Sprintf("%d files outside declared scope", n)
}

// driftFileLines renders the capped, aligned per-file drift list. It returns
// nil when there is no drift to show, so Status never emits an empty block.
func driftFileLines(d *drift.Report) []string {
	if d == nil || len(d.Drift) == 0 {
		return nil
	}

	shown := d.Drift
	truncated := 0
	if len(shown) > maxDriftLines {
		truncated = len(shown) - maxDriftLines
		shown = shown[:maxDriftLines]
	}

	pathWidth := 0
	for _, fc := range shown {
		if len(fc.Path) > pathWidth {
			pathWidth = len(fc.Path)
		}
	}

	lines := make([]string, 0, len(shown)+1)
	for _, fc := range shown {
		lines = append(lines, fmt.Sprintf("%-*s   %s", pathWidth, fc.Path, changeSummary(fc)))
	}
	if truncated > 0 {
		lines = append(lines, fmt.Sprintf("... and %d more", truncated))
	}
	return lines
}

func changeSummary(fc drift.FileChange) string {
	switch {
	case fc.Added > 0 && fc.Deleted > 0:
		return "+" + strconv.Itoa(fc.Added) + " -" + strconv.Itoa(fc.Deleted)
	case fc.Deleted > 0:
		return "-" + strconv.Itoa(fc.Deleted)
	default:
		return "+" + strconv.Itoa(fc.Added)
	}
}

func verdictLine(dec gate.Decision, color bool) string {
	if dec.Mergeable {
		msg := "Auto-mergeable: zero drift, verification passed, all criteria met."
		if color {
			return colorGreen + msg + colorReset
		}
		return msg
	}

	msg := "Needs you: " + strings.Join(dec.Reasons, "; ") + "."
	if color {
		return colorRed + msg + colorReset
	}
	return msg
}

// useColor reports whether ANSI color codes should be used: only when
// stdout is a terminal and NO_COLOR is unset, so output stays pipeable.
func useColor(isTerm bool, noColor string) bool {
	return isTerm && noColor == ""
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
