package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitize_ResolvesValidPathWithinBaseDir(t *testing.T) {
	baseDir := "/home/user/workspace"
	inputPath := "project/src/index.ts"
	expectedPath := filepath.Join(baseDir, inputPath)

	result, err := ResolveSafePath(inputPath, baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expectedPath {
		t.Fatalf("expected %q, got %q", expectedPath, result)
	}
}

func TestSanitize_ThrowsOnTraverseOutsideBaseDir(t *testing.T) {
	baseDir := "/home/user/workspace"
	inputPath := "../config/secret.json"

	_, err := ResolveSafePath(inputPath, baseDir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected error containing 'Path traversal detected', got %v", err)
	}
}

func TestSanitize_ThrowsOnAbsolutePathOutsideBaseDir(t *testing.T) {
	baseDir := "/home/user/workspace"
	inputPath := "/etc/passwd"

	_, err := ResolveSafePath(inputPath, baseDir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected error containing 'Path traversal detected', got %v", err)
	}
}

func TestSanitize_ResolvesAbsolutePathInsideBaseDir(t *testing.T) {
	baseDir := "/home/user/workspace"
	inputPath := "/home/user/workspace/project/README.md"
	expectedPath := filepath.Join(baseDir, "project/README.md")

	result, err := ResolveSafePath(inputPath, baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expectedPath {
		t.Fatalf("expected %q, got %q", expectedPath, result)
	}
}

func TestSanitizeBugfix_AllowsSubdirectoryPaths(t *testing.T) {
	baseDir := "/home/user/workspace"
	expected := filepath.Join(baseDir, "a/b/c")
	result, err := ResolveSafePath("a/b/c", baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestSanitizeBugfix_AllowsExactBaseDirectory(t *testing.T) {
	baseDir := "/home/user/workspace"
	expected := filepath.Clean(baseDir)
	result, err := ResolveSafePath(".", baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestSanitizeBugfix_BlocksParentTraversalWithDotDotSlash(t *testing.T) {
	baseDir := "/home/user/workspace"
	_, err := ResolveSafePath("../outside", baseDir)
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}

func TestSanitizeBugfix_BlocksExactDotDotPath(t *testing.T) {
	baseDir := "/home/user/workspace"
	_, err := ResolveSafePath("..", baseDir)
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}

func TestSanitizeBugfix_BlocksMultiLevelTraversal(t *testing.T) {
	baseDir := "/home/user/workspace"
	_, err := ResolveSafePath("../../etc/passwd", baseDir)
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}

func TestSanitizeBugfix_BlocksAbsolutePathsOutsideBase(t *testing.T) {
	baseDir := "/home/user/workspace"
	_, err := ResolveSafePath("/etc/passwd", baseDir)
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}

func TestSanitizeBugfix_AllowsAbsolutePathsInsideBase(t *testing.T) {
	baseDir := "/home/user/workspace"
	insidePath := filepath.Join(baseDir, "project", "file.txt")
	result, err := ResolveSafePath(insidePath, baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != insidePath {
		t.Fatalf("expected %q, got %q", insidePath, result)
	}
}

func TestSanitizeBugfix_BlocksTraversalTmpEvil(t *testing.T) {
	baseDir := "/home/user/workspace"
	_, err := ResolveSafePath("/tmp/evil", baseDir)
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}

func TestSanitizeBugfix_HandlesEmbeddedDotDotInsideBase(t *testing.T) {
	baseDir := "/home/user/workspace"
	expected := filepath.Join(baseDir, "b")
	result, err := ResolveSafePath("a/../b", baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestSanitizeBugfix_BlocksEmbeddedDotDotOutsideBase(t *testing.T) {
	baseDir := "/home/user/workspace"
	_, err := ResolveSafePath("a/../../outside", baseDir)
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}

func TestSanitize_BlocksSymlinkEscapes(t *testing.T) {
	baseDir := t.TempDir()
	outsideDir := t.TempDir()

	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(baseDir, "symlink-outside")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Skip("Symlinks not supported on this platform/filesystem")
	}

	_, err := ResolveSafePath("symlink-outside", baseDir)
	if err == nil {
		t.Fatal("expected symlink bypass detection error, got nil")
	}
	if !strings.Contains(err.Error(), "Path traversal detected (symlink bypass attempt)") {
		t.Fatalf("expected 'Path traversal detected (symlink bypass attempt)', got %v", err)
	}
}
