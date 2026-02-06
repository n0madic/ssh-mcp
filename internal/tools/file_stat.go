package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// FileStatDeps holds dependencies for the ssh_file_stat tool handler.
type FileStatDeps struct {
	Pool        *connection.Pool
	RateLimiter *security.RateLimiter
}

// HandleFileStat implements the ssh_file_stat tool.
func HandleFileStat(ctx context.Context, deps *FileStatDeps, input SSHFileStatInput) (*SSHFileStatOutput, error) {
	if err := security.ValidatePath(input.RemotePath); err != nil {
		return nil, fmt.Errorf("invalid remote path: %w", err)
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

	input.RemotePath = sshclient.ExpandRemotePath(sftpClient, input.RemotePath)

	// Default to following symlinks; only disable if explicitly set to false.
	followSymlinks := true
	if input.FollowSymlinks != nil {
		followSymlinks = *input.FollowSymlinks
	}

	var stat os.FileInfo
	if followSymlinks {
		stat, err = sftpClient.Stat(input.RemotePath)
	} else {
		stat, err = sftpClient.Lstat(input.RemotePath)
	}
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	isSymlink := stat.Mode()&os.ModeSymlink != 0

	return &SSHFileStatOutput{
		Name:      stat.Name(),
		Path:      input.RemotePath,
		Size:      stat.Size(),
		Mode:      stat.Mode().String(),
		IsDir:     stat.IsDir(),
		IsSymlink: isSymlink,
		ModTime:   stat.ModTime().Format("2006-01-02 15:04:05"),
	}, nil
}
