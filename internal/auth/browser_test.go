//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func skipWithoutPOSIXShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell not available")
	}
}

func TestWaitForOpenerReportsANonZeroExitInsideTheGraceWindow(t *testing.T) {
	t.Parallel()
	skipWithoutPOSIXShell(t)

	cmd := exec.Command("sh", "-c", "exit 3")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := waitForOpener(cmd, 2*time.Second); err == nil {
		t.Fatal("expected an error for an opener that exited 3")
	}
}

func TestWaitForOpenerReturnsNilWhenTheOpenerIsStillRunning(t *testing.T) {
	t.Parallel()
	skipWithoutPOSIXShell(t)

	cmd := exec.Command("sh", "-c", "sleep 5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	start := time.Now()
	if err := waitForOpener(cmd, 100*time.Millisecond); err != nil {
		t.Fatalf("a still-running opener must not be reported as a failure, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("waited %v, want it to give up at the grace window", elapsed)
	}
}

func TestWaitForOpenerReportsAZeroExitAsSuccess(t *testing.T) {
	t.Parallel()
	skipWithoutPOSIXShell(t)

	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := waitForOpener(cmd, 2*time.Second); err != nil {
		t.Fatalf("expected nil for a clean exit, got %v", err)
	}
}
