package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/sftp"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// FileEditDeps holds dependencies for the ssh_edit_file tool handler.
type FileEditDeps struct {
	Pool        *connection.Pool
	RateLimiter *security.RateLimiter
	MaxFileSize int64
}

// HandleEditFile implements the ssh_edit_file tool.
func HandleEditFile(ctx context.Context, deps *FileEditDeps, input SSHEditFileInput) (*SSHEditFileOutput, error) {
	if err := security.ValidatePath(input.RemotePath); err != nil {
		return nil, fmt.Errorf("invalid remote path: %w", err)
	}

	conn, err := getConnectionWithRateLimit(ctx, deps.Pool, deps.RateLimiter, input.SessionID)
	if err != nil {
		return nil, err
	}

	sc, err := sshclient.NewSFTPClient(conn.Client)
	if err != nil {
		return nil, err
	}
	defer sc.Close()

	input.RemotePath = sshclient.ExpandRemotePath(sc, input.RemotePath)

	mode := input.Mode
	if mode == "" {
		mode = "replace"
	}

	// Default backup to true.
	doBackup := true
	if input.Backup != nil {
		doBackup = *input.Backup
	}

	switch mode {
	case "replace":
		return editReplace(sc, input, doBackup, deps.MaxFileSize)
	case "patch":
		return editPatch(sc, deps, input, doBackup)
	default:
		return nil, fmt.Errorf("unknown edit mode: %q (must be 'replace' or 'patch')", mode)
	}
}

func editReplace(sc *sftp.Client, input SSHEditFileInput, doBackup bool, maxFileSize int64) (*SSHEditFileOutput, error) {
	if doBackup {
		if err := createBackup(sc, input.RemotePath, maxFileSize); err != nil {
			return nil, fmt.Errorf("create backup: %w", err)
		}
	}

	// Preserve existing permissions or default to 0644.
	var perms = defaultPerms(sc, input.RemotePath)

	n, err := sshclient.WriteFile(sc, input.RemotePath, []byte(input.Content), perms)
	if err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	return &SSHEditFileOutput{
		BytesWritten: n,
		Message:      fmt.Sprintf("Replaced content of %s (%d bytes)", input.RemotePath, n),
	}, nil
}

func editPatch(sc *sftp.Client, deps *FileEditDeps, input SSHEditFileInput, doBackup bool) (*SSHEditFileOutput, error) {
	if input.OldString == "" {
		return nil, fmt.Errorf("old_string is required for patch mode")
	}

	data, err := sshclient.ReadFile(sc, input.RemotePath, deps.MaxFileSize)
	if err != nil {
		return nil, fmt.Errorf("read file for patch: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, input.OldString) {
		return nil, fmt.Errorf("old_string not found in %s", input.RemotePath)
	}

	newContent := strings.Replace(content, input.OldString, input.NewString, 1)

	if doBackup {
		if err := createBackup(sc, input.RemotePath, deps.MaxFileSize); err != nil {
			return nil, fmt.Errorf("create backup: %w", err)
		}
	}

	perms := defaultPerms(sc, input.RemotePath)

	n, err := sshclient.WriteFile(sc, input.RemotePath, []byte(newContent), perms)
	if err != nil {
		return nil, fmt.Errorf("write patched file: %w", err)
	}

	return &SSHEditFileOutput{
		BytesWritten: n,
		Message:      fmt.Sprintf("Patched %s (%d bytes)", input.RemotePath, n),
	}, nil
}

func createBackup(sc *sftp.Client, remotePath string, maxFileSize int64) error {
	data, err := sshclient.ReadFile(sc, remotePath, maxFileSize)
	if err != nil {
		// File doesn't exist yet, no backup needed.
		return nil
	}

	perms := defaultPerms(sc, remotePath)
	_, err = sshclient.WriteFile(sc, remotePath+".bak", data, perms)
	return err
}

func defaultPerms(sc *sftp.Client, remotePath string) os.FileMode {
	if stat, err := sc.Stat(remotePath); err == nil {
		return stat.Mode().Perm()
	}
	return 0644
}
