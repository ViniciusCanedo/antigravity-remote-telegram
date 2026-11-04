package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"antigravity-telegram-remote/internal/logger"
	"github.com/joho/godotenv"
)

const (
	ConfigDirName  = ".antigravity-telegram-remote"
	ConfigFileName = "config.json"
	DefaultDbName  = "antigravity.db"
	DefaultCodeDir = "Code"
)

var ValidLogLevels = []logger.LogLevel{
	logger.LevelDebug,
	logger.LevelInfo,
	logger.LevelWarn,
	logger.LevelError,
	logger.LevelNone,
}

var numericRegex = regexp.MustCompile(`^\d+$`)

// Loader encapsulates file system, environment, and configuration operations.
type Loader struct {
	HomeDirFunc func() (string, error)
	LookupEnv   func(key string) (string, bool)
	ReadFile    func(path string) ([]byte, error)
	WriteFile   func(path string, data []byte, perm os.FileMode) error
	StatExists  func(path string) bool
	MkdirAll    func(path string, perm os.FileMode) error
}

// NewLoader creates a Loader with default operating system hooks.
func NewLoader() *Loader {
	return &Loader{
		HomeDirFunc: os.UserHomeDir,
		LookupEnv:   os.LookupEnv,
		ReadFile:    os.ReadFile,
		WriteFile: func(path string, data []byte, perm os.FileMode) error {
			return os.WriteFile(path, data, perm)
		},
		StatExists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil || !os.IsNotExist(err)
		},
		MkdirAll: os.MkdirAll,
	}
}

// DefaultLoader is the package singleton equivalent to ConfigLoader in TypeScript.
var DefaultLoader = NewLoader()

// GetConfigDir returns the configuration directory (~/.antigravity-telegram-remote).
func (l *Loader) GetConfigDir() string {
	home, _ := l.HomeDirFunc()
	return filepath.Join(home, ConfigDirName)
}

// GetConfigFilePath returns the configuration file path (~/.antigravity-telegram-remote/config.json).
func (l *Loader) GetConfigFilePath() string {
	return filepath.Join(l.GetConfigDir(), ConfigFileName)
}

// GetDefaultDbPath returns the SQLite database path (~/.antigravity-telegram-remote/antigravity.db).
func (l *Loader) GetDefaultDbPath() string {
	return filepath.Join(l.GetConfigDir(), DefaultDbName)
}

// ConfigExists checks if config.json exists.
func (l *Loader) ConfigExists() bool {
	return l.StatExists(l.GetConfigFilePath())
}

// EnsureConfigDirExists creates the configuration directory if it does not exist.
func (l *Loader) EnsureConfigDirExists() error {
	dir := l.GetConfigDir()
	if !l.StatExists(dir) {
		return l.MkdirAll(dir, 0755)
	}
	return nil
}

func (l *Loader) expandTilde(raw string) string {
	home, _ := l.HomeDirFunc()
	if raw == "~" {
		return home
	}
	if strings.HasPrefix(raw, "~/") {
		return filepath.Join(home, raw[2:])
	}
	return raw
}

func (l *Loader) readPersistedConfig(filePath string) (PersistedConfig, error) {
	if !l.StatExists(filePath) {
		return PersistedConfig{}, nil
	}
	raw, err := l.ReadFile(filePath)
	if err != nil {
		return PersistedConfig{}, err
	}
	var cfg PersistedConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return PersistedConfig{}, fmt.Errorf("Failed to parse config file at %s. Check for syntax errors.", filePath)
	}
	return cfg, nil
}

func (l *Loader) resolveAllowedUserIds(persisted PersistedConfig) []string {
	if envVal, ok := l.LookupEnv("ALLOWED_USER_IDS"); ok && envVal != "" {
		parts := strings.Split(envVal, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" && numericRegex.MatchString(trimmed) {
				result = append(result, trimmed)
			}
		}
		return result
	}
	if len(persisted.AllowedUserIds) > 0 {
		result := make([]string, 0, len(persisted.AllowedUserIds))
		for _, id := range persisted.AllowedUserIds {
			trimmed := strings.TrimSpace(id)
			if trimmed != "" && numericRegex.MatchString(trimmed) {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return []string{}
}

func (l *Loader) resolveBoolean(envKey string, persistedValue *bool, defaultValue bool) bool {
	if envVal, ok := l.LookupEnv(envKey); ok {
		return strings.ToLower(envVal) == "true"
	}
	if persistedValue != nil {
		return *persistedValue
	}
	return defaultValue
}

func (l *Loader) resolveLogLevel(persistedValue *logger.LogLevel) logger.LogLevel {
	var raw string
	if envVal, ok := l.LookupEnv("LOG_LEVEL"); ok {
		raw = strings.ToLower(envVal)
	} else if persistedValue != nil {
		raw = strings.ToLower(string(*persistedValue))
	}

	for _, lvl := range ValidLogLevels {
		if logger.LogLevel(raw) == lvl {
			return lvl
		}
	}
	return logger.LevelInfo
}

func (l *Loader) resolveExtractionMode(persistedValue *ExtractionMode) ExtractionMode {
	var raw string
	if envVal, ok := l.LookupEnv("EXTRACTION_MODE"); ok {
		raw = envVal
	} else if persistedValue != nil {
		raw = string(*persistedValue)
	}
	if raw == "legacy" {
		return ExtractionModeLegacy
	}
	return ExtractionModeStructured
}

func (l *Loader) mergeConfig(persisted PersistedConfig) (AppConfig, error) {
	var token string
	if envVal, ok := l.LookupEnv("TELEGRAM_BOT_TOKEN"); ok {
		token = envVal
	} else if persisted.TelegramBotToken != nil {
		token = *persisted.TelegramBotToken
	}
	if token == "" {
		return AppConfig{}, errors.New("Missing required config: TELEGRAM_BOT_TOKEN")
	}

	allowedUserIds := l.resolveAllowedUserIds(persisted)
	if len(allowedUserIds) == 0 {
		return AppConfig{}, errors.New("Missing required config: ALLOWED_USER_IDS")
	}

	home, _ := l.HomeDirFunc()
	defaultDir := filepath.Join(home, DefaultCodeDir)
	rawDir := defaultDir
	if envVal, ok := l.LookupEnv("WORKSPACE_BASE_DIR"); ok {
		rawDir = envVal
	} else if persisted.WorkspaceBaseDir != nil {
		rawDir = *persisted.WorkspaceBaseDir
	}
	workspaceBaseDir := l.expandTilde(rawDir)

	autoApproveFileEdits := l.resolveBoolean("AUTO_APPROVE_FILE_EDITS", persisted.AutoApproveFileEdits, false)
	logLevel := l.resolveLogLevel(persisted.LogLevel)
	extractionMode := l.resolveExtractionMode(persisted.ExtractionMode)
	useTopics := l.resolveBoolean("USE_TOPICS", persisted.UseTopics, true)

	return AppConfig{
		TelegramBotToken:     token,
		AllowedUserIds:       allowedUserIds,
		WorkspaceBaseDir:     workspaceBaseDir,
		AutoApproveFileEdits: autoApproveFileEdits,
		LogLevel:             logLevel,
		ExtractionMode:       extractionMode,
		UseTopics:            useTopics,
	}, nil
}

// Load loads configuration from persisted JSON file, overriding with environment variables.
func (l *Loader) Load(persistedOverride *PersistedConfig) (AppConfig, error) {
	var persisted PersistedConfig
	if persistedOverride != nil {
		persisted = *persistedOverride
	} else {
		var err error
		persisted, err = l.readPersistedConfig(l.GetConfigFilePath())
		if err != nil {
			return AppConfig{}, err
		}
	}
	return l.mergeConfig(persisted)
}

// Save merges the partial configuration into ~/.antigravity-telegram-remote/config.json.
func (l *Loader) Save(config PersistedConfig) error {
	if err := l.EnsureConfigDirExists(); err != nil {
		return err
	}
	existing, err := l.readPersistedConfig(l.GetConfigFilePath())
	if err != nil {
		return err
	}

	merged := existing
	if config.TelegramBotToken != nil {
		merged.TelegramBotToken = config.TelegramBotToken
	}
	if config.AllowedUserIds != nil {
		merged.AllowedUserIds = config.AllowedUserIds
	}
	if config.WorkspaceBaseDir != nil {
		merged.WorkspaceBaseDir = config.WorkspaceBaseDir
	}
	if config.AutoApproveFileEdits != nil {
		merged.AutoApproveFileEdits = config.AutoApproveFileEdits
	}
	if config.LogLevel != nil {
		merged.LogLevel = config.LogLevel
	}
	if config.ExtractionMode != nil {
		merged.ExtractionMode = config.ExtractionMode
	}
	if config.UseTopics != nil {
		merged.UseTopics = config.UseTopics
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return l.WriteFile(l.GetConfigFilePath(), data, 0644)
}

// LoadDotEnv loads an environment file (defaulting to .env) overriding existing environment variables.
func (l *Loader) LoadDotEnv(envPath string) error {
	if envPath == "" {
		envPath = ".env"
	}
	if !l.StatExists(envPath) {
		return nil
	}
	return godotenv.Overload(envPath)
}

// Package-level delegates calling DefaultLoader:
func GetConfigDir() string         { return DefaultLoader.GetConfigDir() }
func GetConfigFilePath() string    { return DefaultLoader.GetConfigFilePath() }
func GetDefaultDbPath() string     { return DefaultLoader.GetDefaultDbPath() }
func ConfigExists() bool           { return DefaultLoader.ConfigExists() }
func EnsureConfigDirExists() error { return DefaultLoader.EnsureConfigDirExists() }
func Load(persistedOverride *PersistedConfig) (AppConfig, error) {
	return DefaultLoader.Load(persistedOverride)
}
func Save(partial PersistedConfig) error { return DefaultLoader.Save(partial) }
func LoadDotEnv(envPath string) error    { return DefaultLoader.LoadDotEnv(envPath) }
