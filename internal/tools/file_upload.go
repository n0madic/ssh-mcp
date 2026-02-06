package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// FileUploadDeps holds dependencies for the ssh_upload_file tool handler.
type FileUploadDeps struct {
	Pool         *connection.Pool
	LocalBaseDir string
	RateLimiter  *security.RateLimiter
}

// HandleUploadFile implements the ssh_upload_file tool.
func HandleUploadFile(ctx context.Context, deps *FileUploadDeps, input SSHUploadFileInput) (*SSHUploadFileOutput, error) {
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

	n, err := sshclient.UploadFile(sftpClient, input.LocalPath, input.RemotePath, nil)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	return &SSHUploadFileOutput{
		BytesWritten: n,
		Message:      fmt.Sprintf("Uploaded %d bytes to %s", n, input.RemotePath),
	}, nil
}
