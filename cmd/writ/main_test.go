package main

import (
	"bytes"
	"testing"
)

func TestRunVersionDefaults(t *testing.T) {
	cmd := newVersionCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runVersion: unexpected error %v", err)
	}

	want := "writ version dev\n"
	if got := out.String(); got != want {
		t.Errorf("runVersion with default commit/date: output = %q, want %q", got, want)
	}
}

func TestVersionStringInjected(t *testing.T) {
	got := versionString("v9.9.9", "abc1234", "2026-08-21")
	want := "writ version v9.9.9 (commit abc1234, built 2026-08-21)"
	if got != want {
		t.Errorf("versionString with injected values: got %q, want %q", got, want)
	}
}

func TestVersionStringDefaults(t *testing.T) {
	got := versionString(defaultVersion, defaultCommit, defaultDate)
	want := "writ version dev"
	if got != want {
		t.Errorf("versionString with default commit/date: got %q, want %q", got, want)
	}
}
