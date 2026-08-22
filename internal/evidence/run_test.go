package evidence

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Laaaaksh/writ/internal/writ"
)

func TestRun_ExitZeroPasses(t *testing.T) {
	w := &writ.Writ{Verify: writ.VerifySpec{Command: "echo hi"}}
	report, err := Run(w, t.TempDir())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !report.Ran {
		t.Errorf("Ran = false, want true")
	}
	if !report.Passed {
		t.Errorf("Passed = false, want true")
	}
	if report.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", report.ExitCode)
	}
	if !strings.Contains(report.Output, "hi") {
		t.Errorf("Output = %q, want it to contain %q", report.Output, "hi")
	}
}

func TestRun_ExitOneFails(t *testing.T) {
	w := &writ.Writ{Verify: writ.VerifySpec{Command: "exit 1"}}
	report, err := Run(w, t.TempDir())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !report.Ran {
		t.Errorf("Ran = false, want true")
	}
	if report.Passed {
		t.Errorf("Passed = true, want false")
	}
	if report.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", report.ExitCode)
	}
}

func TestRun_EmptyCommand(t *testing.T) {
	w := &writ.Writ{Verify: writ.VerifySpec{Command: ""}}
	report, err := Run(w, t.TempDir())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Ran {
		t.Errorf("Ran = true, want false")
	}
	if report.Passed {
		t.Errorf("Passed = true, want false")
	}
	if report.Summary == "" {
		t.Errorf("Summary is empty, want a message explaining no verification was configured")
	}
}

func TestRun_WhitespaceOnlyCommand(t *testing.T) {
	w := &writ.Writ{Verify: writ.VerifySpec{Command: "   "}}
	report, err := Run(w, t.TempDir())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Ran {
		t.Errorf("Ran = true, want false")
	}
}

func TestRun_Timeout(t *testing.T) {
	t.Setenv("WRIT_VERIFY_TIMEOUT", "100ms")
	w := &writ.Writ{Verify: writ.VerifySpec{Command: "sleep 5"}}

	start := time.Now()
	report, err := Run(w, t.TempDir())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !report.Ran {
		t.Errorf("Ran = false, want true")
	}
	if report.Passed {
		t.Errorf("Passed = true, want false")
	}
	if !strings.Contains(report.Summary, "timed out") {
		t.Errorf("Summary = %q, want it to mention timeout", report.Summary)
	}
	// The timeout knob must be named in the message users actually see,
	// since render.Status prints this Summary verbatim as the EVIDENCE line.
	if !strings.Contains(report.Summary, "WRIT_VERIFY_TIMEOUT") {
		t.Errorf("Summary = %q, want it to name WRIT_VERIFY_TIMEOUT so users can discover the knob", report.Summary)
	}
	if elapsed > 4*time.Second {
		t.Errorf("Run took %v, want well under the 5s sleep — timeout did not cut it short", elapsed)
	}
}

// TestVerifyTimeoutFallback locks down every WRIT_VERIFY_TIMEOUT input path:
// unset and invalid fall back to the default, and so does a non-positive
// duration — which would otherwise kill every verification instantly.
func TestVerifyTimeoutFallback(t *testing.T) {
	tests := []struct {
		name string
		raw  string // empty means leave the variable unset
		want time.Duration
	}{
		{name: "unset uses default", raw: "", want: defaultTimeout},
		{name: "valid duration is honored", raw: "90s", want: 90 * time.Second},
		{name: "invalid string falls back", raw: "soon", want: defaultTimeout},
		{name: "bare number without unit falls back", raw: "30", want: defaultTimeout},
		{name: "zero falls back", raw: "0s", want: defaultTimeout},
		{name: "negative falls back", raw: "-5m", want: defaultTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.raw == "" {
				t.Setenv("WRIT_VERIFY_TIMEOUT", "")
			} else {
				t.Setenv("WRIT_VERIFY_TIMEOUT", tt.raw)
			}
			if got := verifyTimeout(); got != tt.want {
				t.Errorf("verifyTimeout() with WRIT_VERIFY_TIMEOUT=%q = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

// TestRun_TimeoutKillsChildProcess proves that a grandchild spawned by the
// verify command is also killed on timeout, not just the direct sh child.
// It does this by having the child write a marker file after sleeping, then
// asserting the marker never appears — process-group kill is the only thing
// that reliably prevents it, since a plain Process.Kill would only stop the
// immediate "sh -c" process and leave the spawned child running.
func TestRun_TimeoutKillsChildProcess(t *testing.T) {
	t.Setenv("WRIT_VERIFY_TIMEOUT", "150ms")
	dir := t.TempDir()
	marker := dir + "/child-finished"

	// The outer shell backgrounds a child that outlives it; only killing the
	// whole process group stops the child before it writes the marker.
	command := "(sleep 1 && touch " + marker + ") & wait"
	w := &writ.Writ{Verify: writ.VerifySpec{Command: command}}

	report, err := Run(w, dir)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !report.Ran || report.Passed {
		t.Fatalf("report = %+v, want Ran=true Passed=false", report)
	}

	// Give a leaked child, if any survived, time to have written the marker.
	time.Sleep(1200 * time.Millisecond)

	if _, statErr := os.Stat(marker); statErr == nil {
		t.Errorf("marker file exists — child process survived the timeout kill")
	}
}

func TestRun_OutputTruncatedKeepingTail(t *testing.T) {
	// Print enough lines that the combined output exceeds 64KB, ending with
	// a unique tail marker that must survive truncation.
	command := `for i in $(seq 1 3000); do echo "line-$i-0123456789012345678901234567890123456789"; done; echo TAIL_MARKER`
	w := &writ.Writ{Verify: writ.VerifySpec{Command: command}}

	report, err := Run(w, t.TempDir())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(report.Output) > maxOutputBytes+1024 {
		t.Errorf("Output length = %d, want capped near %d", len(report.Output), maxOutputBytes)
	}
	if !strings.Contains(report.Output, "TAIL_MARKER") {
		t.Errorf("Output missing TAIL_MARKER — tail was not retained")
	}
	if strings.Contains(report.Output, "line-1-") {
		t.Errorf("Output still contains the very first line — head was not dropped")
	}
	if !strings.Contains(report.Output, "dropped") {
		t.Errorf("Output = %q, want a truncation marker mentioning dropped bytes", report.Output[:200])
	}
}

func TestSummarize(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		exitCode int
		want     string
	}{
		{
			name:     "go tests pass",
			output:   "=== RUN   TestFoo\n--- PASS: TestFoo (0.00s)\nok  \tgithub.com/example/pkg\t0.002s\n",
			exitCode: 0,
			want:     "go test ok",
		},
		{
			name:     "go tests fail",
			output:   "--- FAIL: TestFoo (0.00s)\n--- FAIL: TestBar (0.00s)\nFAIL\ngithub.com/example/pkg\t0.002s\nFAIL\n",
			exitCode: 1,
			want:     "go test failed, 2 failures",
		},
		{
			name:     "rspec",
			output:   "Finished in 0.5 seconds\n12 examples, 3 failures\n",
			exitCode: 1,
			want:     "12 examples, 3 failures",
		},
		{
			name:     "rspec singular",
			output:   "1 example, 0 failures\n",
			exitCode: 0,
			want:     "1 example, 0 failures",
		},
		{
			name:     "jest",
			output:   "Test Suites: 1 failed, 1 total\nTests:       2 failed, 8 passed, 10 total\n",
			exitCode: 1,
			want:     "10 tests, 2 failures",
		},
		{
			name:     "pytest passed then failed",
			output:   "===== 8 passed, 2 failed in 1.23s =====",
			exitCode: 1,
			want:     "10 tests, 2 failures",
		},
		{
			name:     "pytest failed then passed",
			output:   "===== 2 failed, 8 passed in 1.23s =====",
			exitCode: 1,
			want:     "10 tests, 2 failures",
		},
		{
			name:     "pytest all passed",
			output:   "===== 5 passed in 0.10s =====",
			exitCode: 0,
			want:     "5 tests, 0 failures",
		},
		{
			name:     "unrecognized output falls back to exit code",
			output:   "some custom script output with no known pattern\n",
			exitCode: 3,
			want:     "exit 3",
		},
		{
			name:     "empty output falls back to exit code",
			output:   "",
			exitCode: 0,
			want:     "exit 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarize(tt.output, tt.exitCode)
			if got != tt.want {
				t.Errorf("summarize(...) = %q, want %q", got, tt.want)
			}
		})
	}
}
