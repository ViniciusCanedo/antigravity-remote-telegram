package config

import (
	"antigravity-telegram-remote/internal/logger"
)

// ExtractionMode defines the DOM extraction mode ('legacy' | 'structured').
type ExtractionMode string

const (
	ExtractionModeLegacy     ExtractionMode = "legacy"
	ExtractionModeStructured ExtractionMode = "structured"
)

// ResponseDeliveryMode defines the response delivery mode ('stream').
type ResponseDeliveryMode string

const (
	ResponseDeliveryModeStream ResponseDeliveryMode = "stream"
)

// AppConfig represents the validated runtime configuration.
type AppConfig struct {
	TelegramBotToken     string          `json:"telegramBotToken"`
	AllowedUserIds       []string        `json:"allowedUserIds"`
	WorkspaceBaseDir     string          `json:"workspaceBaseDir"`
	AutoApproveFileEdits bool            `json:"autoApproveFileEdits"`
	LogLevel             logger.LogLevel `json:"logLevel"`
	ExtractionMode       ExtractionMode  `json:"extractionMode"`
	UseTopics            bool            `json:"useTopics"`
}

// PersistedConfig represents optional persisted configuration stored in ~/.antigravity-telegram-remote/config.json.
// Pointers are used to distinguish omitted fields from zero values.
type PersistedConfig struct {
	TelegramBotToken     *string          `json:"telegramBotToken,omitempty"`
	AllowedUserIds       []string         `json:"allowedUserIds,omitempty"`
	WorkspaceBaseDir     *string          `json:"workspaceBaseDir,omitempty"`
	AutoApproveFileEdits *bool            `json:"autoApproveFileEdits,omitempty"`
	LogLevel             *logger.LogLevel `json:"logLevel,omitempty"`
	ExtractionMode       *ExtractionMode  `json:"extractionMode,omitempty"`
	UseTopics            *bool            `json:"useTopics,omitempty"`
}

// ResolveResponseDeliveryMode always returns ResponseDeliveryModeStream ("stream").
func ResolveResponseDeliveryMode() ResponseDeliveryMode {
	return ResponseDeliveryModeStream
}

// LoadConfig loads the application configuration using the default loader.
func LoadConfig() (AppConfig, error) {
	return DefaultLoader.Load(nil)
}
