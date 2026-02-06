package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// DirUploadDeps holds dependencies for the ssh_upload_directory tool handler.
type DirUploadDeps struct {
	Pool         *connection.Pool
	LocalBaseDir string
	RateLimiter  *security.RateLimiter
}

// HandleUploadDirectory implements the ssh_upload_directory tool.
func HandleUploadDirectory(ctx context.Context, deps *DirUploadDeps, input SSHUploadDirectoryInput) (*SSHUploadDirectoryOutput, error) {
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

	fileCount, totalBytes, err := sshclient.UploadDir(sftpClient, input.LocalPath, input.RemotePath)
	if err != nil {
		return nil, fmt.Errorf("upload directory: %w", err)
	}

	return &SSHUploadDirectoryOutput{
		FilesUploaded: fileCount,
		BytesWritten:  totalBytes,
		Message:       fmt.Sprintf("Uploaded %d files (%d bytes) to %s", fileCount, totalBytes, input.RemotePath),
	}, nil
}
