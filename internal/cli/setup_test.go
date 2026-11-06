package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"antigravity-telegram-remote/internal/config"
)

func TestIsNonEmpty(t *testing.T) {
	if IsNonEmpty("") {
		t.Error("expected false for empty string")
	}
	if IsNonEmpty("   \t\n") {
		t.Error("expected false for whitespace")
	}
	if !IsNonEmpty("hello") {
		t.Error("expected true for non-empty string")
	}
}

func TestIsNumericString(t *testing.T) {
	if !IsNumericString("123456") {
		t.Error("expected true for 123456")
	}
	if !IsNumericString("  789012  ") {
		t.Error("expected true for padded 789012")
	}
	if IsNumericString("123a") {
		t.Error("expected false for 123a")
	}
	if IsNumericString("") {
		t.Error("expected false for empty string")
	}
}

func TestParseAllowedUserIDs(t *testing.T) {
	ids := ParseAllowedUserIDs(" 12345, 67890 , , 11223 ")
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != "12345" || ids[1] != "67890" || ids[2] != "11223" {
		t.Errorf("unexpected IDs: %v", ids)
	}
}

func TestValidateAllowedUserIDs(t *testing.T) {
	emptyErr := ValidateAllowedUserIDs("")
	if emptyErr != "Please enter at least one user ID." {
		t.Errorf("expected empty error message, got: %q", emptyErr)
	}

	invalidErr := ValidateAllowedUserIDs("12345, abc")
	expectedInvalid := "Invalid user ID: \"abc\" — must be numeric."
	if invalidErr != expectedInvalid {
		t.Errorf("expected %q, got %q", expectedInvalid, invalidErr)
	}

	validErr := ValidateAllowedUserIDs("123456, 789012")
	if validErr != "" {
		t.Errorf("expected valid IDs to return empty error, got: %q", validErr)
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	if ExpandTilde("~") != home {
		t.Errorf("expected '~' to expand to %s, got %s", home, ExpandTilde("~"))
	}
	if ExpandTilde("~/projects") != filepath.Join(home, "projects") {
		t.Errorf("expected '~/projects' to expand to %s, got %s", filepath.Join(home, "projects"), ExpandTilde("~/projects"))
	}
	if ExpandTilde("/var/log") != "/var/log" {
		t.Errorf("expected absolute path to remain unchanged")
	}
}

func TestDirectoryCompleter(t *testing.T) {
	tempDir := t.TempDir()
	subDir1 := filepath.Join(tempDir, "alpha")
	subDir2 := filepath.Join(tempDir, "alpine")
	subFile := filepath.Join(tempDir, "alphabet.txt")
	_ = os.MkdirAll(subDir1, 0755)
	_ = os.MkdirAll(subDir2, 0755)
	_ = os.WriteFile(subFile, []byte("test"), 0644)

	// Incomplete prefix "alpi" matches only alpine
	candidates, line := DirectoryCompleter(filepath.Join(tempDir, "alpi"))
	if line != filepath.Join(tempDir, "alpi") {
		t.Errorf("expected line to match input, got %s", line)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate for 'alpi', got %d: %v", len(candidates), candidates)
	}
	if !strings.HasSuffix(candidates[0], "alpine/") {
		t.Errorf("expected candidate to end with 'alpine/', got %s", candidates[0])
	}

	// Incomplete prefix "al" should match both directories but not the file
	candidates2, _ := DirectoryCompleter(filepath.Join(tempDir, "al"))
	if len(candidates2) != 2 {
		t.Fatalf("expected 2 candidates for 'al', got %d: %v", len(candidates2), candidates2)
	}
}

func TestVerifyTelegramToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/botvalid-token/getMe") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":123456,"username":"test_bot","first_name":"Test Bot"}}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
	}))
	defer server.Close()

	// VerifyTelegramToken uses https://api.telegram.org directly, so test invalid token returns nil
	info := VerifyTelegramToken(context.Background(), "invalid-token-123")
	if info != nil {
		t.Errorf("expected nil for invalid token, got %v", info)
	}
}

func TestSetupActionWithIO(t *testing.T) {
	tempDir := t.TempDir()
	targetWorkspace := filepath.Join(tempDir, "my-code")

	// Input sequence:
	// 1. token: "mock-bot-token"
	// 2. allowed user IDs: "111222, 333444"
	// 3. workspace dir: targetWorkspace
	// 4. confirmation for mkdir: "y"
	input := "mock-bot-token\n111222, 333444\n" + targetWorkspace + "\ny\n"
	in := bytes.NewBufferString(input)
	var out bytes.Buffer

	var savedCfg config.PersistedConfig
	saveMock := func(cfg config.PersistedConfig) error {
		savedCfg = cfg
		return nil
	}

	err := SetupActionWithIO(context.Background(), in, &out, saveMock)
	if err != nil {
		t.Fatalf("unexpected SetupAction error: %v", err)
	}

	if savedCfg.TelegramBotToken == nil || *savedCfg.TelegramBotToken != "mock-bot-token" {
		t.Errorf("saved token mismatch: %v", savedCfg.TelegramBotToken)
	}
	if len(savedCfg.AllowedUserIds) != 2 || savedCfg.AllowedUserIds[0] != "111222" {
		t.Errorf("saved allowed user IDs mismatch: %v", savedCfg.AllowedUserIds)
	}
	if savedCfg.WorkspaceBaseDir == nil || *savedCfg.WorkspaceBaseDir != targetWorkspace {
		t.Errorf("saved workspace dir mismatch: %v", savedCfg.WorkspaceBaseDir)
	}

	// Target directory should have been created
	if fi, err := os.Stat(targetWorkspace); err != nil || !fi.IsDir() {
		t.Errorf("expected directory %s to be created", targetWorkspace)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "Setup completed!") {
		t.Errorf("expected completion message, got: %s", outStr)
	}
}
