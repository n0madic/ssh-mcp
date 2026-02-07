package tools

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/acarl005/stripansi"

	"golang.org/x/crypto/ssh"

	"github.com/n0madic/ssh-mcp/internal/config"
	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
)

// ExecuteDeps holds dependencies for the ssh_execute tool handler.
type ExecuteDeps struct {
	Pool        *connection.Pool
	Filter      *security.Filter
	RateLimiter *security.RateLimiter
	Config      *config.SSHConfig
}

// HandleExecute implements the ssh_execute tool.
func HandleExecute(ctx context.Context, deps *ExecuteDeps, input SSHExecuteInput) (*SSHExecuteOutput, error) {
	sessionID := connection.SessionID(input.SessionID)

	// Get connection (with auto-reconnect).
	conn, err := deps.Pool.GetConnection(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}

	// Rate limit check.
	if err := deps.RateLimiter.Allow(conn.Host); err != nil {
		return nil, err
	}

	// Build the command.
	cmd := input.Command

	// Command filter check on the original command (before cd/sudo prepend).
	// This ensures the allowlist matches what the user actually requested.
	if err := deps.Filter.AllowCommand(cmd); err != nil {
		return nil, err
	}

	// Prepend working directory if specified.
	if input.WorkingDir != "" {
		cmd = fmt.Sprintf("cd %s && %s", shellQuote(input.WorkingDir), cmd)
	}

	// Handle sudo.
	if input.Sudo {
		if !deps.Config.AllowSudo {
			return nil, fmt.Errorf("sudo is disabled; start server with --enable-sudo to allow")
		}
		// Use sh -c to support shell builtins (like cd) inside sudo.
		cmd = fmt.Sprintf("sudo -S sh -c %s", shellQuote(cmd))
	}

	// Set timeout.
	timeout := deps.Config.CommandTimeout
	if input.Timeout > 0 {
		timeout = time.Duration(input.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create SSH session.
	conn.IncrementCommandCount()
	session, err := conn.Client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	// Set up stdin for sudo password.
	if input.Sudo && input.SudoPassword != "" {
		session.Stdin = strings.NewReader(input.SudoPassword + "\n")
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	start := time.Now()

	// Run the command with context timeout.
	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	var exitCode int
	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return nil, fmt.Errorf("command timed out after %s", timeout)
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(interface{ ExitStatus() int }); ok {
				exitCode = exitErr.ExitStatus()
			} else {
				return nil, fmt.Errorf("execute command: %w", err)
			}
		}
	}

	duration := time.Since(start)

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Strip ANSI escape codes if enabled.
	if deps.Config.StripANSI {
		stdoutStr = stripansi.Strip(stdoutStr)
		stderrStr = stripansi.Strip(stderrStr)
	}

	return &SSHExecuteOutput{
		Stdout:     stdoutStr,
		Stderr:     stderrStr,
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
	}, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
