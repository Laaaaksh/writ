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
		ID:       "w1",
		Intent:   "add cancel action to AI calls",
		Base:     "main",
		Scope:    []string{"internal/foo/**"},
		Approved: &writ.Approval{},
		Criteria: []writ.Criterion{
			{ID: "c1", Text: "a", Met: boolPtr(true), Attestation: &writ.Attestation{By: "agent", Note: "n1"}},
			{ID: "c2", Text: "b", Met: boolPtr(true), Attestation: &writ.Attestation{By: "agent", Note: "n2"}},
			{ID: "c3", Text: "c", Met: boolPtr(true), Attestation: &writ.Attestation{By: "agent", Note: "n3"}},
			{ID: "c4", Text: "d", Met: boolPtr(true), Attestation: &writ.Attestation{By: "agent", Note: "n4"}},
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
		"  CONTRACT     4/4 criteria\n" +
		"                 c1   claimed by agent   \"n1\"\n" +
		"                 c2   claimed by agent   \"n2\"\n" +
		"                 c3   claimed by agent   \"n3\"\n" +
		"                 c4   claimed by agent   \"n4\"\n" +
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
		"  CONTRACT     4/4 criteria\n" +
		"                 c1   claimed by agent   \"n1\"\n" +
		"                 c2   claimed by agent   \"n2\"\n" +
		"                 c3   claimed by agent   \"n3\"\n" +
		"                 c4   claimed by agent   \"n4\"\n" +
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
	// 4 criterion lines + 20 shown files + 1 "... and N more" line.
	if fileLines != 25 {
		t.Errorf("expected 25 indented lines (4 criteria + 20 files + truncation notice), got %d:\n%s", fileLines, got)
	}
	if !strings.Contains(got, "... and 5 more") {
		t.Errorf("Status missing truncation notice for the remaining 5 files:\n%s", got)
	}
	// The 21st file must not appear verbatim as its own listed entry.
	if strings.Count(got, ".go") != 20 {
		t.Errorf("expected exactly 20 listed files, got %d occurrences of \".go\":\n%s", strings.Count(got, ".go"), got)
	}
}

func TestStatusShowsAllThreeProvenanceStates(t *testing.T) {
	w := &writ.Writ{
		ID:       "w1",
		Intent:   "add cancel action to AI calls",
		Base:     "main",
		Scope:    []string{"internal/foo/**"},
		Approved: &writ.Approval{},
		Criteria: []writ.Criterion{
			{ID: "cancels-queued", Text: "a", Met: boolPtr(true), Attestation: &writ.Attestation{By: "agent", Note: "cancel path covered by spec"}},
			{ID: "state-transition", Text: "b", Met: boolPtr(true), Attestation: &writ.Attestation{By: "agent", Note: "asserted in model spec"}},
			{ID: "confirmed-by-you", Text: "c", Met: boolPtr(true), Attestation: &writ.Attestation{By: "human", Note: "checked by hand"}},
			{ID: "no-regression", Text: "d"},
		},
		Verify: writ.VerifySpec{Command: "go test ./..."},
	}
	d := &drift.Report{InScope: []drift.FileChange{{Path: "a.go", Added: 1}}}
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "ok"}
	dec := gate.Decide(w, d, e)

	got := Status(w, d, e, dec)

	want := "" +
		"  writ    add cancel action to AI calls\n" +
		"\n" +
		"  CONTRACT     3/4 criteria\n" +
		"                 cancels-queued     claimed by agent   \"cancel path covered by spec\"\n" +
		"                 state-transition   claimed by agent   \"asserted in model spec\"\n" +
		"                 confirmed-by-you   confirmed by you\n" +
		"                 no-regression      not assessed\n" +
		"  EVIDENCE     ok\n" +
		"  IN SCOPE     1 files\n" +
		"  DRIFT        none\n" +
		"\n" +
		"  Needs you: criterion \"no-regression\" not yet assessed.\n"

	if got != want {
		t.Errorf("Status mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
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

// Exactly one drifted file must use the singular form in both the DRIFT
// line and the verdict; "1 files" is the kind of sloppiness users notice.
func TestStatusSingleDriftFileUsesSingular(t *testing.T) {
	w := sampleWrit()
	d := &drift.Report{Drift: []drift.FileChange{{Path: "cmd/rogue.sh", Added: 3}}}
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "ok"}
	dec := gate.Decide(w, d, e)

	got := statusString(w, d, e, dec, false)

	want := "" +
		"  writ    add cancel action to AI calls\n" +
		"\n" +
		"  CONTRACT     4/4 criteria\n" +
		"                 c1   claimed by agent   \"n1\"\n" +
		"                 c2   claimed by agent   \"n2\"\n" +
		"                 c3   claimed by agent   \"n3\"\n" +
		"                 c4   claimed by agent   \"n4\"\n" +
		"  EVIDENCE     ok\n" +
		"  IN SCOPE     0 files\n" +
		"  DRIFT        1 file outside declared scope\n" +
		"                 cmd/rogue.sh   +3\n" +
		"\n" +
		"  Needs you: 1 file(s) drifted outside the declared scope.\n"

	if got != want {
		t.Errorf("Status mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if strings.Contains(got, "1 files") {
		t.Errorf("singular drift count rendered as plural:\n%s", got)
	}
}

// A modified file shows both directions, a deleted file must not render a
// meaningless "+0", and an empty-change entry (mode flip, binary) degrades
// to "+0" rather than something blank.
func TestStatusChangeSummaries(t *testing.T) {
	w := sampleWrit()
	d := &drift.Report{Drift: []drift.FileChange{
		{Path: "edited.txt", Added: 2, Deleted: 5},
		{Path: "gone.txt", Deleted: 7},
	}}
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "ok"}
	dec := gate.Decide(w, d, e)

	got := statusString(w, d, e, dec, false)

	if !strings.Contains(got, "edited.txt   +2 -5") {
		t.Errorf("mixed change not rendered as \"+2 -5\":\n%s", got)
	}
	if !strings.Contains(got, "gone.txt     -7\n") {
		t.Errorf("deletion-only change rendered wrong (want bare \"-7\", aligned under edited.txt):\n%s", got)
	}
	if strings.Contains(got, "+0 -7") {
		t.Errorf("deletion-only change rendered a spurious +0:\n%s", got)
	}
}

// The colored verdicts are what every interactive terminal user sees on
// every status run; they must wrap exactly the verdict message in green or
// red and always reset, while color=false never emits escapes.
func TestStatusVerdictColors(t *testing.T) {
	w := sampleWrit()
	e := &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", Summary: "ok"}

	clean := &drift.Report{}
	green := statusString(w, clean, e, gate.Decide(w, clean, e), true)
	const greenMsg = "\033[32mAuto-mergeable: zero drift, verification passed, all criteria met.\033[0m"
	if !strings.Contains(green, greenMsg) {
		t.Errorf("mergeable verdict not wrapped in green with a reset:\n%q", green)
	}

	drifted := &drift.Report{Drift: []drift.FileChange{{Path: "x.go", Added: 1}}}
	red := statusString(w, drifted, e, gate.Decide(w, drifted, e), true)
	if !strings.HasPrefix(strings.TrimSpace(red[strings.Index(red, "\033[31m"):]), "\033[31mNeeds you:") {
		t.Errorf("needs-human verdict not prefixed with red after trimming indent:\n%q", red)
	}
	if !strings.Contains(red, ".\033[0m\n") {
		t.Errorf("needs-human verdict missing reset before its trailing newline:\n%q", red)
	}

	plain := statusString(w, clean, e, gate.Decide(w, clean, e), false)
	if strings.Contains(plain, "\033[") {
		t.Errorf("color=false emitted ANSI escapes:\n%q", plain)
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
