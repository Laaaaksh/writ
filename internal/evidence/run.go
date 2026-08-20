// Package evidence runs a writ's verification command and records the result.
package evidence

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Laaaaksh/writ/internal/writ"
)

// Report is the outcome of running a writ's verification command.
type Report struct {
	Ran      bool
	Passed   bool
	Command  string
	ExitCode int
	Output   string
	Summary  string
}

// maxOutputBytes is the cap on retained combined stdout+stderr, keeping the tail.
const maxOutputBytes = 64 * 1024

// defaultTimeout is used when WRIT_VERIFY_TIMEOUT is unset or invalid.
const defaultTimeout = 10 * time.Minute

// Run executes w's verification command in repoDir and records the outcome.
//
// The returned error indicates the harness failed to run the command at all
// (e.g. it could not start); a non-zero exit from the command itself is a
// successful run reported via Passed/ExitCode, not an error.
func Run(w *writ.Writ, repoDir string) (*Report, error) {
	command := w.Verify.Command
	if strings.TrimSpace(command) == "" {
		return &Report{
			Ran:     false,
			Passed:  false,
			Command: command,
			Summary: "no verification command configured",
		}, nil
	}

	timeout := verifyTimeout()

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = repoDir
	// Put the command in its own process group so a timeout can kill it and
	// every child it spawned, not just the direct sh process.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var buf lockedTailBuffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting verify command: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case waitErr := <-done:
		exitCode := 0
		if waitErr != nil {
			exitErr, ok := waitErr.(*exec.ExitError)
			if !ok {
				return nil, fmt.Errorf("running verify command: %w", waitErr)
			}
			exitCode = exitErr.ExitCode()
		}

		output := buf.String()
		return &Report{
			Ran:      true,
			Passed:   exitCode == 0,
			Command:  command,
			ExitCode: exitCode,
			Output:   output,
			Summary:  summarize(output, exitCode),
		}, nil

	case <-time.After(timeout):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done // reap the process to avoid a zombie
		return &Report{
			Ran:      true,
			Passed:   false,
			Command:  command,
			ExitCode: -1,
			Output:   buf.String(),
			Summary:  fmt.Sprintf("timed out after %s", timeout),
		}, nil
	}
}

// verifyTimeout reads WRIT_VERIFY_TIMEOUT as a Go duration string, falling
// back to defaultTimeout if unset or invalid.
func verifyTimeout() time.Duration {
	raw := os.Getenv("WRIT_VERIFY_TIMEOUT")
	if raw == "" {
		return defaultTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return defaultTimeout
	}
	return d
}

// lockedTailBuffer accumulates output, keeping only the tail once it exceeds
// maxOutputBytes. exec.Cmd copies stdout and stderr from separate goroutines
// when they share a writer, so writes must be synchronized.
type lockedTailBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	totalLen int
}

func (b *lockedTailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.buf.Write(p)
	b.totalLen += n
	if b.buf.Len() > maxOutputBytes*2 {
		b.truncateLocked()
	}
	return n, err
}

func (b *lockedTailBuffer) truncateLocked() {
	data := b.buf.Bytes()
	if len(data) <= maxOutputBytes {
		return
	}
	tail := make([]byte, maxOutputBytes)
	copy(tail, data[len(data)-maxOutputBytes:])
	b.buf.Reset()
	b.buf.Write(tail)
}

// String returns the retained output, prefixed with a truncation marker if
// the tail had to be cut.
func (b *lockedTailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.truncateLocked()
	data := b.buf.String()
	if b.totalLen > maxOutputBytes {
		dropped := b.totalLen - len(data)
		marker := fmt.Sprintf("[... %d bytes dropped, showing tail ...]\n", dropped)
		return marker + data
	}
	return data
}

var (
	goFail        = regexp.MustCompile(`(?m)^---\s+FAIL`)
	goOK          = regexp.MustCompile(`(?m)^ok\s+\S+`)
	goPackageFAIL = regexp.MustCompile(`(?m)^FAIL(\s|$)`)
	rspecRe       = regexp.MustCompile(`(\d+)\s+(examples?),\s+(\d+)\s+(failures?)`)
	jestSuiteRe   = regexp.MustCompile(`Tests:\s+(?:(\d+)\s+failed,\s+)?(?:(\d+)\s+skipped,\s+)?(\d+)\s+passed,\s+(\d+)\s+total`)
	pytestPassed  = regexp.MustCompile(`(\d+)\s+passed`)
	pytestFailed  = regexp.MustCompile(`(\d+)\s+failed`)
)

// summarize extracts a one-line human summary from the tail of output for a
// handful of common test runners. It never affects Passed, which is derived
// solely from the exit code; if nothing recognisable is found it falls back
// to reporting the exit code.
func summarize(output string, exitCode int) string {
	tail := output
	if len(tail) > 4096 {
		tail = tail[len(tail)-4096:]
	}

	if m := rspecRe.FindStringSubmatch(tail); m != nil {
		return fmt.Sprintf("%s %s, %s %s", m[1], m[2], m[3], m[4])
	}

	if m := jestSuiteRe.FindStringSubmatch(tail); m != nil {
		failed := m[1]
		if failed == "" {
			failed = "0"
		}
		total := m[4]
		return fmt.Sprintf("%s tests, %s failures", total, failed)
	}

	passedMatch := pytestPassed.FindStringSubmatch(tail)
	failedMatch := pytestFailed.FindStringSubmatch(tail)
	if passedMatch != nil || failedMatch != nil {
		passed, failed := "0", "0"
		if passedMatch != nil {
			passed = passedMatch[1]
		}
		if failedMatch != nil {
			failed = failedMatch[1]
		}
		if total, err := addCounts(passed, failed); err == nil {
			return fmt.Sprintf("%s tests, %s failures", total, failed)
		}
	}

	if goPackageFAIL.MatchString(tail) || goFail.MatchString(tail) {
		failures := len(goFail.FindAllString(tail, -1))
		if failures == 0 {
			failures = 1
		}
		return fmt.Sprintf("go test failed, %d failures", failures)
	}
	if goOK.MatchString(tail) {
		return "go test ok"
	}

	return fmt.Sprintf("exit %d", exitCode)
}

func addCounts(a, b string) (string, error) {
	an, err := strconv.Atoi(a)
	if err != nil {
		return "", err
	}
	bn, err := strconv.Atoi(b)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(an + bn), nil
}
