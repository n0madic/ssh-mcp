package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// DownloadDeps holds dependencies for the ssh_download tool handler.
type DownloadDeps struct {
	Pool         *connection.Pool
	LocalBaseDir string
	RateLimiter  *security.RateLimiter
}

// HandleDownload implements the ssh_download tool.
// It auto-detects whether remote_path is a file or directory and delegates accordingly.
func HandleDownload(ctx context.Context, deps *DownloadDeps, input SSHDownloadInput) (*SSHDownloadOutput, error) {
	if err := security.ValidateLocalPath(input.LocalPath, deps.LocalBaseDir); err != nil {
		return nil, fmt.Errorf("invalid local path: %w", err)
	}
	if err := security.ValidatePath(input.RemotePath); err != nil {
		return nil, fmt.Errorf("invalid remote path: %w", err)
	}

	_, client, err := getConnectionWithRateLimit(ctx, deps.Pool, deps.RateLimiter, input.SessionID)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sshclient.NewSFTPClient(client)
	if err != nil {
		return nil, err
	}
	defer sftpClient.Close()

	input.RemotePath = sshclient.ExpandRemotePath(sftpClient, input.RemotePath)

	stat, err := sftpClient.Stat(input.RemotePath)
	if err != nil {
		return nil, fmt.Errorf("stat remote path: %w", err)
	}

	if stat.IsDir() {
		fileCount, totalBytes, err := sshclient.DownloadDir(sftpClient, input.RemotePath, input.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("download directory: %w", err)
		}
		return &SSHDownloadOutput{
			FilesDownloaded: fileCount,
			BytesRead:       totalBytes,
			Message:         fmt.Sprintf("Downloaded %d files (%d bytes) from %s", fileCount, totalBytes, input.RemotePath),
		}, nil
	}

	n, err := sshclient.DownloadFile(sftpClient, input.RemotePath, input.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	return &SSHDownloadOutput{
		FilesDownloaded: 1,
		BytesRead:       n,
		Message:         fmt.Sprintf("Downloaded %d bytes from %s", n, input.RemotePath),
	}, nil
}
