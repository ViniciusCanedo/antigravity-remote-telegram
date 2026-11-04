package workspace

import (
	"log"
	"os"
	"sort"
	"strings"
)

// WorkspaceService manages project directories under baseDir.
type WorkspaceService struct {
	baseDir string
}

// NewWorkspaceService creates a new WorkspaceService for the given base directory.
func NewWorkspaceService(baseDir string) *WorkspaceService {
	return &WorkspaceService{baseDir: baseDir}
}

// EnsureBaseDir creates the base directory if it does not exist.
func (s *WorkspaceService) EnsureBaseDir() error {
	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return os.MkdirAll(s.baseDir, 0755)
	}
	return nil
}

// ScanWorkspaces returns the sorted names of non-hidden subdirectories in baseDir.
func (s *WorkspaceService) ScanWorkspaces() []string {
	_ = s.EnsureBaseDir()
	log.Printf("[DEBUG] Scanning workspaces in: %s", s.baseDir)

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		log.Printf("[WorkspaceService] Failed to scan workspaces in %s: %v", s.baseDir, err)
		return []string{}
	}

	var workspaces []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			workspaces = append(workspaces, entry.Name())
		}
	}

	if workspaces == nil {
		workspaces = []string{}
	}
	sort.Strings(workspaces)
	return workspaces
}

// ValidatePath validates relativePath against baseDir and returns a safe absolute path.
func (s *WorkspaceService) ValidatePath(relativePath string) (string, error) {
	return ResolveSafePath(relativePath, s.baseDir)
}

// GetBaseDir returns the base directory path.
func (s *WorkspaceService) GetBaseDir() string {
	return s.baseDir
}

// GetWorkspacePath returns the absolute path of the specified workspace.
func (s *WorkspaceService) GetWorkspacePath(workspaceName string) (string, error) {
	return s.ValidatePath(workspaceName)
}

// Exists checks if the specified workspace exists and is a directory.
func (s *WorkspaceService) Exists(workspaceName string) (bool, error) {
	fullPath, err := s.ValidatePath(workspaceName)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}
