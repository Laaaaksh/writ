package render

import (
	"strings"
	"testing"

	"github.com/Laaaaksh/writ/internal/drift"
	"github.com/Laaaaksh/writ/internal/evidence"
	"github.com/Laaaaksh/writ/internal/gate"
	"github.com/Laaaaksh/writ/internal/writ"
)

func boolPtr(b bool) *bool { return &b }

func sampleWrit() *writ.Writ {
	return &writ.Writ{
		ID:     "w1",
		Intent: "add cancel action to AI calls",
		Base:   "main",
		Scope:  []string{"internal/foo/**"},
		Criteria: []writ.Criterion{
			{ID: "c1", Text: "a", Met: boolPtr(true)},
			{ID: "c2", Text: "b", Met: boolPtr(true)},
			{ID: "c3", Text: "c", Met: boolPtr(true)},
			{ID: "c4", Text: "d", Met: boolPtr(true)},
		},
		Verify: writ.VerifySpec{Command: "go test ./..."},
	}
}

func TestStatusAutoMergeable(t *testing.T) {
	w := sampleWrit()
	d := &drift.Report{
		InScope: make([]drift.FileChange, 12),
	}
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "370 tests, 0 failures"}
	dec := gate.Decide(w, d, e)

	got := Status(w, d, e, dec)

	want := "" +
		"  writ    add cancel action to AI calls\n" +
		"\n" +
		"  CONTRACT     4/4 criteria met\n" +
		"  EVIDENCE     370 tests, 0 failures\n" +
		"  IN SCOPE     12 files\n" +
		"  DRIFT        none\n" +
		"\n" +
		"  Auto-mergeable: zero drift, verification passed, all criteria met.\n"

	if got != want {
		t.Errorf("Status mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestStatusWithDrift(t *testing.T) {
	w := sampleWrit()
	d := &drift.Report{
		InScope: make([]drift.FileChange, 12),
		Drift: []drift.FileChange{
			{Path: "config/routes.rb", Added: 7},
			{Path: "Gemfile", Added: 1},
		},
	}
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "370 tests, 0 failures"}
	dec := gate.Decide(w, d, e)

	got := Status(w, d, e, dec)

	want := "" +
		"  writ    add cancel action to AI calls\n" +
		"\n" +
		"  CONTRACT     4/4 criteria met\n" +
		"  EVIDENCE     370 tests, 0 failures\n" +
		"  IN SCOPE     12 files\n" +
		"  DRIFT        2 files outside declared scope\n" +
		"                 config/routes.rb   +7\n" +
		"                 Gemfile            +1\n" +
		"\n" +
		"  Needs you: 2 file(s) drifted outside the declared scope.\n"

	if got != want {
		t.Errorf("Status mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestStatusNeverPrintsFullInScopeList(t *testing.T) {
	w := sampleWrit()
	inScope := make([]drift.FileChange, 500)
	for i := range inScope {
		inScope[i] = drift.FileChange{Path: "file_that_should_never_appear.go", Added: 1}
	}
	d := &drift.Report{InScope: inScope}
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "ok"}
	dec := gate.Decide(w, d, e)

	got := Status(w, d, e, dec)

	if strings.Contains(got, "file_that_should_never_appear.go") {
		t.Errorf("Status printed an in-scope file path; it must only print the count:\n%s", got)
	}
	if !strings.Contains(got, "500 files") {
		t.Errorf("Status missing in-scope count:\n%s", got)
	}
}

func TestStatusTruncatesDriftListAt20(t *testing.T) {
	w := sampleWrit()
	drifted := make([]drift.FileChange, 25)
	for i := range drifted {
		drifted[i] = drift.FileChange{Path: strings.Repeat("a", i+1) + ".go", Added: i + 1}
	}
	d := &drift.Report{Drift: drifted}
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "ok"}
	dec := gate.Decide(w, d, e)

	got := Status(w, d, e, dec)

	lines := strings.Split(got, "\n")
	fileLines := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "                 ") {
			fileLines++
		}
	}
	// 20 shown files + 1 "... and N more" line.
	if fileLines != 21 {
		t.Errorf("expected 21 indented lines (20 files + truncation notice), got %d:\n%s", fileLines, got)
	}
	if !strings.Contains(got, "... and 5 more") {
		t.Errorf("Status missing truncation notice for the remaining 5 files:\n%s", got)
	}
	// The 21st file must not appear verbatim as its own listed entry.
	if strings.Count(got, ".go") != 20 {
		t.Errorf("expected exactly 20 listed files, got %d occurrences of \".go\":\n%s", strings.Count(got, ".go"), got)
	}
}

func TestStatusNilReports(t *testing.T) {
	w := sampleWrit()
	dec := gate.Decide(w, nil, nil)

	got := Status(w, nil, nil, dec)

	if !strings.Contains(got, "IN SCOPE     unknown") {
		t.Errorf("Status with nil drift report missing 'unknown' IN SCOPE line:\n%s", got)
	}
	if !strings.Contains(got, "DRIFT        unknown (drift not computed)") {
		t.Errorf("Status with nil drift report missing DRIFT line:\n%s", got)
	}
	if !strings.Contains(got, "Needs you:") {
		t.Errorf("Status with nil reports must need a human:\n%s", got)
	}
}

func TestStatusUnconfiguredVerification(t *testing.T) {
	w := sampleWrit()
	w.Verify.Command = ""
	d := &drift.Report{InScope: []drift.FileChange{{Path: "a.go", Added: 1}}}
	dec := gate.Decide(w, d, nil)

	got := Status(w, d, nil, dec)

	if !strings.Contains(got, "EVIDENCE     not configured") {
		t.Errorf("Status missing 'not configured' EVIDENCE line:\n%s", got)
	}
}

// Status is run in `go test`, where stdout is never a terminal, so its
// output is always plain text regardless of NO_COLOR. This exercises that
// path explicitly and also verifies no stray ANSI escapes leak through.
func TestStatusPlainTextWhenNotATerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	w := sampleWrit()
	d := &drift.Report{InScope: []drift.FileChange{{Path: "a.go", Added: 1}}}
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "ok"}
	dec := gate.Decide(w, d, e)

	got := Status(w, d, e, dec)

	if strings.Contains(got, "\033[") {
		t.Errorf("Status emitted ANSI escapes when output is not a terminal:\n%q", got)
	}
}

func TestUseColor(t *testing.T) {
	tests := []struct {
		name       string
		isTerminal bool
		noColor    string
		want       bool
	}{
		{"terminal and NO_COLOR unset", true, "", true},
		{"terminal and NO_COLOR set", true, "1", false},
		{"not a terminal and NO_COLOR unset", false, "", false},
		{"not a terminal and NO_COLOR set", false, "1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := useColor(tt.isTerminal, tt.noColor); got != tt.want {
				t.Errorf("useColor(%v, %q) = %v, want %v", tt.isTerminal, tt.noColor, got, tt.want)
			}
		})
	}
}
