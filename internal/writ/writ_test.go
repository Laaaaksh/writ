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
			name:   "empty verify command",
			mutate: func(w *Writ) { w.Verify.Command = "" },
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
