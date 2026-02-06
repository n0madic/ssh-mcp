package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// DirListDeps holds dependencies for the ssh_list_directory tool handler.
type DirListDeps struct {
	Pool        *connection.Pool
	RateLimiter *security.RateLimiter
}

// HandleListDirectory implements the ssh_list_directory tool.
func HandleListDirectory(ctx context.Context, deps *DirListDeps, input SSHListDirectoryInput) (*SSHListDirectoryOutput, error) {
	if err := security.ValidatePath(input.Path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
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

	input.Path = sshclient.ExpandRemotePath(sftpClient, input.Path)

	entries, err := sshclient.ListDir(sftpClient, input.Path)
	if err != nil {
		return nil, fmt.Errorf("list directory: %w", err)
	}

	return &SSHListDirectoryOutput{
		Entries: entries,
		Count:   len(entries),
	}, nil
}
