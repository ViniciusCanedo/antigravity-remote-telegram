package bridge

import (
	"reflect"
	"testing"
)

func TestModelService_GetCurrentModel_Default(t *testing.T) {
	service := NewModelService()
	if service.GetCurrentModel() != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, service.GetCurrentModel())
	}
}

func TestModelService_SetModel_SwitchesValidModel(t *testing.T) {
	service := NewModelService()
	result := service.SetModel("claude-sonnet-4.6-thinking")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Model != ModelClaudeSonnet46Think {
		t.Fatalf("expected model %q, got %q", ModelClaudeSonnet46Think, result.Model)
	}
	if service.GetCurrentModel() != ModelClaudeSonnet46Think {
		t.Fatalf("expected current model %q, got %q", ModelClaudeSonnet46Think, service.GetCurrentModel())
	}
}

func TestModelService_SetModel_RetainsLastSetModel(t *testing.T) {
	service := NewModelService()
	service.SetModel("claude-sonnet-4.6-thinking")
	service.SetModel("gemini-3.1-pro-high")
	if service.GetCurrentModel() != ModelGemini31ProHigh {
		t.Fatalf("expected current model %q, got %q", ModelGemini31ProHigh, service.GetCurrentModel())
	}
}

func TestModelService_SetModel_InvalidModelReturnsError(t *testing.T) {
	service := NewModelService()
	result := service.SetModel("invalid_model")
	if result.Success {
		t.Fatal("expected failure for invalid model, got success")
	}
	if result.Error == "" {
		t.Fatal("expected error message to be defined")
	}
	if service.GetCurrentModel() != DefaultModel {
		t.Fatalf("expected current model to remain %q, got %q", DefaultModel, service.GetCurrentModel())
	}
}

func TestModelService_SetModel_CaseInsensitive(t *testing.T) {
	service := NewModelService()
	result := service.SetModel("GEMINI-3-FLASH")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Model != ModelGemini3Flash {
		t.Fatalf("expected model %q, got %q", ModelGemini3Flash, result.Model)
	}
}

func TestModelService_SetModel_EmptyStringReturnsError(t *testing.T) {
	service := NewModelService()
	result := service.SetModel("")
	if result.Success {
		t.Fatal("expected failure for empty string, got success")
	}
	if result.Error == "" {
		t.Fatal("expected error message to be defined")
	}
}

func TestModelService_SetModel_ClaudeOpus46Thinking(t *testing.T) {
	service := NewModelService()
	result := service.SetModel("claude-opus-4.6-thinking")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Model != ModelClaudeOpus46Think {
		t.Fatalf("expected %q, got %q", ModelClaudeOpus46Think, result.Model)
	}
}

func TestModelService_SetModel_GptOss120bMedium(t *testing.T) {
	service := NewModelService()
	result := service.SetModel("gpt-oss-120b-medium")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Model != ModelGptOss120bMedium {
		t.Fatalf("expected %q, got %q", ModelGptOss120bMedium, result.Model)
	}
}

func TestModelService_GetAvailableModels_ReturnsList(t *testing.T) {
	service := NewModelService()
	models := service.GetAvailableModels()
	expected := []string{
		"gemini-3.1-pro-high",
		"gemini-3.1-pro-low",
		"gemini-3-flash",
		"claude-sonnet-4.6-thinking",
		"claude-opus-4.6-thinking",
		"gpt-oss-120b-medium",
	}
	if !reflect.DeepEqual(models, expected) {
		t.Fatalf("expected %v, got %v", expected, models)
	}
	if len(models) == 0 {
		t.Fatal("expected non-empty available models list")
	}
}

func TestModelService_GetAvailableModels_IncludesExpectedModels(t *testing.T) {
	service := NewModelService()
	models := service.GetAvailableModels()
	lookup := make(map[string]bool)
	for _, m := range models {
		lookup[m] = true
	}

	for _, expected := range []string{
		"claude-sonnet-4.6-thinking",
		"gpt-oss-120b-medium",
		"gemini-3.1-pro-high",
		"claude-opus-4.6-thinking",
		"gemini-3-flash",
	} {
		if !lookup[expected] {
			t.Fatalf("expected available models to contain %q", expected)
		}
	}
}
