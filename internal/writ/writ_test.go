package writ

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validWrit() *Writ {
	met := true
	return &Writ{
		ID:      "w1",
		Intent:  "do the thing",
		Base:    "main",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Scope:   []string{"internal/foo/**", "cmd/foo/main.go"},
		Criteria: []Criterion{
			{ID: "c1", Text: "the thing works", Met: &met},
			{ID: "c2", Text: "tests pass"},
		},
		Verify: VerifySpec{Command: "go test ./..."},
	}
}

func approvedWrit() *Writ {
	met := true
	w := validWrit()
	w.Approved = &Approval{At: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}
	w.Criteria[0].Attestation = &Attestation{
		By:   "agent",
		Note: "covered by unit tests",
		At:   time.Date(2026, 1, 2, 1, 0, 0, 0, time.UTC),
	}
	w.Criteria[1].Met = &met
	w.Criteria[1].Attestation = &Attestation{
		By:   "human",
		Note: "confirmed manually",
		At:   time.Date(2026, 1, 2, 2, 0, 0, 0, time.UTC),
	}
	return w
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	w := validWrit()

	if err := w.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.ID != w.ID || got.Intent != w.Intent || got.Base != w.Base {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, w)
	}
	if !got.Created.Equal(w.Created) {
		t.Fatalf("Created mismatch: got %v, want %v", got.Created, w.Created)
	}
	if len(got.Scope) != len(w.Scope) {
		t.Fatalf("Scope mismatch: got %v, want %v", got.Scope, w.Scope)
	}
	if len(got.Criteria) != len(w.Criteria) {
		t.Fatalf("Criteria mismatch: got %+v, want %+v", got.Criteria, w.Criteria)
	}
	if got.Criteria[0].Met == nil || *got.Criteria[0].Met != true {
		t.Fatalf("Criteria[0].Met mismatch: got %+v", got.Criteria[0])
	}
	if got.Criteria[1].Met != nil {
		t.Fatalf("Criteria[1].Met should remain nil, got %v", *got.Criteria[1].Met)
	}
	if got.Verify.Command != w.Verify.Command {
		t.Fatalf("Verify mismatch: got %+v, want %+v", got.Verify, w.Verify)
	}
}

func TestSaveLoadRoundTripApprovalAndAttestations(t *testing.T) {
	dir := t.TempDir()
	w := approvedWrit()

	if err := w.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Approved == nil {
		t.Fatal("Approved should round-trip non-nil")
	}
	if !got.Approved.At.Equal(w.Approved.At) {
		t.Errorf("Approved.At mismatch: got %v, want %v", got.Approved.At, w.Approved.At)
	}

	if len(got.Criteria) != 2 {
		t.Fatalf("Criteria mismatch: got %+v", got.Criteria)
	}

	c0 := got.Criteria[0]
	if c0.Attestation == nil {
		t.Fatal("Criteria[0].Attestation should round-trip non-nil")
	}
	if c0.Attestation.By != "agent" || c0.Attestation.Note != "covered by unit tests" {
		t.Errorf("Criteria[0].Attestation mismatch: got %+v", c0.Attestation)
	}
	if !c0.Attestation.At.Equal(w.Criteria[0].Attestation.At) {
		t.Errorf("Criteria[0].Attestation.At mismatch: got %v, want %v", c0.Attestation.At, w.Criteria[0].Attestation.At)
	}

	c1 := got.Criteria[1]
	if c1.Attestation == nil || c1.Attestation.By != "human" {
		t.Errorf("Criteria[1].Attestation mismatch: got %+v", c1.Attestation)
	}
	if c1.Met == nil || !*c1.Met {
		t.Errorf("Criteria[1].Met should round-trip true, got %+v", c1.Met)
	}
}

func TestLoadNoWrit(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir)
	if !errors.Is(err, ErrNoWrit) {
		t.Fatalf("Load on empty dir: got %v, want ErrNoWrit", err)
	}
}

func TestValidateValid(t *testing.T) {
	if err := validWrit().Validate(); err != nil {
		t.Fatalf("Validate on a valid writ: %v", err)
	}
}

func TestIsWholeRepoScope(t *testing.T) {
	tests := []struct {
		entry string
		want  bool
	}{
		{"**", true},
		{"**/*", true},
		{"*", true},
		{".", true},
		{"/", true},
		{"./**", true},
		{"  **  ", true}, // surrounding whitespace must not smuggle it past the check
		{"internal/**", false},
		{"cmd/foo/main.go", false},
		{"a**b", false},
		{".hidden/**", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsWholeRepoScope(tt.entry); got != tt.want {
			t.Errorf("IsWholeRepoScope(%q) = %v, want %v", tt.entry, got, tt.want)
		}
	}
}

func TestValidateRejections(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(w *Writ)
	}{
		{
			name:   "empty intent",
			mutate: func(w *Writ) { w.Intent = "" },
		},
		{
			name:   "empty id",
			mutate: func(w *Writ) { w.ID = "" },
		},
		{
			name:   "empty base",
			mutate: func(w *Writ) { w.Base = "" },
		},
		{
			name:   "empty scope",
			mutate: func(w *Writ) { w.Scope = nil },
		},
		{
			name:   "scope is **",
			mutate: func(w *Writ) { w.Scope = []string{"**"} },
		},
		{
			name:   "scope is **/*",
			mutate: func(w *Writ) { w.Scope = []string{"**/*"} },
		},
		{
			name:   "scope is *",
			mutate: func(w *Writ) { w.Scope = []string{"*"} },
		},
		{
			name:   "scope is .",
			mutate: func(w *Writ) { w.Scope = []string{"."} },
		},
		{
			name:   "scope is /",
			mutate: func(w *Writ) { w.Scope = []string{"/"} },
		},
		{
			name:   "scope is ./**",
			mutate: func(w *Writ) { w.Scope = []string{"./**"} },
		},
		{
			name:   "whole-repo scope mixed with a narrow one",
			mutate: func(w *Writ) { w.Scope = []string{"internal/foo/**", "**"} },
		},
		{
			name:   "no criteria",
			mutate: func(w *Writ) { w.Criteria = nil },
		},
		{
			name: "duplicate criterion ids",
			mutate: func(w *Writ) {
				w.Criteria = []Criterion{
					{ID: "c1", Text: "a"},
					{ID: "c1", Text: "b"},
				}
			},
		},
		{
			name: "criterion with empty id",
			mutate: func(w *Writ) {
				w.Criteria = []Criterion{
					{ID: "", Text: "a"},
					{ID: "c2", Text: "b"},
				}
			},
		},
		{
			name: "criterion with whitespace-only id",
			mutate: func(w *Writ) {
				w.Criteria[0].ID = "   "
			},
		},
		{
			name: "criterion with empty text",
			mutate: func(w *Writ) {
				w.Criteria[0].Text = ""
			},
		},
		{
			name: "criterion with whitespace-only text",
			mutate: func(w *Writ) {
				w.Criteria[0].Text = "  \t "
			},
		},
		{
			name:   "empty verify command",
			mutate: func(w *Writ) { w.Verify.Command = "" },
		},
		{
			name: "attestation with bad By",
			mutate: func(w *Writ) {
				met := true
				w.Criteria[0].Met = &met
				w.Criteria[0].Attestation = &Attestation{By: "robot", Note: "n"}
			},
		},
		{
			name: "attestation with nil Met",
			mutate: func(w *Writ) {
				w.Criteria[1].Attestation = &Attestation{By: "agent", Note: "n"}
			},
		},
		{
			name: "attestation with false Met",
			mutate: func(w *Writ) {
				notMet := false
				w.Criteria[0].Met = &notMet
				w.Criteria[0].Attestation = &Attestation{By: "agent", Note: "n"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := validWrit()
			tt.mutate(w)
			if err := w.Validate(); err == nil {
				t.Fatalf("Validate: expected an error, got nil")
			}
		})
	}
}

func TestValidateApprovedAndAttestedIsValid(t *testing.T) {
	if err := approvedWrit().Validate(); err != nil {
		t.Fatalf("Validate on an approved, attested writ: %v", err)
	}
}

func TestValidateNamesEmptyCriterionIdAndText(t *testing.T) {
	w := validWrit()
	w.Criteria = []Criterion{{ID: "  ", Text: ""}}

	err := w.Validate()
	if err == nil {
		t.Fatal("Validate: expected an error for a criterion with no id and no text")
	}
	msg := err.Error()
	for _, want := range []string{
		`criterion 1: id must not be empty`,
		`criterion "  ": text must not be empty`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("Validate error missing %q: %s", want, msg)
		}
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	w := &Writ{}
	err := w.Validate()
	if err == nil {
		t.Fatal("Validate: expected an error for an empty writ")
	}
	// An entirely empty writ trips every rejection: id, intent, base, scope,
	// criteria, and verify.command.
	msg := err.Error()
	for _, want := range []string{"id", "intent", "base", "scope", "criterion", "verify.command"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Validate error missing %q: %s", want, msg)
		}
	}
}
