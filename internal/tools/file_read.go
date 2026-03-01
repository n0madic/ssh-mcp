package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// FileReadDeps holds dependencies for the ssh_read_file tool handler.
type FileReadDeps struct {
	Pool        *connection.Pool
	RateLimiter *security.RateLimiter
	MaxFileSize int64
}

// HandleReadFile implements the ssh_read_file tool.
func HandleReadFile(ctx context.Context, deps *FileReadDeps, input SSHReadFileInput) (*SSHReadFileOutput, error) {
	if err := security.ValidatePath(input.RemotePath); err != nil {
		return nil, fmt.Errorf("invalid remote path: %w", err)
	}

	conn, err := getConnectionWithRateLimit(ctx, deps.Pool, deps.RateLimiter, input.SessionID)
	if err != nil {
		return nil, err
	}

	sc, err := sshclient.NewSFTPClient(conn.Client)
	if err != nil {
		return nil, err
	}
	defer sc.Close()

	input.RemotePath = sshclient.ExpandRemotePath(sc, input.RemotePath)

	// Determine max file size: use input override if set, otherwise server default.
	maxSize := deps.MaxFileSize
	if input.MaxSize > 0 {
		maxSize = input.MaxSize
	}

	// Read file content.
	var data []byte
	if maxSize > 0 {
		data, err = sshclient.ReadFile(sc, input.RemotePath, maxSize)
	} else {
		data, err = sshclient.ReadFile(sc, input.RemotePath)
	}
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Get file size from stat.
	stat, err := sc.Stat(input.RemotePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileSize := stat.Size()

	content := string(data)

	// Split into lines.
	lines := strings.Split(content, "\n")
	// Trim trailing empty line from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	totalLines := len(lines)

	// Handle empty file.
	if totalLines == 0 {
		return &SSHReadFileOutput{
			TotalLines: 0,
			FileSize:   fileSize,
			FromLine:   0,
			ToLine:     0,
			Message:    fmt.Sprintf("%s: 0 lines, %d bytes", input.RemotePath, fileSize),
		}, nil
	}

	// Apply offset (1-based).
	offset := input.Offset
	if offset <= 0 {
		offset = 1
	}

	// Offset beyond EOF.
	if offset > totalLines {
		return &SSHReadFileOutput{
			TotalLines: totalLines,
			FileSize:   fileSize,
			FromLine:   offset,
			ToLine:     offset - 1,
			Message:    fmt.Sprintf("%s: offset %d is beyond end of file (%d lines, %d bytes)", input.RemotePath, offset, totalLines, fileSize),
		}, nil
	}

	// Apply limit.
	startIdx := offset - 1 // convert to 0-based
	endIdx := totalLines
	if input.Limit > 0 && startIdx+input.Limit < endIdx {
		endIdx = startIdx + input.Limit
	}

	// Format with line numbers.
	var b strings.Builder
	for i := startIdx; i < endIdx; i++ {
		fmt.Fprintf(&b, "%6d\t%s\n", i+1, lines[i])
	}

	fromLine := startIdx + 1
	toLine := endIdx

	return &SSHReadFileOutput{
		Content:    b.String(),
		TotalLines: totalLines,
		FileSize:   fileSize,
		FromLine:   fromLine,
		ToLine:     toLine,
		Message:    fmt.Sprintf("%s: showing lines %d-%d of %d (%d bytes)", input.RemotePath, fromLine, toLine, totalLines, fileSize),
	}, nil
}
