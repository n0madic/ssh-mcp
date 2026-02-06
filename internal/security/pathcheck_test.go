package security

import (
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
	tests := []string{
		"/home/user/file.txt",
		"/etc/config",
		"relative/path",
		"file.txt",
	}

	for _, p := range tests {
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
	if err := ValidateLocalPath("/tmp/foo/bar", "/tmp/foo"); err != nil {
		t.Errorf("expected path within base to be valid, got: %v", err)
	}
}

func TestValidateLocalPath_EscapesBase(t *testing.T) {
	if err := ValidateLocalPath("/etc/passwd", "/tmp"); err == nil {
		t.Error("expected error for path escaping base directory")
	}
}

func TestValidateLocalPath_NoBaseDir(t *testing.T) {
	validPaths := []string{
		"/any/absolute/path",
		"relative/path",
		"/tmp/../etc/hosts",
	}

	for _, p := range validPaths {
		// Without ".." in raw path, should pass with empty baseDir
		if !strings.Contains(p, "..") {
			if err := ValidateLocalPath(p, ""); err != nil {
				t.Errorf("expected %q to be valid with empty baseDir, got: %v", p, err)
			}
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
