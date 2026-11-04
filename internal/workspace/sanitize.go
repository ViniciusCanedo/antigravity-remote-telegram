package workspace

import (
	"errors"
	"path/filepath"
	"strings"
)

// ResolveSafePath resolves inputPath relative to baseDir and prevents path traversal
// escapes (both via ".." / external absolute paths and symlink bypasses).
// It returns an error containing "Path traversal detected" or
// "Path traversal detected (symlink bypass attempt)".
func ResolveSafePath(inputPath string, baseDir string) (string, error) {
	var normalizedBaseDir string
	if filepath.IsAbs(baseDir) {
		normalizedBaseDir = filepath.Clean(baseDir)
	} else {
		abs, err := filepath.Abs(baseDir)
		if err != nil {
			return "", err
		}
		normalizedBaseDir = abs
	}

	var resolvedPath string
	if filepath.IsAbs(inputPath) {
		resolvedPath = filepath.Clean(inputPath)
	} else {
		resolvedPath = filepath.Clean(filepath.Join(normalizedBaseDir, inputPath))
	}

	relative, err := filepath.Rel(normalizedBaseDir, resolvedPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("Path traversal detected")
	}

	// Resolve symlinks to prevent symlink-based path traversal
	realPath, err1 := filepath.EvalSymlinks(resolvedPath)
	realBaseDir, err2 := filepath.EvalSymlinks(normalizedBaseDir)
	if err1 == nil && err2 == nil {
		relReal, errRel := filepath.Rel(realBaseDir, realPath)
		if errRel != nil || relReal == ".." || strings.HasPrefix(relReal, ".."+string(filepath.Separator)) {
			return "", errors.New("Path traversal detected (symlink bypass attempt)")
		}
		return realPath, nil
	}

	return resolvedPath, nil
}
