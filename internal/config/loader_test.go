package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"antigravity-telegram-remote/internal/logger"
)

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func logLevelPtr(l logger.LogLevel) *logger.LogLevel {
	return &l
}

func extractionModePtr(e ExtractionMode) *ExtractionMode {
	return &e
}

func newTestLoader(env map[string]string, files map[string][]byte) (*Loader, *[]string, *map[string][]byte) {
	homeDir := "/mock/home"
	if h, err := os.UserHomeDir(); err == nil {
		homeDir = h
	}

	dirsCreated := &[]string{}
	writtenFiles := &map[string][]byte{}
	*writtenFiles = make(map[string][]byte)

	loader := &Loader{
		HomeDirFunc: func() (string, error) {
			return homeDir, nil
		},
		LookupEnv: func(key string) (string, bool) {
			val, ok := env[key]
			return val, ok
		},
		ReadFile: func(path string) ([]byte, error) {
			if content, ok := files[path]; ok {
				return content, nil
			}
			if content, ok := (*writtenFiles)[path]; ok {
				return content, nil
			}
			return nil, os.ErrNotExist
		},
		WriteFile: func(path string, data []byte, perm os.FileMode) error {
			(*writtenFiles)[path] = data
			return nil
		},
		StatExists: func(path string) bool {
			if _, ok := files[path]; ok {
				return true
			}
			if _, ok := (*writtenFiles)[path]; ok {
				return true
			}
			return false
		},
		MkdirAll: func(path string, perm os.FileMode) error {
			*dirsCreated = append(*dirsCreated, path)
			return nil
		},
	}

	return loader, dirsCreated, writtenFiles
}

func TestConfigLoader_GetConfigDir(t *testing.T) {
	loader, _, _ := newTestLoader(nil, nil)
	home, _ := loader.HomeDirFunc()
	expected := filepath.Join(home, ".antigravity-telegram-remote")
	if loader.GetConfigDir() != expected {
		t.Errorf("expected %s, got %s", expected, loader.GetConfigDir())
	}
}

func TestConfigLoader_GetConfigFilePath(t *testing.T) {
	loader, _, _ := newTestLoader(nil, nil)
	home, _ := loader.HomeDirFunc()
	expected := filepath.Join(home, ".antigravity-telegram-remote", "config.json")
	if loader.GetConfigFilePath() != expected {
		t.Errorf("expected %s, got %s", expected, loader.GetConfigFilePath())
	}
}

func TestConfigLoader_GetDefaultDbPath(t *testing.T) {
	loader, _, _ := newTestLoader(nil, nil)
	home, _ := loader.HomeDirFunc()
	expected := filepath.Join(home, ".antigravity-telegram-remote", "antigravity.db")
	if loader.GetDefaultDbPath() != expected {
		t.Errorf("expected %s, got %s", expected, loader.GetDefaultDbPath())
	}
}

func TestConfigLoader_ConfigExists_True(t *testing.T) {
	loader, _, _ := newTestLoader(nil, nil)
	configPath := loader.GetConfigFilePath()
	loader.StatExists = func(path string) bool {
		return path == configPath
	}

	if !loader.ConfigExists() {
		t.Errorf("expected configExists to return true")
	}
}

func TestConfigLoader_ConfigExists_False(t *testing.T) {
	loader, _, _ := newTestLoader(nil, nil)
	loader.StatExists = func(path string) bool {
		return false
	}

	if loader.ConfigExists() {
		t.Errorf("expected configExists to return false")
	}
}

func TestConfigLoader_Load_FromPersistedOverride(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	persisted := &PersistedConfig{
		TelegramBotToken: stringPtr("test-token"),
		AllowedUserIds:   []string{"111", "222"},
	}

	cfg, err := loader.Load(persisted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TelegramBotToken != "test-token" {
		t.Errorf("expected test-token, got %s", cfg.TelegramBotToken)
	}
	if len(cfg.AllowedUserIds) != 2 || cfg.AllowedUserIds[0] != "111" || cfg.AllowedUserIds[1] != "222" {
		t.Errorf("expected ['111', '222'], got %v", cfg.AllowedUserIds)
	}
}

func TestConfigLoader_Load_EnvOverridesPersisted(t *testing.T) {
	env := map[string]string{
		"TELEGRAM_BOT_TOKEN": "env-token",
		"ALLOWED_USER_IDS":   "999",
	}
	loader, _, _ := newTestLoader(env, nil)
	persisted := &PersistedConfig{
		TelegramBotToken: stringPtr("file-token"),
		AllowedUserIds:   []string{"111"},
	}

	cfg, err := loader.Load(persisted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TelegramBotToken != "env-token" {
		t.Errorf("expected env-token, got %s", cfg.TelegramBotToken)
	}
	if len(cfg.AllowedUserIds) != 1 || cfg.AllowedUserIds[0] != "999" {
		t.Errorf("expected ['999'], got %v", cfg.AllowedUserIds)
	}
}

func TestConfigLoader_Load_ThrowsWhenTokenMissing(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	_, err := loader.Load(&PersistedConfig{})
	if err == nil || !strings.Contains(err.Error(), "Missing required config: TELEGRAM_BOT_TOKEN") {
		t.Fatalf("expected error containing 'Missing required config: TELEGRAM_BOT_TOKEN', got %v", err)
	}
}

func TestConfigLoader_Load_ThrowsWhenAllowedUserIdsMissing(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	_, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
	})
	if err == nil || !strings.Contains(err.Error(), "Missing required config: ALLOWED_USER_IDS") {
		t.Fatalf("expected error containing 'Missing required config: ALLOWED_USER_IDS', got %v", err)
	}
}

func TestConfigLoader_Load_DefaultWorkspaceBaseDir(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := loader.HomeDirFunc()
	expected := filepath.Join(home, "Code")
	if cfg.WorkspaceBaseDir != expected {
		t.Errorf("expected %s, got %s", expected, cfg.WorkspaceBaseDir)
	}
}

func TestConfigLoader_Load_ExpandsTildeInWorkspaceBaseDir(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
		WorkspaceBaseDir: stringPtr("~/projects"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := loader.HomeDirFunc()
	expected := filepath.Join(home, "projects")
	if cfg.WorkspaceBaseDir != expected {
		t.Errorf("expected %s, got %s", expected, cfg.WorkspaceBaseDir)
	}
}

func TestConfigLoader_Load_ResolvesAutoApproveFromEnv(t *testing.T) {
	env := map[string]string{
		"AUTO_APPROVE_FILE_EDITS": "true",
	}
	loader, _, _ := newTestLoader(env, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AutoApproveFileEdits {
		t.Errorf("expected autoApproveFileEdits to be true")
	}
}

func TestConfigLoader_Load_DefaultsAutoApproveToFalse(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoApproveFileEdits {
		t.Errorf("expected autoApproveFileEdits to be false")
	}
}

func TestConfigLoader_Load_ResolvesLogLevelFromEnv(t *testing.T) {
	env := map[string]string{
		"LOG_LEVEL": "debug",
	}
	loader, _, _ := newTestLoader(env, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != logger.LevelDebug {
		t.Errorf("expected debug, got %s", cfg.LogLevel)
	}
}

func TestConfigLoader_Load_DefaultsLogLevelToInfo(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != logger.LevelInfo {
		t.Errorf("expected info, got %s", cfg.LogLevel)
	}
}

func TestConfigLoader_Load_ResolvesExtractionModeFromEnv(t *testing.T) {
	env := map[string]string{
		"EXTRACTION_MODE": "legacy",
	}
	loader, _, _ := newTestLoader(env, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExtractionMode != ExtractionModeLegacy {
		t.Errorf("expected legacy, got %s", cfg.ExtractionMode)
	}
}

func TestConfigLoader_Load_DefaultsExtractionModeToStructured(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExtractionMode != ExtractionModeStructured {
		t.Errorf("expected structured, got %s", cfg.ExtractionMode)
	}
}

func TestConfigLoader_Load_ResolvesUseTopicsFromEnv(t *testing.T) {
	env := map[string]string{
		"USE_TOPICS": "false",
	}
	loader, _, _ := newTestLoader(env, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UseTopics {
		t.Errorf("expected useTopics to be false")
	}
}

func TestConfigLoader_Load_DefaultsUseTopicsToTrue(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.UseTopics {
		t.Errorf("expected useTopics to be true")
	}
}

func TestConfigLoader_Load_CommaSeparatedAllowedUserIdsWithWhitespace(t *testing.T) {
	env := map[string]string{
		"TELEGRAM_BOT_TOKEN": "tok",
		"ALLOWED_USER_IDS":   " 111 , 222 , 333 ",
	}
	loader, _, _ := newTestLoader(env, nil)
	cfg, err := loader.Load(&PersistedConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.AllowedUserIds) != 3 || cfg.AllowedUserIds[0] != "111" || cfg.AllowedUserIds[1] != "222" || cfg.AllowedUserIds[2] != "333" {
		t.Errorf("expected ['111', '222', '333'], got %v", cfg.AllowedUserIds)
	}
}

func TestConfigLoader_Load_FiltersEmptyStringsFromAllowedUserIds(t *testing.T) {
	env := map[string]string{
		"TELEGRAM_BOT_TOKEN": "tok",
		"ALLOWED_USER_IDS":   "111,,222,",
	}
	loader, _, _ := newTestLoader(env, nil)
	cfg, err := loader.Load(&PersistedConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.AllowedUserIds) != 2 || cfg.AllowedUserIds[0] != "111" || cfg.AllowedUserIds[1] != "222" {
		t.Errorf("expected ['111', '222'], got %v", cfg.AllowedUserIds)
	}
}

func TestConfigLoader_Load_ThrowsClearErrorForMalformedJSON(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	configPath := loader.GetConfigFilePath()
	loader.StatExists = func(path string) bool {
		return path == configPath
	}
	loader.ReadFile = func(path string) ([]byte, error) {
		return []byte("{ invalid json }"), nil
	}

	_, err := loader.Load(nil)
	if err == nil || !strings.Contains(err.Error(), "Failed to parse config file") {
		t.Fatalf("expected error containing 'Failed to parse config file', got %v", err)
	}
}

func TestConfigLoader_Load_EmptyPersistedWhenFileDoesNotExist(t *testing.T) {
	env := map[string]string{
		"TELEGRAM_BOT_TOKEN": "tok",
		"ALLOWED_USER_IDS":   "1",
	}
	loader, _, _ := newTestLoader(env, nil)
	loader.StatExists = func(path string) bool {
		return false
	}

	cfg, err := loader.Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TelegramBotToken != "tok" {
		t.Errorf("expected tok, got %s", cfg.TelegramBotToken)
	}
}

func TestConfigLoader_Load_IgnoresInvalidLogLevelValues(t *testing.T) {
	env := map[string]string{
		"LOG_LEVEL": "INVALID",
	}
	loader, _, _ := newTestLoader(env, nil)
	cfg, err := loader.Load(&PersistedConfig{
		TelegramBotToken: stringPtr("tok"),
		AllowedUserIds:   []string{"1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != logger.LevelInfo {
		t.Errorf("expected info, got %s", cfg.LogLevel)
	}
}

func TestConfigLoader_Save_CreatesDirAndWritesMergedConfig(t *testing.T) {
	loader, dirsCreated, writtenFiles := newTestLoader(map[string]string{}, nil)
	loader.StatExists = func(path string) bool {
		return false
	}

	err := loader.Save(PersistedConfig{
		TelegramBotToken: stringPtr("new-token"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*dirsCreated) == 0 {
		t.Errorf("expected directory creation, got none")
	}

	filePath := loader.GetConfigFilePath()
	data, ok := (*writtenFiles)[filePath]
	if !ok {
		t.Fatalf("expected file %s to be written", filePath)
	}

	var parsed PersistedConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal written config: %v", err)
	}
	if parsed.TelegramBotToken == nil || *parsed.TelegramBotToken != "new-token" {
		t.Errorf("expected new-token in written config, got %v", parsed.TelegramBotToken)
	}
}

func TestConfigLoader_LoadDotEnv(t *testing.T) {
	loader, _, _ := newTestLoader(map[string]string{}, nil)
	// Missing file should return nil
	err := loader.LoadDotEnv("nonexistent.env")
	if err != nil {
		t.Errorf("expected nil error for missing .env, got %v", err)
	}
}

func TestConfigLoader_PackageDelegates(t *testing.T) {
	origLoader := DefaultLoader
	t.Cleanup(func() {
		DefaultLoader = origLoader
	})

	loader, _, _ := newTestLoader(map[string]string{
		"TELEGRAM_BOT_TOKEN": "token",
		"ALLOWED_USER_IDS":   "123",
	}, nil)
	DefaultLoader = loader

	if GetConfigDir() != loader.GetConfigDir() {
		t.Errorf("GetConfigDir mismatch")
	}
	if GetConfigFilePath() != loader.GetConfigFilePath() {
		t.Errorf("GetConfigFilePath mismatch")
	}
	if GetDefaultDbPath() != loader.GetDefaultDbPath() {
		t.Errorf("GetDefaultDbPath mismatch")
	}
	if ConfigExists() != loader.ConfigExists() {
		t.Errorf("ConfigExists mismatch")
	}
	if err := EnsureConfigDirExists(); err != nil {
		t.Errorf("EnsureConfigDirExists unexpected error: %v", err)
	}
	cfg, err := Load(nil)
	if err != nil || cfg.TelegramBotToken != "token" {
		t.Errorf("Load mismatch: cfg=%v, err=%v", cfg, err)
	}
	if err := Save(PersistedConfig{}); err != nil {
		t.Errorf("Save unexpected error: %v", err)
	}
	if err := LoadDotEnv(""); err != nil {
		t.Errorf("LoadDotEnv unexpected error: %v", err)
	}
}
