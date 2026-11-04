package cdp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_Security_DoesNotInvokeShell(t *testing.T) {
	// 1. Inspect source file of client.go
	sourcePath := filepath.Join(".", "client.go")
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("failed to read client.go: %v", err)
	}
	source := string(sourceBytes)

	// Prohibit shell execution wrappers
	forbiddenPatterns := []string{
		`"sh", "-c"`,
		`"bash", "-c"`,
		`"/bin/sh"`,
		`"/bin/bash"`,
		`"cmd", "/c"`,
		`"cmd.exe"`,
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(source, pattern) {
			t.Fatalf("security violation: client.go contains shell execution pattern %s", pattern)
		}
	}

	// Ensure direct exec.Command is used
	if !strings.Contains(source, "exec.Command(command, args...)") {
		t.Fatalf("expected client.go to use exec.Command(command, args...) directly")
	}

	// 2. Functional check of defaultRunCommand
	// Running a harmless command like "echo" with args
	err = defaultRunCommand("echo", []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error running direct command: %v", err)
	}
}
