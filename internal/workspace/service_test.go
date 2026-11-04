package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorkspaceService_EnsureBaseDir_CreatesWhenNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "nested", "dir")
	svc := NewWorkspaceService(newDir)

	if err := svc.EnsureBaseDir(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected directory %s to exist", newDir)
	}
}

func TestWorkspaceService_EnsureBaseDir_NoErrorWhenExists(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	if err := svc.EnsureBaseDir(); err != nil {
		t.Fatalf("unexpected error when directory exists: %v", err)
	}
}

func TestWorkspaceService_ScanWorkspaces_SortedOrder(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	_ = os.Mkdir(filepath.Join(tmpDir, "project-b"), 0755)
	_ = os.Mkdir(filepath.Join(tmpDir, "project-a"), 0755)
	_ = os.Mkdir(filepath.Join(tmpDir, "project-c"), 0755)

	result := svc.ScanWorkspaces()
	expected := []string{"project-a", "project-b", "project-c"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestWorkspaceService_ScanWorkspaces_ExcludesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	_ = os.Mkdir(filepath.Join(tmpDir, "project-a"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hello"), 0644)

	result := svc.ScanWorkspaces()
	expected := []string{"project-a"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestWorkspaceService_ScanWorkspaces_ExcludesDotDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	_ = os.Mkdir(filepath.Join(tmpDir, ".hidden"), 0755)
	_ = os.Mkdir(filepath.Join(tmpDir, "visible"), 0755)

	result := svc.ScanWorkspaces()
	expected := []string{"visible"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestWorkspaceService_ScanWorkspaces_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	result := svc.ScanWorkspaces()
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %v", result)
	}
}

func TestWorkspaceService_ValidatePath_ValidRelative(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	result, err := svc.ValidatePath("my-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(tmpDir, "my-project")
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestWorkspaceService_ValidatePath_RejectsTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	_, err := svc.ValidatePath("../etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}

func TestWorkspaceService_ValidatePath_RejectsNestedTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	_, err := svc.ValidatePath("a/../../etc")
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}

func TestWorkspaceService_Exists_TrueForDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	_ = os.Mkdir(filepath.Join(tmpDir, "my-project"), 0755)

	exists, err := svc.Exists("my-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected exists to be true")
	}
}

func TestWorkspaceService_Exists_FalseForNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	exists, err := svc.Exists("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected exists to be false")
	}
}

func TestWorkspaceService_Exists_FalseForFile(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	_ = os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("data"), 0644)

	exists, err := svc.Exists("file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected exists to be false for a regular file")
	}
}

func TestWorkspaceService_GetBaseDir(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	if svc.GetBaseDir() != tmpDir {
		t.Fatalf("expected %q, got %q", tmpDir, svc.GetBaseDir())
	}
}

func TestWorkspaceService_GetWorkspacePath_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	result, err := svc.GetWorkspacePath("proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(tmpDir, "proj")
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestWorkspaceService_GetWorkspacePath_RejectsTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewWorkspaceService(tmpDir)

	_, err := svc.GetWorkspacePath("../outside")
	if err == nil || !strings.Contains(err.Error(), "Path traversal detected") {
		t.Fatalf("expected 'Path traversal detected', got %v", err)
	}
}
