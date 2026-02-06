package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// DirDownloadDeps holds dependencies for the ssh_download_directory tool handler.
type DirDownloadDeps struct {
	Pool         *connection.Pool
	LocalBaseDir string
	RateLimiter  *security.RateLimiter
}

// HandleDownloadDirectory implements the ssh_download_directory tool.
func HandleDownloadDirectory(ctx context.Context, deps *DirDownloadDeps, input SSHDownloadDirectoryInput) (*SSHDownloadDirectoryOutput, error) {
	if err := security.ValidateLocalPath(input.LocalPath, deps.LocalBaseDir); err != nil {
		return nil, fmt.Errorf("invalid local path: %w", err)
	}
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

	fileCount, totalBytes, err := sshclient.DownloadDir(sftpClient, input.RemotePath, input.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("download directory: %w", err)
	}

	return &SSHDownloadDirectoryOutput{
		FilesDownloaded: fileCount,
		BytesRead:       totalBytes,
		Message:         fmt.Sprintf("Downloaded %d files (%d bytes) from %s", fileCount, totalBytes, input.RemotePath),
	}, nil
}
