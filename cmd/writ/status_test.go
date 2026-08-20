package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunStatusNoWritOpen(t *testing.T) {
	dir := newTestRepo(t)
	withDir(t, dir)

	cmd := newStatusCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, nil)

	ec, ok := err.(exitCodeErr)
	if !ok {
		t.Fatalf("runStatus with no open writ: got err %v, want exitCodeErr", err)
	}
	if ec.code != 2 {
		t.Errorf("runStatus with no open writ: exit code = %d, want 2", ec.code)
	}
	if !strings.Contains(errOut.String(), "no writ is open") {
		t.Errorf("runStatus with no open writ: stderr = %q, want a helpful message", errOut.String())
	}
}
