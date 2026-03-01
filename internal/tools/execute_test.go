package tools

import (
	"strings"
	"testing"
	"time"
)

func TestKillGracePeriod(t *testing.T) {
	if killGracePeriod != 5*time.Second {
		t.Errorf("killGracePeriod = %v, want 5s", killGracePeriod)
	}
}

func TestSSHExecuteOutputText_Timeout(t *testing.T) {
	out := SSHExecuteOutput{
		Stdout:     "partial output",
		Stderr:     "[TIMEOUT] Command timed out after 10s",
		ExitCode:   -1,
		DurationMs: 10000,
	}

	result := out.Text()
	if !strings.Contains(result, "partial output") {
		t.Error("expected Text() to contain stdout")
	}
	if !strings.Contains(result, "[TIMEOUT]") {
		t.Error("expected Text() to contain [TIMEOUT]")
	}
	if !strings.Contains(result, "Exit code: -1") {
		t.Error("expected Text() to contain exit code -1")
	}
}

func TestSSHExecuteOutputText_TimeoutWithExistingStderr(t *testing.T) {
	out := SSHExecuteOutput{
		Stdout:     "some output",
		Stderr:     "some warning\n[TIMEOUT] Command timed out after 5s",
		ExitCode:   -1,
		DurationMs: 5000,
	}

	result := out.Text()
	if !strings.Contains(result, "some warning") {
		t.Error("expected Text() to contain existing stderr")
	}
	if !strings.Contains(result, "[TIMEOUT]") {
		t.Error("expected Text() to contain timeout message")
	}
}

func TestSSHExecuteOutputText_NormalCompletion(t *testing.T) {
	out := SSHExecuteOutput{
		Stdout:     "hello world",
		ExitCode:   0,
		DurationMs: 50,
	}

	result := out.Text()
	if result != "hello world" {
		t.Errorf("Text() = %q, want %q", result, "hello world")
	}
}

func TestSSHExecuteOutputText_NonZeroExit(t *testing.T) {
	out := SSHExecuteOutput{
		Stderr:     "command not found",
		ExitCode:   127,
		DurationMs: 10,
	}

	result := out.Text()
	if !strings.Contains(result, "command not found") {
		t.Error("expected stderr in output")
	}
	if !strings.Contains(result, "Exit code: 127") {
		t.Error("expected exit code in output")
	}
}

func TestSSHExecuteOutputText_EmptyOutput(t *testing.T) {
	out := SSHExecuteOutput{
		ExitCode:   0,
		DurationMs: 100,
	}

	result := out.Text()
	if !strings.Contains(result, "Completed") {
		t.Errorf("expected 'Completed' in empty output, got %q", result)
	}
}
