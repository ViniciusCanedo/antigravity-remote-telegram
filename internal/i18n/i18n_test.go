package i18n

import (
	"errors"
	"testing"
)

func TestI18n_ReturnsKeyWhenNoTranslationExists(t *testing.T) {
	InitI18n(LangEN)
	result := T("hello.world")
	if result != "hello.world" {
		t.Errorf("expected 'hello.world', got %q", result)
	}
}

func TestI18n_ReturnsKeyWhenTranslationsAreEmpty(t *testing.T) {
	InitI18nWithReader(LangEN, func(lang Language) ([]byte, error) {
		return []byte("{}"), nil
	})
	result := T("some.key")
	if result != "some.key" {
		t.Errorf("expected 'some.key', got %q", result)
	}
}

func TestI18n_SubstitutesVariablesUsingDoubleBraces(t *testing.T) {
	InitI18n(LangEN)
	result := T("Hello {{name}}, you have {{count}} messages", map[string]any{
		"name":  "Alice",
		"count": 5,
	})
	expected := "Hello Alice, you have 5 messages"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestI18n_ReplacesAllOccurrencesOfSameVariable(t *testing.T) {
	InitI18n(LangEN)
	result := T("{{x}} and {{x}} again", map[string]any{"x": "test"})
	expected := "test and test again"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestI18n_HandlesNumericVariableValues(t *testing.T) {
	InitI18n(LangEN)
	result := T("Count: {{n}}", map[string]any{"n": 42})
	expected := "Count: 42"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestI18n_HandlesBooleanVariableValues(t *testing.T) {
	InitI18n(LangEN)
	result := T("Active: {{val}}", map[string]any{"val": true})
	expected := "Active: true"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestI18n_HandlesNilVariableValues(t *testing.T) {
	InitI18n(LangEN)
	result := T("Value: {{v}}", map[string]any{"v": nil})
	expected := "Value: null"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestI18n_DoesNotSubstituteWhenNoVariablesProvided(t *testing.T) {
	InitI18n(LangEN)
	result := T("Hello {{name}}")
	expected := "Hello {{name}}"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestI18n_HandlesEmptyVariablesMap(t *testing.T) {
	InitI18n(LangEN)
	result := T("Hello {{name}}", map[string]any{})
	expected := "Hello {{name}}"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestI18n_HandlesVariableKeysWithDots(t *testing.T) {
	InitI18n(LangEN)
	result := T("Path: {{file.name}}", map[string]any{"file.name": "test.ts"})
	expected := "Path: test.ts"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestI18n_HandlesEmptyStringKey(t *testing.T) {
	result := T("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestI18n_InitI18nDefaultsWithoutThrowing(t *testing.T) {
	InitI18n()
	if GetCurrentLanguage() != LangVI {
		t.Errorf("expected default language to be 'vi', got %s", GetCurrentLanguage())
	}
	result := T("test.key")
	if result != "test.key" {
		t.Errorf("expected 'test.key', got %q", result)
	}
}

func TestI18n_InitI18nAcceptsJaLanguage(t *testing.T) {
	InitI18n(LangJA)
	if GetCurrentLanguage() != LangJA {
		t.Errorf("expected language to be 'ja', got %s", GetCurrentLanguage())
	}
	result := T("test.key")
	if result != "test.key" {
		t.Errorf("expected 'test.key', got %q", result)
	}
}

func TestI18n_RealTranslationsFromEmbedded(t *testing.T) {
	InitI18n(LangEN)
	if T("Auto-approved") != "Auto-approved" {
		t.Errorf("expected 'Auto-approved', got %q", T("Auto-approved"))
	}

	InitI18n(LangJA)
	if T("Auto-approved") != "自動承認しました" {
		t.Errorf("expected '自動承認しました', got %q", T("Auto-approved"))
	}

	InitI18n(LangVI)
	if T("Auto-approved") != "Tự động phê duyệt" {
		t.Errorf("expected 'Tự động phê duyệt', got %q", T("Auto-approved"))
	}
}

func TestI18n_FallbackToEnglish(t *testing.T) {
	InitI18nWithReader(LangJA, func(lang Language) ([]byte, error) {
		if lang == LangEN {
			return []byte(`{"only_in_en": "English Translation"}`), nil
		}
		return []byte(`{}`), nil
	})

	result := T("only_in_en")
	if result != "English Translation" {
		t.Errorf("expected 'English Translation', got %q", result)
	}
}

func TestI18n_WithMockReader_ErrorHandling(t *testing.T) {
	// Simulated read error and malformed JSON
	InitI18nWithReader(LangVI, func(lang Language) ([]byte, error) {
		if lang == LangVI {
			return nil, errors.New("read error")
		}
		if lang == LangJA {
			return []byte("invalid json"), nil
		}
		return []byte("{}"), nil
	})

	result := T("some.key")
	if result != "some.key" {
		t.Errorf("expected fallback to key 'some.key', got %q", result)
	}
}
