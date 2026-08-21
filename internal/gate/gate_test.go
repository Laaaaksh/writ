package gate

import (
	"strings"
	"testing"

	"github.com/Laaaaksh/writ/internal/drift"
	"github.com/Laaaaksh/writ/internal/evidence"
	"github.com/Laaaaksh/writ/internal/writ"
)

func boolPtr(b bool) *bool { return &b }

func attestation(by string) *writ.Attestation {
	return &writ.Attestation{By: by, Note: "verified"}
}

func cleanWrit() *writ.Writ {
	return &writ.Writ{
		ID:       "w1",
		Intent:   "do the thing",
		Base:     "main",
		Scope:    []string{"internal/foo/**"},
		Approved: &writ.Approval{},
		Criteria: []writ.Criterion{
			{ID: "c1", Text: "the thing works", Met: boolPtr(true), Attestation: attestation("agent")},
			{ID: "c2", Text: "tests pass", Met: boolPtr(true), Attestation: attestation("human")},
		},
		Verify: writ.VerifySpec{Command: "go test ./..."},
	}
}

func cleanDrift() *drift.Report {
	return &drift.Report{
		InScope: []drift.FileChange{{Path: "internal/foo/foo.go", Added: 10, Deleted: 2}},
	}
}

func passingEvidence() *evidence.Report {
	return &evidence.Report{Ran: true, Passed: true, Command: "go test ./...", ExitCode: 0}
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name           string
		w              *writ.Writ
		d              *drift.Report
		e              *evidence.Report
		wantMergeable  bool
		wantNeedsHuman bool
		wantReasonHas  string // substring expected somewhere in Reasons; empty to skip
	}{
		{
			name:           "clean auto-merge",
			w:              cleanWrit(),
			d:              cleanDrift(),
			e:              passingEvidence(),
			wantMergeable:  true,
			wantNeedsHuman: false,
		},
		{
			name:           "nil drift report",
			w:              cleanWrit(),
			d:              nil,
			e:              passingEvidence(),
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  "drift could not be determined",
		},
		{
			name: "drifted files",
			w:    cleanWrit(),
			d: &drift.Report{
				InScope: []drift.FileChange{{Path: "internal/foo/foo.go", Added: 10}},
				Drift:   []drift.FileChange{{Path: "config/routes.rb", Added: 7}},
			},
			e:              passingEvidence(),
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  "drifted outside the declared scope",
		},
		{
			name:           "nil evidence report",
			w:              cleanWrit(),
			d:              cleanDrift(),
			e:              nil,
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  "verification did not run",
		},
		{
			name:           "verification never ran",
			w:              cleanWrit(),
			d:              cleanDrift(),
			e:              &evidence.Report{Ran: false, Command: "go test ./..."},
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  "verification did not run",
		},
		{
			name:           "verification failed",
			w:              cleanWrit(),
			d:              cleanDrift(),
			e:              &evidence.Report{Ran: true, Passed: false, Command: "go test ./...", ExitCode: 1},
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  "verification failed",
		},
		{
			name: "verification never configured",
			w: func() *writ.Writ {
				w := cleanWrit()
				w.Verify.Command = ""
				return w
			}(),
			d:              cleanDrift(),
			e:              nil,
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  "no verification command is configured",
		},
		{
			name: "writ not approved",
			w: func() *writ.Writ {
				w := cleanWrit()
				w.Approved = nil
				return w
			}(),
			d:              cleanDrift(),
			e:              passingEvidence(),
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  "writ has not been approved",
		},
		{
			name: "criterion met but not attested",
			w: func() *writ.Writ {
				w := cleanWrit()
				w.Criteria = []writ.Criterion{{ID: "c1", Text: "the thing works", Met: boolPtr(true)}}
				return w
			}(),
			d:              cleanDrift(),
			e:              passingEvidence(),
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  `criterion "c1" not attested`,
		},
		{
			name: "criterion met and attested by agent auto-merges",
			w: func() *writ.Writ {
				w := cleanWrit()
				w.Criteria = []writ.Criterion{{ID: "c1", Text: "the thing works", Met: boolPtr(true), Attestation: attestation("agent")}}
				return w
			}(),
			d:              cleanDrift(),
			e:              passingEvidence(),
			wantMergeable:  true,
			wantNeedsHuman: false,
		},
		{
			name: "criterion unassessed",
			w: func() *writ.Writ {
				w := cleanWrit()
				w.Criteria = []writ.Criterion{{ID: "c1", Text: "the thing works"}}
				return w
			}(),
			d:              cleanDrift(),
			e:              passingEvidence(),
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  `criterion "c1" not yet assessed`,
		},
		{
			name: "criterion explicitly not met",
			w: func() *writ.Writ {
				w := cleanWrit()
				w.Criteria = []writ.Criterion{{ID: "c1", Text: "the thing works", Met: boolPtr(false)}}
				return w
			}(),
			d:              cleanDrift(),
			e:              passingEvidence(),
			wantMergeable:  false,
			wantNeedsHuman: true,
			wantReasonHas:  `criterion "c1" not met`,
		},
		{
			name: "multiple reasons all reported",
			w: func() *writ.Writ {
				w := cleanWrit()
				w.Criteria = []writ.Criterion{{ID: "c1", Text: "the thing works"}}
				return w
			}(),
			d: &drift.Report{
				Drift: []drift.FileChange{{Path: "config/routes.rb", Added: 7}},
			},
			e:              &evidence.Report{Ran: true, Passed: false, Command: "go test ./...", ExitCode: 1},
			wantMergeable:  false,
			wantNeedsHuman: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := Decide(tt.w, tt.d, tt.e)

			if dec.Mergeable != tt.wantMergeable {
				t.Errorf("Mergeable = %v, want %v", dec.Mergeable, tt.wantMergeable)
			}
			if dec.NeedsHuman != tt.wantNeedsHuman {
				t.Errorf("NeedsHuman = %v, want %v", dec.NeedsHuman, tt.wantNeedsHuman)
			}
			if tt.wantMergeable && len(dec.Reasons) != 0 {
				t.Errorf("Reasons = %v, want empty for a mergeable decision", dec.Reasons)
			}
			if tt.wantReasonHas != "" {
				found := false
				for _, r := range dec.Reasons {
					if strings.Contains(r, tt.wantReasonHas) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Reasons = %v, want one containing %q", dec.Reasons, tt.wantReasonHas)
				}
			}
		})
	}
}

func TestDecideMultipleReasonsCollectsAll(t *testing.T) {
	w := cleanWrit()
	w.Criteria = []writ.Criterion{{ID: "c1", Text: "the thing works"}}
	d := &drift.Report{Drift: []drift.FileChange{{Path: "config/routes.rb", Added: 7}}}
	e := &evidence.Report{Ran: true, Passed: false, Command: "go test ./...", ExitCode: 1}

	dec := Decide(w, d, e)

	if len(dec.Reasons) != 3 {
		t.Fatalf("Reasons = %v, want 3 distinct reasons", dec.Reasons)
	}
	// Drift is named first: it is the core signal drift-only review is built around.
	if !strings.Contains(dec.Reasons[0], "drifted outside the declared scope") {
		t.Errorf("Reasons[0] = %q, want the drift reason first", dec.Reasons[0])
	}
}
