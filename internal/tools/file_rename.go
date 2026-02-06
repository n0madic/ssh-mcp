package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// FileRenameDeps holds dependencies for the ssh_rename tool handler.
type FileRenameDeps struct {
	Pool        *connection.Pool
	RateLimiter *security.RateLimiter
}

// HandleRename implements the ssh_rename tool.
func HandleRename(ctx context.Context, deps *FileRenameDeps, input SSHRenameInput) (*SSHRenameOutput, error) {
	if err := security.ValidatePath(input.OldPath); err != nil {
		return nil, fmt.Errorf("invalid old path: %w", err)
	}
	if err := security.ValidatePath(input.NewPath); err != nil {
		return nil, fmt.Errorf("invalid new path: %w", err)
	}

	conn, err := getConnectionWithRateLimit(ctx, deps.Pool, deps.RateLimiter, input.SessionID)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sshclient.NewSFTPClient(conn.Client)
	if err != nil {
		return nil, err
	}
	defer sftpClient.Close()

	input.OldPath = sshclient.ExpandRemotePath(sftpClient, input.OldPath)
	input.NewPath = sshclient.ExpandRemotePath(sftpClient, input.NewPath)

	if err := sftpClient.Rename(input.OldPath, input.NewPath); err != nil {
		return nil, fmt.Errorf("rename failed: %w", err)
	}

	return &SSHRenameOutput{
		Message: fmt.Sprintf("Renamed %s → %s", input.OldPath, input.NewPath),
	}, nil
}
