package connection

import (
	"bytes"
	"context"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// RemoteInfo holds detected information about the remote host.
type RemoteInfo struct {
	OS    string // "Linux", "Darwin", "FreeBSD", "Windows"
	Arch  string // "x86_64", "aarch64", "arm64", "AMD64"
	Shell string // "/bin/bash", "/bin/zsh", "C:\Windows\system32\cmd.exe"
}

const detectTimeout = 5 * time.Second

// detectRemoteInfo runs lightweight probe commands to detect the remote OS,
// architecture, and shell. Best-effort: failures are logged but never block
// the connection. Returns empty fields on complete failure.
func detectRemoteInfo(ctx context.Context, client *ssh.Client) RemoteInfo {
	ctx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()

	// Try POSIX probe first (Linux/macOS/FreeBSD).
	output, err := runProbeCommand(ctx, client, "uname -s; uname -m; echo $SHELL")
	if err == nil {
		info := parseDetectionOutput(output)
		if info.OS != "" {
			return info
		}
	}

	// Fallback: try Windows detection.
	output, err = runProbeCommand(ctx, client, "echo %OS%; echo %PROCESSOR_ARCHITECTURE%; echo %COMSPEC%")
	if err == nil {
		info := parseWindowsDetectionOutput(output)
		if info.OS != "" {
			return info
		}
	}

	if err != nil {
		log.Printf("Remote info detection failed: %v", err)
	}

	return RemoteInfo{}
}

// runProbeCommand executes a command on the SSH client and returns trimmed output.
func runProbeCommand(ctx context.Context, client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout bytes.Buffer
	session.Stdout = &stdout

	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case err := <-done:
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(stdout.String()), nil
	case <-ctx.Done():
		session.Close()
		return "", ctx.Err()
	}
}

// parseDetectionOutput parses POSIX probe output (uname -s; uname -m; echo $SHELL).
func parseDetectionOutput(output string) RemoteInfo {
	lines := strings.Split(output, "\n")
	var info RemoteInfo

	if len(lines) >= 1 {
		info.OS = strings.TrimSpace(lines[0])
	}
	if len(lines) >= 2 {
		info.Arch = strings.TrimSpace(lines[1])
	}
	if len(lines) >= 3 {
		info.Shell = strings.TrimSpace(lines[2])
	}

	return info
}

// parseWindowsDetectionOutput parses Windows probe output
// (echo %OS%; echo %PROCESSOR_ARCHITECTURE%; echo %COMSPEC%).
// Normalizes "Windows_NT" to "Windows".
func parseWindowsDetectionOutput(output string) RemoteInfo {
	lines := strings.Split(output, "\n")
	var info RemoteInfo

	if len(lines) >= 1 {
		os := strings.TrimSpace(lines[0])
		if os == "Windows_NT" {
			info.OS = "Windows"
		} else if strings.HasPrefix(os, "Windows") {
			info.OS = os
		}
	}

	// Only parse arch/shell if OS was recognized as Windows.
	if info.OS == "" {
		return info
	}

	if len(lines) >= 2 {
		info.Arch = strings.TrimSpace(lines[1])
	}
	if len(lines) >= 3 {
		info.Shell = strings.TrimSpace(lines[2])
	}

	return info
}
