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
