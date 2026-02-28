package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// FileInfoDeps holds dependencies for the ssh_file_info tool handler.
type FileInfoDeps struct {
	Pool        *connection.Pool
	RateLimiter *security.RateLimiter
}

// HandleFileInfo implements the ssh_file_info tool.
// For files/symlinks it returns stat info. For directories it also lists contents
// unless stat_only is set to true.
func HandleFileInfo(ctx context.Context, deps *FileInfoDeps, input SSHFileInfoInput) (*SSHFileInfoOutput, error) {
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

	out := &SSHFileInfoOutput{
		Name:      stat.Name(),
		Path:      input.RemotePath,
		Size:      stat.Size(),
		Mode:      stat.Mode().String(),
		IsDir:     stat.IsDir(),
		IsSymlink: stat.Mode()&os.ModeSymlink != 0,
		ModTime:   stat.ModTime().Format("2006-01-02 15:04:05"),
	}

	// For directories, list contents unless stat_only is explicitly true.
	if stat.IsDir() {
		statOnly := false
		if input.StatOnly != nil {
			statOnly = *input.StatOnly
		}
		if !statOnly {
			entries, err := sshclient.ListDir(sftpClient, input.RemotePath)
			if err != nil {
				return nil, fmt.Errorf("list directory: %w", err)
			}
			out.Entries = entries
		}
	}

	return out, nil
}
