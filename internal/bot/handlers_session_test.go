package bot

import (
	"testing"
)

func TestJoinCommandHandler_ResolveProjectPath(t *testing.T) {
	resolver := &mockWorkspacePathResolver{
		getWorkspacePathFn: func(projectName string) (string, error) {
			if projectName == "my-project" {
				return "/home/user/code/my-project", nil
			}
			return "", nil
		},
	}

	handler := NewJoinCommandHandlerWithResolver(nil, nil, nil, nil, resolver)
	result, err := handler.ResolveProjectPath("my-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/home/user/code/my-project" {
		t.Fatalf("expected '/home/user/code/my-project', got %q", result)
	}
}
