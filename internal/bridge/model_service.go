package bridge

import (
	"fmt"
	"strings"
	"sync"

	"antigravity-telegram-remote/internal/i18n"
)

// Model represents an LLM model selectable in Antigravity.
type Model string

const (
	ModelGemini31ProHigh     Model = "gemini-3.1-pro-high"
	ModelGemini31ProLow      Model = "gemini-3.1-pro-low"
	ModelGemini3Flash        Model = "gemini-3-flash"
	ModelClaudeSonnet46Think Model = "claude-sonnet-4.6-thinking"
	ModelClaudeOpus46Think   Model = "claude-opus-4.6-thinking"
	ModelGptOss120bMedium    Model = "gpt-oss-120b-medium"

	DefaultModel Model = ModelGemini3Flash
)

// AvailableModels contains the list of all available models.
var AvailableModels = []Model{
	ModelGemini31ProHigh,
	ModelGemini31ProLow,
	ModelGemini3Flash,
	ModelClaudeSonnet46Think,
	ModelClaudeOpus46Think,
	ModelGptOss120bMedium,
}

// ModelSetResult represents the result of setting an LLM model.
type ModelSetResult struct {
	Success bool
	Model   Model
	Error   string
}

// ModelService manages LLM model selection with thread-safe access.
type ModelService struct {
	mu           sync.RWMutex
	currentModel Model
}

// NewModelService creates a new ModelService initialized with DefaultModel.
func NewModelService() *ModelService {
	return &ModelService{currentModel: DefaultModel}
}

// GetCurrentModel returns the current LLM model.
func (s *ModelService) GetCurrentModel() Model {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentModel
}

// SetModel switches the LLM model case-insensitively.
func (s *ModelService) SetModel(modelName string) ModelSetResult {
	if strings.TrimSpace(modelName) == "" {
		return ModelSetResult{
			Success: false,
			Error:   i18n.T("⚠️ Model name not specified. Available models: ") + strings.Join(s.GetAvailableModels(), ", "),
		}
	}

	normalized := Model(strings.ToLower(strings.TrimSpace(modelName)))
	valid := false
	for _, m := range AvailableModels {
		if m == normalized {
			valid = true
			break
		}
	}

	if !valid {
		return ModelSetResult{
			Success: false,
			Error:   i18n.T(fmt.Sprintf("⚠️ Invalid model \"%s\". Available models: %s", modelName, strings.Join(s.GetAvailableModels(), ", "))),
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentModel = normalized
	return ModelSetResult{
		Success: true,
		Model:   s.currentModel,
	}
}

// GetAvailableModels returns the list of available model names as strings.
func (s *ModelService) GetAvailableModels() []string {
	res := make([]string, len(AvailableModels))
	for i, m := range AvailableModels {
		res[i] = string(m)
	}
	return res
}
