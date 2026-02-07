package security

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// MaxFilenameLength is the maximum allowed filename length (standard filesystem limit).
const MaxFilenameLength = 255

// ValidateFilename rejects filenames that are too long, contain null bytes,
// path separators, directory traversal, or control characters.
func ValidateFilename(name string) error {
	if utf8.RuneCountInString(name) > MaxFilenameLength {
		return fmt.Errorf("filename is too long (%d characters, max %d)", utf8.RuneCountInString(name), MaxFilenameLength)
	}

	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("filename contains null bytes")
	}

	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("filename contains path separator")
	}

	if name == ".." || strings.Contains(name, "..") {
		return fmt.Errorf("filename contains directory traversal")
	}

	for _, r := range name {
		if r < 0x20 {
			return fmt.Errorf("filename contains control character (0x%02x)", r)
		}
	}

	return nil
}

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

	// Validate the filename component.
	base := path.Base(p)
	if base != "." && base != "/" {
		if err := ValidateFilename(base); err != nil {
			return fmt.Errorf("invalid filename in path: %w", err)
		}
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
	// We must resolve symlinks to prevent traversal attacks (e.g. ln -s / /tmp/jail/root).
	// If the path doesn't exist yet (upload scenario), validate the parent directory.
	finalPath := absPath
	if _, err := os.Lstat(absPath); err == nil {
		// Path exists, resolve symlinks
		finalPath, err = filepath.EvalSymlinks(absPath)
		if err != nil {
			return fmt.Errorf("failed to resolve symlinks for %q: %w", localPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check path %q: %w", localPath, err)
	} else {
		// Path doesn't exist - validate parent instead
		parent := filepath.Dir(absPath)
		if parent != "." && parent != string(filepath.Separator) {
			resolvedParent, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return fmt.Errorf("failed to resolve symlinks for parent of %q: %w", localPath, err)
			}
			finalPath = filepath.Join(resolvedParent, filepath.Base(absPath))
		}
	}

	if !strings.HasPrefix(finalPath, absBase+string(filepath.Separator)) && finalPath != absBase {
		// Fallback: in some cases (like macOS /var -> /private/var), the base dir might also be a symlink.
		// Let's resolve the base dir too.
		finalBase, err := filepath.EvalSymlinks(absBase)
		if err != nil {
			return fmt.Errorf("failed to resolve symlinks for base %q: %w", baseDir, err)
		}
		if !strings.HasPrefix(finalPath, finalBase+string(filepath.Separator)) && finalPath != finalBase {
			return fmt.Errorf("path %q (resolves to %q) is outside allowed base directory %q (resolves to %q)", localPath, finalPath, baseDir, finalBase)
		}
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
