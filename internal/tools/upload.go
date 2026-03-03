package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// UploadDeps holds dependencies for the ssh_upload tool handler.
type UploadDeps struct {
	Pool         *connection.Pool
	LocalBaseDir string
	RateLimiter  *security.RateLimiter
}

// HandleUpload implements the ssh_upload tool.
// It auto-detects whether local_path is a file or directory and delegates accordingly.
func HandleUpload(ctx context.Context, deps *UploadDeps, input SSHUploadInput) (*SSHUploadOutput, error) {
	if err := security.ValidateLocalPath(input.LocalPath, deps.LocalBaseDir); err != nil {
		return nil, fmt.Errorf("invalid local path: %w", err)
	}
	if err := security.ValidatePath(input.RemotePath); err != nil {
		return nil, fmt.Errorf("invalid remote path: %w", err)
	}

	info, err := os.Stat(input.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("stat local path: %w", err)
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

	if info.IsDir() {
		fileCount, totalBytes, err := sshclient.UploadDir(sftpClient, input.LocalPath, input.RemotePath)
		if err != nil {
			return nil, fmt.Errorf("upload directory: %w", err)
		}
		return &SSHUploadOutput{
			FilesUploaded: fileCount,
			BytesWritten:  totalBytes,
			Message:       fmt.Sprintf("Uploaded %d files (%d bytes) to %s", fileCount, totalBytes, input.RemotePath),
		}, nil
	}

	n, err := sshclient.UploadFile(sftpClient, input.LocalPath, input.RemotePath, nil)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	return &SSHUploadOutput{
		FilesUploaded: 1,
		BytesWritten:  n,
		Message:       fmt.Sprintf("Uploaded %d bytes to %s", n, input.RemotePath),
	}, nil
}
