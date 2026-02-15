package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2E runs all E2E tests with a single shared SSH container and MCP server.
func TestE2E(t *testing.T) {
	if os.Getenv("E2E_SKIP") != "" {
		t.Skip("E2E tests skipped (E2E_SKIP is set)")
	}

	env := setupSharedEnv(t)

	t.Run("ConnectExecuteDisconnect", func(t *testing.T) {
		sessionID := sshConnect(t, env)
		t.Logf("Session ID: %s", sessionID)

		// Execute whoami.
		text := callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    "whoami",
		})
		if !strings.Contains(text, "testuser") {
			t.Errorf("expected whoami to return 'testuser', got: %s", text)
		}

		// Execute echo.
		text = callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    "echo hello-e2e",
		})
		if !strings.Contains(text, "hello-e2e") {
			t.Errorf("expected echo output to contain 'hello-e2e', got: %s", text)
		}

		// Disconnect.
		text = callTool(t, env, "ssh_disconnect", map[string]any{
			"session_id": sessionID,
		})
		t.Logf("Disconnect response: %s", text)
	})

	t.Run("FileOperations", func(t *testing.T) {
		sessionID := sshConnect(t, env)

		// Create a local temp file to upload.
		tmpDir := t.TempDir()
		localUpload := filepath.Join(tmpDir, "upload-test.txt")
		uploadContent := "Hello from E2E test!"
		if err := os.WriteFile(localUpload, []byte(uploadContent), 0644); err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}

		// Upload.
		text := callTool(t, env, "ssh_upload_file", map[string]any{
			"session_id":  sessionID,
			"local_path":  localUpload,
			"remote_path": "/home/testuser/uploaded.txt",
		})
		t.Logf("Upload response: %s", text)

		// Verify with cat.
		text = callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    "cat /home/testuser/uploaded.txt",
		})
		if !strings.Contains(text, uploadContent) {
			t.Errorf("expected uploaded content %q, got: %s", uploadContent, text)
		}

		// Download.
		localDownload := filepath.Join(tmpDir, "downloaded.txt")
		text = callTool(t, env, "ssh_download_file", map[string]any{
			"session_id":  sessionID,
			"remote_path": "/home/testuser/uploaded.txt",
			"local_path":  localDownload,
		})
		t.Logf("Download response: %s", text)

		// Compare content.
		downloaded, err := os.ReadFile(localDownload)
		if err != nil {
			t.Fatalf("failed to read downloaded file: %v", err)
		}
		if string(downloaded) != uploadContent {
			t.Errorf("downloaded content mismatch: got %q, want %q", string(downloaded), uploadContent)
		}
	})

	t.Run("DirectoryOperations", func(t *testing.T) {
		sessionID := sshConnect(t, env)

		// Create a local directory structure.
		tmpDir := t.TempDir()
		srcDir := filepath.Join(tmpDir, "src-dir")
		os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
		os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("file1"), 0644)
		os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("file2"), 0644)

		// Upload directory.
		text := callTool(t, env, "ssh_upload_directory", map[string]any{
			"session_id":  sessionID,
			"local_path":  srcDir,
			"remote_path": "/home/testuser/uploaded-dir",
		})
		t.Logf("Upload dir response: %s", text)

		// Verify files exist.
		text = callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    "cat /home/testuser/uploaded-dir/file1.txt",
		})
		if !strings.Contains(text, "file1") {
			t.Errorf("expected 'file1' in uploaded dir, got: %s", text)
		}

		text = callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    "cat /home/testuser/uploaded-dir/subdir/file2.txt",
		})
		if !strings.Contains(text, "file2") {
			t.Errorf("expected 'file2' in uploaded subdir, got: %s", text)
		}

		// Download directory.
		dstDir := filepath.Join(tmpDir, "dst-dir")
		text = callTool(t, env, "ssh_download_directory", map[string]any{
			"session_id":  sessionID,
			"remote_path": "/home/testuser/uploaded-dir",
			"local_path":  dstDir,
		})
		t.Logf("Download dir response: %s", text)

		// Verify downloaded files.
		content1, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
		if err != nil {
			t.Fatalf("failed to read downloaded file1: %v", err)
		}
		if string(content1) != "file1" {
			t.Errorf("downloaded file1 content mismatch: got %q", string(content1))
		}

		content2, err := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
		if err != nil {
			t.Fatalf("failed to read downloaded file2: %v", err)
		}
		if string(content2) != "file2" {
			t.Errorf("downloaded file2 content mismatch: got %q", string(content2))
		}
	})

	t.Run("EditFile", func(t *testing.T) {
		sessionID := sshConnect(t, env)
		remotePath := "/home/testuser/edit-test.txt"

		// Create file with replace mode.
		text := callTool(t, env, "ssh_edit_file", map[string]any{
			"session_id":  sessionID,
			"remote_path": remotePath,
			"mode":        "replace",
			"content":     "original content here",
			"backup":      false,
		})
		t.Logf("Edit (replace) response: %s", text)

		// Verify content.
		text = callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    fmt.Sprintf("cat %s", remotePath),
		})
		if !strings.Contains(text, "original content here") {
			t.Errorf("expected 'original content here', got: %s", text)
		}

		// Patch mode: find and replace.
		text = callTool(t, env, "ssh_edit_file", map[string]any{
			"session_id":  sessionID,
			"remote_path": remotePath,
			"mode":        "patch",
			"old_string":  "original",
			"new_string":  "modified",
			"backup":      false,
		})
		t.Logf("Edit (patch) response: %s", text)

		// Verify patched content.
		text = callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    fmt.Sprintf("cat %s", remotePath),
		})
		if !strings.Contains(text, "modified content here") {
			t.Errorf("expected 'modified content here', got: %s", text)
		}
	})

	t.Run("ListDirectory", func(t *testing.T) {
		sessionID := sshConnect(t, env)

		text := callTool(t, env, "ssh_list_directory", map[string]any{
			"session_id": sessionID,
			"path":       "/home/testuser",
		})
		t.Logf("List directory response: %s", text)

		if !strings.Contains(text, "test-file.txt") {
			t.Errorf("expected 'test-file.txt' in listing, got: %s", text)
		}
		if !strings.Contains(text, "test-dir") {
			t.Errorf("expected 'test-dir' in listing, got: %s", text)
		}
	})

	t.Run("FileStat", func(t *testing.T) {
		sessionID := sshConnect(t, env)

		text := callTool(t, env, "ssh_file_stat", map[string]any{
			"session_id":  sessionID,
			"remote_path": "/home/testuser/test-file.txt",
		})
		t.Logf("File stat response: %s", text)

		if !strings.Contains(text, "test-file.txt") {
			t.Errorf("expected 'test-file.txt' in stat output, got: %s", text)
		}
		if !strings.Contains(text, "file:") {
			t.Errorf("expected 'file:' type in stat output, got: %s", text)
		}
	})

	t.Run("SessionReuse", func(t *testing.T) {
		sessionID1 := sshConnect(t, env)
		sessionID2 := sshConnect(t, env)

		if sessionID1 != sessionID2 {
			t.Errorf("expected same session ID for same host, got %q and %q", sessionID1, sessionID2)
		}
	})

	t.Run("ListSessions", func(t *testing.T) {
		sessionID := sshConnect(t, env)

		text := callTool(t, env, "ssh_list_sessions", map[string]any{})
		t.Logf("List sessions response: %s", text)

		if !strings.Contains(text, sessionID) {
			t.Errorf("expected session ID %q in list, got: %s", sessionID, text)
		}
		if !strings.Contains(text, "connected") {
			t.Errorf("expected 'connected' status in list, got: %s", text)
		}
	})

	t.Run("RemoteInfoDetection", func(t *testing.T) {
		// Connect and verify that remote OS info is detected.
		text := callTool(t, env, "ssh_connect", map[string]any{
			"host": fmt.Sprintf("testuser:password@%s:%d", env.sshHost, env.sshPort),
		})
		t.Logf("ssh_connect response: %s", text)

		// The Docker container runs Ubuntu (Linux), so the connect message should contain "Linux".
		if !strings.Contains(text, "Linux") {
			t.Errorf("expected 'Linux' in connect message, got: %s", text)
		}

		// Verify ssh_list_sessions also shows OS info.
		text = callTool(t, env, "ssh_list_sessions", map[string]any{})
		t.Logf("ssh_list_sessions response: %s", text)

		if !strings.Contains(text, "Linux") {
			t.Errorf("expected 'Linux' in session listing, got: %s", text)
		}
	})

	t.Run("Rename", func(t *testing.T) {
		sessionID := sshConnect(t, env)

		// Create a file to rename.
		callTool(t, env, "ssh_edit_file", map[string]any{
			"session_id":  sessionID,
			"remote_path": "/home/testuser/rename-src.txt",
			"mode":        "replace",
			"content":     "rename test content",
			"backup":      false,
		})

		// Rename.
		text := callTool(t, env, "ssh_rename", map[string]any{
			"session_id": sessionID,
			"old_path":   "/home/testuser/rename-src.txt",
			"new_path":   "/home/testuser/rename-dst.txt",
		})
		t.Logf("Rename response: %s", text)

		// Verify new file exists.
		text = callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    "cat /home/testuser/rename-dst.txt",
		})
		if !strings.Contains(text, "rename test content") {
			t.Errorf("expected 'rename test content' after rename, got: %s", text)
		}

		// Verify old file is gone.
		text = callTool(t, env, "ssh_execute", map[string]any{
			"session_id": sessionID,
			"command":    "test -f /home/testuser/rename-src.txt && echo exists || echo gone",
		})
		if !strings.Contains(text, "gone") {
			t.Errorf("expected old file to be gone after rename, got: %s", text)
		}
	})
}
