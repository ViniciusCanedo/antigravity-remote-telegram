package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupConfigTestEnv(t *testing.T, env map[string]string) {
	origLoader := DefaultLoader
	t.Cleanup(func() {
		DefaultLoader = origLoader
	})

	homeDir, _ := os.UserHomeDir()
	loader := &Loader{
		HomeDirFunc: func() (string, error) {
			return homeDir, nil
		},
		LookupEnv: func(key string) (string, bool) {
			val, ok := env[key]
			return val, ok
		},
		ReadFile: func(path string) ([]byte, error) {
			return []byte("{}"), nil
		},
		WriteFile: func(path string, data []byte, perm os.FileMode) error {
			return nil
		},
		StatExists: func(path string) bool {
			return false
		},
		MkdirAll: func(path string, perm os.FileMode) error {
			return nil
		},
	}
	DefaultLoader = loader
}

func TestConfig_ThrowsIfTelegramBotTokenMissing(t *testing.T) {
	setupConfigTestEnv(t, map[string]string{
		"ALLOWED_USER_IDS": "123456",
	})

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "Missing required config: TELEGRAM_BOT_TOKEN") {
		t.Fatalf("expected error containing 'Missing required config: TELEGRAM_BOT_TOKEN', got %v", err)
	}
}

func TestConfig_ThrowsIfAllowedUserIdsMissing(t *testing.T) {
	setupConfigTestEnv(t, map[string]string{
		"TELEGRAM_BOT_TOKEN": "token",
	})

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "Missing required config: ALLOWED_USER_IDS") {
		t.Fatalf("expected error containing 'Missing required config: ALLOWED_USER_IDS', got %v", err)
	}
}

func TestConfig_ReturnsValidConfigWhenAllSet(t *testing.T) {
	setupConfigTestEnv(t, map[string]string{
		"TELEGRAM_BOT_TOKEN": "secret_token",
		"ALLOWED_USER_IDS":   "123456789,987654321",
		"WORKSPACE_BASE_DIR": "/custom/dir",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.TelegramBotToken != "secret_token" {
		t.Errorf("expected secret_token, got %s", cfg.TelegramBotToken)
	}
	if len(cfg.AllowedUserIds) != 2 || cfg.AllowedUserIds[0] != "123456789" || cfg.AllowedUserIds[1] != "987654321" {
		t.Errorf("expected ['123456789', '987654321'], got %v", cfg.AllowedUserIds)
	}
	if cfg.WorkspaceBaseDir != "/custom/dir" {
		t.Errorf("expected /custom/dir, got %s", cfg.WorkspaceBaseDir)
	}
}

func TestConfig_DefaultWorkspaceBaseDir(t *testing.T) {
	setupConfigTestEnv(t, map[string]string{
		"TELEGRAM_BOT_TOKEN": "secret_token",
		"ALLOWED_USER_IDS":   "123456789",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	homeDir, _ := os.UserHomeDir()
	expected := filepath.Join(homeDir, "Code")
	if cfg.WorkspaceBaseDir != expected {
		t.Errorf("expected %s, got %s", expected, cfg.WorkspaceBaseDir)
	}
}

func TestConfig_DefaultsUseTopicsToTrue(t *testing.T) {
	setupConfigTestEnv(t, map[string]string{
		"TELEGRAM_BOT_TOKEN": "secret_token",
		"ALLOWED_USER_IDS":   "123456789",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.UseTopics {
		t.Errorf("expected useTopics to default to true")
	}
}

func TestConfig_DisablesUseTopicsWhenFalse(t *testing.T) {
	setupConfigTestEnv(t, map[string]string{
		"TELEGRAM_BOT_TOKEN": "secret_token",
		"ALLOWED_USER_IDS":   "123456789",
		"USE_TOPICS":         "false",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.UseTopics {
		t.Errorf("expected useTopics to be false")
	}
}

func TestConfig_DefaultsAutoApproveFileEditsToFalse(t *testing.T) {
	setupConfigTestEnv(t, map[string]string{
		"TELEGRAM_BOT_TOKEN": "secret_token",
		"ALLOWED_USER_IDS":   "123456789",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AutoApproveFileEdits {
		t.Errorf("expected autoApproveFileEdits to default to false")
	}
}

func TestConfig_EnablesAutoApproveFileEditsWhenTrue(t *testing.T) {
	setupConfigTestEnv(t, map[string]string{
		"TELEGRAM_BOT_TOKEN":      "secret_token",
		"ALLOWED_USER_IDS":        "123456789",
		"AUTO_APPROVE_FILE_EDITS": "true",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.AutoApproveFileEdits {
		t.Errorf("expected autoApproveFileEdits to be true")
	}
}

func TestConfig_ResolveResponseDeliveryMode(t *testing.T) {
	mode := ResolveResponseDeliveryMode()
	if mode != ResponseDeliveryModeStream {
		t.Errorf("expected %s, got %s", ResponseDeliveryModeStream, mode)
	}
}
