package security

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// ValidatePath rejects paths with traversal attempts.
func ValidatePath(p string) error {
	// Reject null bytes.
	if strings.ContainsRune(p, 0) {
		return fmt.Errorf("path %q contains null bytes", p)
	}

	// Check for directory traversal in the raw path before cleaning.
	if strings.Contains(p, "..") {
		return fmt.Errorf("path %q contains directory traversal", p)
	}

	return nil
}

// ValidateLocalPath validates a local filesystem path.
// It always rejects null bytes and directory traversal.
// If baseDir is non-empty, it also ensures the resolved path is within baseDir.
func ValidateLocalPath(localPath, baseDir string) error {
	if strings.ContainsRune(localPath, 0) {
		return fmt.Errorf("path contains null bytes")
	}

	if strings.Contains(localPath, "..") {
		return fmt.Errorf("path %q contains directory traversal", localPath)
	}

	if baseDir == "" {
		return nil
	}

	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("cannot resolve path %q: %w", localPath, err)
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("cannot resolve base dir %q: %w", baseDir, err)
	}

	// Ensure the path is within the base directory.
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path %q is outside allowed base directory %q", localPath, baseDir)
	}

	return nil
}

// SanitizePath returns a safe path by joining base with the requested path
// and ensuring the result stays within base.
func SanitizePath(base, requested string) (string, error) {
	if err := ValidatePath(requested); err != nil {
		return "", err
	}

	// If the requested path is absolute, use it directly (after validation).
	if path.IsAbs(requested) {
		return path.Clean(requested), nil
	}

	// Join and clean.
	joined := path.Join(base, requested)
	cleaned := path.Clean(joined)

	// Ensure the result is within base.
	if !strings.HasPrefix(cleaned, path.Clean(base)) {
		return "", fmt.Errorf("path %q escapes base directory %q", requested, base)
	}

	return cleaned, nil
}
