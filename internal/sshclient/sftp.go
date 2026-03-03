package sshclient

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// NewSFTPClient creates a new SFTP client from an SSH client.
func NewSFTPClient(client *ssh.Client) (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}
	return sftpClient, nil
}

// ExpandRemotePath expands ~ and resolves relative paths on the remote server using RealPath.
func ExpandRemotePath(sftpClient *sftp.Client, remotePath string) string {
	// RealPath canonicalizes the path on the server, handling ~, .., and relative paths.
	if realPath, err := sftpClient.RealPath(remotePath); err == nil {
		return realPath
	}
	// Fallback if RealPath fails (shouldn't happen for valid paths).
	return remotePath
}

// UploadFile uploads a local file to a remote path, preserving permissions.
func UploadFile(sftpClient *sftp.Client, localPath, remotePath string, perms *fs.FileMode) (int64, error) {
	localFile, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("open local file: %w", err)
	}
	defer localFile.Close()

	// Determine permissions to apply.
	var mode fs.FileMode = 0644
	if perms != nil {
		mode = *perms
	} else {
		if stat, err := localFile.Stat(); err == nil {
			mode = stat.Mode().Perm()
		}
	}

	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return 0, fmt.Errorf("create remote file: %w", err)
	}
	defer remoteFile.Close()

	n, err := io.Copy(remoteFile, localFile)
	if err != nil {
		return 0, fmt.Errorf("copy to remote: %w", err)
	}

	if err := sftpClient.Chmod(remotePath, mode); err != nil {
		return n, fmt.Errorf("chmod remote file: %w", err)
	}

	return n, nil
}

// DownloadFile downloads a remote file to a local path, preserving permissions.
func DownloadFile(sftpClient *sftp.Client, remotePath, localPath string) (int64, error) {
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return 0, fmt.Errorf("open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Get remote file permissions.
	remoteStat, err := sftpClient.Stat(remotePath)
	if err != nil {
		return 0, fmt.Errorf("stat remote file: %w", err)
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("create local file: %w", err)
	}
	defer localFile.Close()

	n, err := io.Copy(localFile, remoteFile)
	if err != nil {
		return 0, fmt.Errorf("copy to local: %w", err)
	}

	// Apply remote file permissions to local file.
	if err := os.Chmod(localPath, remoteStat.Mode().Perm()); err != nil {
		return n, fmt.Errorf("chmod local file: %w", err)
	}

	return n, nil
}

// UploadDir recursively uploads a local directory to a remote path, preserving permissions.
func UploadDir(sftpClient *sftp.Client, localDir, remoteDir string) (int, int64, error) {
	fileCount := 0
	var totalBytes int64

	err := filepath.Walk(localDir, func(localPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to prevent reading files outside the allowed directory.
		// filepath.Walk uses os.Lstat, so info correctly reports symlinks.
		if info.Mode()&os.ModeSymlink != 0 {
			log.Printf("upload: skipping symlink %s", localPath)
			return nil
		}

		relPath, err := filepath.Rel(localDir, localPath)
		if err != nil {
			return err
		}
		remotePath := path.Join(remoteDir, filepath.ToSlash(relPath))

		if info.IsDir() {
			if err := sftpClient.MkdirAll(remotePath); err != nil {
				return fmt.Errorf("mkdir %s: %w", remotePath, err)
			}
			if err := sftpClient.Chmod(remotePath, info.Mode().Perm()); err != nil {
				// Non-fatal: some servers may not support chmod on dirs.
				_ = err
			}
			return nil
		}

		perms := info.Mode().Perm()
		n, err := UploadFile(sftpClient, localPath, remotePath, &perms)
		if err != nil {
			return fmt.Errorf("upload %s: %w", localPath, err)
		}
		fileCount++
		totalBytes += n
		return nil
	})

	return fileCount, totalBytes, err
}

// DownloadDir recursively downloads a remote directory to a local path, preserving permissions.
func DownloadDir(sftpClient *sftp.Client, remoteDir, localDir string) (int, int64, error) {
	fileCount := 0
	var totalBytes int64

	err := walkRemoteDir(sftpClient, remoteDir, func(remotePath string, info os.FileInfo) error {
		relPath, err := filepath.Rel(remoteDir, remotePath)
		if err != nil {
			return err
		}
		localPath := filepath.Join(localDir, relPath)

		if info.IsDir() {
			if err := os.MkdirAll(localPath, info.Mode().Perm()); err != nil {
				return fmt.Errorf("mkdir %s: %w", localPath, err)
			}
			return nil
		}

		// Ensure parent directory exists.
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("mkdir parent %s: %w", filepath.Dir(localPath), err)
		}

		n, err := DownloadFile(sftpClient, remotePath, localPath)
		if err != nil {
			return fmt.Errorf("download %s: %w", remotePath, err)
		}
		fileCount++
		totalBytes += n
		return nil
	})

	return fileCount, totalBytes, err
}

// ReadFile reads a remote file and returns its contents.
// If maxSize > 0, the file size is checked first and reading is capped with io.LimitReader.
func ReadFile(sftpClient *sftp.Client, remotePath string, maxSize ...int64) ([]byte, error) {
	var limit int64
	if len(maxSize) > 0 {
		limit = maxSize[0]
	}

	file, err := sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("open remote file: %w", err)
	}
	defer file.Close()

	if limit > 0 {
		stat, err := sftpClient.Stat(remotePath)
		if err != nil {
			return nil, fmt.Errorf("stat remote file: %w", err)
		}
		if stat.Size() > limit {
			return nil, fmt.Errorf("file %s is %d bytes, exceeds maximum allowed size of %d bytes",
				remotePath, stat.Size(), limit)
		}
		data, err := io.ReadAll(io.LimitReader(file, limit+1))
		if err != nil {
			return nil, fmt.Errorf("read remote file: %w", err)
		}
		return data, nil
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read remote file: %w", err)
	}

	return data, nil
}

// WriteFile writes data to a remote file with given permissions.
// Parent directories are created automatically if they don't exist.
func WriteFile(sftpClient *sftp.Client, remotePath string, data []byte, perms fs.FileMode) (int64, error) {
	if dir := path.Dir(remotePath); dir != "." && dir != "/" {
		if err := sftpClient.MkdirAll(dir); err != nil {
			return 0, fmt.Errorf("create parent directories: %w", err)
		}
	}
	file, err := sftpClient.Create(remotePath)
	if err != nil {
		return 0, fmt.Errorf("create remote file: %w", err)
	}
	defer file.Close()

	n, err := file.Write(data)
	if err != nil {
		return 0, fmt.Errorf("write remote file: %w", err)
	}

	if err := sftpClient.Chmod(remotePath, perms); err != nil {
		return int64(n), fmt.Errorf("chmod remote file: %w", err)
	}

	return int64(n), nil
}

func walkRemoteDir(sftpClient *sftp.Client, dirPath string, fn func(string, os.FileInfo) error) error {
	// Use Walker for efficient directory traversal.
	walker := sftpClient.Walk(dirPath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return fmt.Errorf("walk error: %w", err)
		}
		if err := fn(walker.Path(), walker.Stat()); err != nil {
			return err
		}
	}
	return nil
}
