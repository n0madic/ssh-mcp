package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath_Valid(t *testing.T) {
	tests := []string{
		"/home/user/file.txt",
		"/etc/config",
		"relative/path",
		"file.txt",
	}

	for _, p := range tests {
		if err := ValidatePath(p); err != nil {
			t.Errorf("expected %q to be valid, got: %v", p, err)
		}
	}
}

func TestValidatePath_Traversal(t *testing.T) {
	tests := []string{
		"../etc/passwd",
		"/home/user/../../etc/passwd",
		"foo/../../bar",
	}

	for _, p := range tests {
		if err := ValidatePath(p); err == nil {
			t.Errorf("expected %q to be rejected for traversal", p)
		}
	}
}

func TestSanitizePath_Absolute(t *testing.T) {
	result, err := SanitizePath("/base", "/absolute/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %s", result)
	}
}

func TestSanitizePath_Relative(t *testing.T) {
	result, err := SanitizePath("/base", "subdir/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/base/subdir/file.txt" {
		t.Errorf("expected /base/subdir/file.txt, got %s", result)
	}
}

func TestSanitizePath_Traversal(t *testing.T) {
	_, err := SanitizePath("/base", "../escape")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestValidateLocalPath_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some real paths
	file := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(file, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	subfile := filepath.Join(subdir, "subfile.txt")
	if err := os.WriteFile(subfile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []string{
		file,
		subfile,
	}

	for _, p := range tests {
		// Test with empty baseDir (should allow if no traversal)
		if err := ValidateLocalPath(p, ""); err != nil {
			t.Errorf("expected %q to be valid with empty baseDir, got: %v", p, err)
		}
	}
}

func TestValidateLocalPath_NullBytes(t *testing.T) {
	path := "/tmp/file\x00.txt"
	if err := ValidateLocalPath(path, ""); err == nil {
		t.Error("expected error for path with null byte")
	}
}

func TestValidateLocalPath_Traversal(t *testing.T) {
	tests := []string{
		"../etc/passwd",
		"/home/user/../../etc/passwd",
		"foo/../../bar",
	}

	for _, p := range tests {
		if err := ValidateLocalPath(p, ""); err == nil {
			t.Errorf("expected %q to be rejected for traversal", p)
		}
	}
}

func TestValidateLocalPath_WithinBase(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	if err := os.Mkdir(baseDir, 0755); err != nil {
		t.Fatalf("failed to create base dir: %v", err)
	}
	targetFile := filepath.Join(baseDir, "file.txt")
	if err := os.WriteFile(targetFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	if err := ValidateLocalPath(targetFile, baseDir); err != nil {
		t.Errorf("expected path within base to be valid, got: %v", err)
	}
}

func TestValidateLocalPath_EscapesBase(t *testing.T) {
	if err := ValidateLocalPath("/etc/passwd", "/tmp"); err == nil {
		t.Error("expected error for path escaping base directory")
	}
}

func TestValidateLocalPath_NoBaseDir(t *testing.T) {
	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(file, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	validPaths := []string{
		file,
	}

	for _, p := range validPaths {
		if err := ValidateLocalPath(p, ""); err != nil {
			t.Errorf("expected %q to be valid with empty baseDir, got: %v", p, err)
		}
	}
}

func TestValidateFilename_Valid(t *testing.T) {
	tests := []string{
		"file.txt",
		"my-file_v2.tar.gz",
		"a",
		strings.Repeat("x", MaxFilenameLength), // exactly 255 chars
		"日本語ファイル.txt",
	}

	for _, name := range tests {
		if err := ValidateFilename(name); err != nil {
			t.Errorf("expected %q to be valid, got: %v", name, err)
		}
	}
}

func TestValidateFilename_TooLong(t *testing.T) {
	name := strings.Repeat("a", MaxFilenameLength+1)
	if err := ValidateFilename(name); err == nil {
		t.Error("expected error for filename exceeding max length")
	}
}

func TestValidateFilename_NullByte(t *testing.T) {
	if err := ValidateFilename("file\x00.txt"); err == nil {
		t.Error("expected error for filename with null byte")
	}
}

func TestValidateFilename_PathSeparator(t *testing.T) {
	tests := []string{
		"dir/file.txt",
		"dir\\file.txt",
	}

	for _, name := range tests {
		if err := ValidateFilename(name); err == nil {
			t.Errorf("expected error for filename with path separator: %q", name)
		}
	}
}

func TestValidateFilename_ControlCharacters(t *testing.T) {
	tests := []string{
		"file\x01.txt",
		"file\x0a.txt", // newline
		"file\x1f.txt",
		"\ttabfile.txt",
	}

	for _, name := range tests {
		if err := ValidateFilename(name); err == nil {
			t.Errorf("expected error for filename with control character: %q", name)
		}
	}
}

func TestValidateFilename_DirectoryTraversal(t *testing.T) {
	if err := ValidateFilename(".."); err == nil {
		t.Error("expected error for '..' filename")
	}
}

func TestValidatePath_LongFilename(t *testing.T) {
	longName := strings.Repeat("x", MaxFilenameLength+1)
	p := "/home/user/" + longName
	if err := ValidatePath(p); err == nil {
		t.Error("expected ValidatePath to reject path with too-long filename")
	}
}

func TestValidatePath_ControlCharInFilename(t *testing.T) {
	p := "/home/user/file\x01.txt"
	if err := ValidatePath(p); err == nil {
		t.Error("expected ValidatePath to reject path with control char in filename")
	}
}

func TestValidateLocalPath_SymlinkTraversal(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create a "safe" directory within tmpDir
	safeDir := filepath.Join(tmpDir, "safe")
	if err := os.Mkdir(safeDir, 0755); err != nil {
		t.Fatalf("failed to create safe dir: %v", err)
	}

	// Create a secret file outside the safe directory (but inside tmpDir for cleanup)
	secretDir := filepath.Join(tmpDir, "secret")
	if err := os.Mkdir(secretDir, 0755); err != nil {
		t.Fatalf("failed to create secret dir: %v", err)
	}
	secretFile := filepath.Join(secretDir, "passwd")
	if err := os.WriteFile(secretFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to create secret file: %v", err)
	}

	// Create a symlink in safeDir pointing to secretFile
	symlink := filepath.Join(safeDir, "link_to_secret")
	if err := os.Symlink(secretFile, symlink); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// ValidateLocalPath should reject accessing the symlink even though the link itself is in safeDir
	// because it resolves to outside safeDir
	if err := ValidateLocalPath(symlink, safeDir); err == nil {
		t.Error("expected ValidateLocalPath to reject symlink traversal")
	}
}

func TestValidateLocalPath_NonExistentPath(t *testing.T) {
	// Test that ValidateLocalPath works for non-existent paths (upload scenario)
	tmpDir := t.TempDir()

	// Test 1: Non-existent file in allowed directory should pass
	newFile := filepath.Join(tmpDir, "newfile.txt")
	if err := ValidateLocalPath(newFile, tmpDir); err != nil {
		t.Errorf("expected ValidateLocalPath to accept non-existent path in base dir: %v", err)
	}

	// Test 2: Non-existent file under symlinked parent should be rejected
	safeDir := filepath.Join(tmpDir, "safe")
	if err := os.Mkdir(safeDir, 0755); err != nil {
		t.Fatalf("failed to create safe dir: %v", err)
	}

	secretDir := filepath.Join(tmpDir, "secret")
	if err := os.Mkdir(secretDir, 0755); err != nil {
		t.Fatalf("failed to create secret dir: %v", err)
	}

	// Create symlink in safe dir pointing to secret dir
	symlinkDir := filepath.Join(safeDir, "link_to_secret")
	if err := os.Symlink(secretDir, symlinkDir); err != nil {
		t.Fatalf("failed to create symlink dir: %v", err)
	}

	// Try to "upload" to a file under the symlinked directory
	newFileUnderSymlink := filepath.Join(symlinkDir, "newfile.txt")
	if err := ValidateLocalPath(newFileUnderSymlink, safeDir); err == nil {
		t.Error("expected ValidateLocalPath to reject non-existent path under symlinked parent that escapes base")
	}
}
