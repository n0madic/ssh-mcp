package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// FileDownloadDeps holds dependencies for the ssh_download_file tool handler.
type FileDownloadDeps struct {
	Pool         *connection.Pool
	LocalBaseDir string
	RateLimiter  *security.RateLimiter
}

// HandleDownloadFile implements the ssh_download_file tool.
func HandleDownloadFile(ctx context.Context, deps *FileDownloadDeps, input SSHDownloadFileInput) (*SSHDownloadFileOutput, error) {
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

	n, err := sshclient.DownloadFile(sftpClient, input.RemotePath, input.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	return &SSHDownloadFileOutput{
		BytesRead: n,
		Message:   fmt.Sprintf("Downloaded %d bytes from %s", n, input.RemotePath),
	}, nil
}
