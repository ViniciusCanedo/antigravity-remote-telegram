package bridge

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type mockCdpEvaluator struct {
	contextID *int
	callFn    func(ctx context.Context, method string, params map[string]any) (map[string]any, error)
	wsName    string
}

func (m *mockCdpEvaluator) GetPrimaryContextId() *int {
	return m.contextID
}

func (m *mockCdpEvaluator) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	if m.callFn != nil {
		return m.callFn(ctx, method, params)
	}
	return nil, nil
}

func (m *mockCdpEvaluator) GetCurrentWorkspaceName() string {
	return m.wsName
}

func TestTitleGenerator_Fallback_GeneratesShortTitle(t *testing.T) {
	service := NewTitleGeneratorService()
	title := service.GenerateTitle(context.Background(), "React認証バグを修正したい")

	if title == "" {
		t.Fatal("expected non-empty title")
	}
	if len([]rune(title)) > 80 {
		t.Fatalf("expected title length <= 80, got %d", len([]rune(title)))
	}
}

func TestTitleGenerator_Fallback_RemovesWorkspacePrefix(t *testing.T) {
	service := NewTitleGeneratorService()
	title := service.GenerateTitle(context.Background(), "[ワークスペース: /Users/test/Code/project]\nReactの認証バグ")

	if strings.Contains(title, "ワークスペース") {
		t.Fatalf("expected title not to contain 'ワークスペース', got %q", title)
	}
	if title == "" {
		t.Fatal("expected non-empty title")
	}
}

func TestTitleGenerator_Fallback_EmptyPromptReturnsUntitled(t *testing.T) {
	service := NewTitleGeneratorService()
	title := service.GenerateTitle(context.Background(), "")
	if title != "untitled" {
		t.Fatalf("expected 'untitled', got %q", title)
	}
}

func TestTitleGenerator_Fallback_SpecialCharsOnlyReturnsUntitled(t *testing.T) {
	service := NewTitleGeneratorService()
	title := service.GenerateTitle(context.Background(), "!!!@@@###")
	if title != "untitled" {
		t.Fatalf("expected 'untitled', got %q", title)
	}
}

func TestTitleGenerator_Fallback_TruncatesLongPrompt(t *testing.T) {
	service := NewTitleGeneratorService()
	longPrompt := strings.Repeat("a", 200)
	title := service.GenerateTitle(context.Background(), longPrompt)
	if len([]rune(title)) > 80 {
		t.Fatalf("expected length <= 80, got %d", len([]rune(title)))
	}
}

func TestTitleGenerator_Sanitize_SpacesToHyphens(t *testing.T) {
	service := NewTitleGeneratorService()
	result := service.SanitizeForChannelName("hello world")
	if result != "hello-world" {
		t.Fatalf("expected 'hello-world', got %q", result)
	}
}

func TestTitleGenerator_Sanitize_UppercaseToLowercase(t *testing.T) {
	service := NewTitleGeneratorService()
	result := service.SanitizeForChannelName("Hello World")
	if result != "hello-world" {
		t.Fatalf("expected 'hello-world', got %q", result)
	}
}

func TestTitleGenerator_Sanitize_PreservesJapaneseCharacters(t *testing.T) {
	service := NewTitleGeneratorService()
	result := service.SanitizeForChannelName("認証バグ修正")
	if result != "認証バグ修正" {
		t.Fatalf("expected '認証バグ修正', got %q", result)
	}
}

func TestTitleGenerator_Sanitize_SpecialCharsToHyphens(t *testing.T) {
	service := NewTitleGeneratorService()
	result := service.SanitizeForChannelName("test@#$%ing")
	if strings.Contains(result, "@") || strings.Contains(result, "#") {
		t.Fatalf("expected special chars replaced with hyphens, got %q", result)
	}
	if result != "test-ing" {
		t.Fatalf("expected 'test-ing', got %q", result)
	}
}

func TestTitleGenerator_Sanitize_CollapsesConsecutiveHyphens(t *testing.T) {
	service := NewTitleGeneratorService()
	result := service.SanitizeForChannelName("a---b")
	if result != "a-b" {
		t.Fatalf("expected 'a-b', got %q", result)
	}
}

func TestTitleGenerator_Sanitize_RemovesLeadingAndTrailingHyphens(t *testing.T) {
	service := NewTitleGeneratorService()
	result := service.SanitizeForChannelName("-test-")
	if result != "test" {
		t.Fatalf("expected 'test', got %q", result)
	}
}

func TestTitleGenerator_Sanitize_EmptyStringReturnsUntitled(t *testing.T) {
	service := NewTitleGeneratorService()
	result := service.SanitizeForChannelName("")
	if result != "untitled" {
		t.Fatalf("expected 'untitled', got %q", result)
	}
}

func TestTitleGenerator_ViaCDP_FallsBackOnCDPError(t *testing.T) {
	mockCdp := &mockCdpEvaluator{
		callFn: func(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
			return nil, errors.New("CDP error")
		},
	}

	service := NewTitleGeneratorService()
	title := service.GenerateTitle(context.Background(), "テスト用プロンプト", mockCdp)
	if title == "" {
		t.Fatal("expected non-empty title")
	}
	if title == "untitled" {
		t.Fatal("expected fallback title, got 'untitled'")
	}
}

func TestTitleGenerator_ViaCDP_FallsBackOnOkFalse(t *testing.T) {
	mockCdp := &mockCdpEvaluator{
		callFn: func(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
			return map[string]any{
				"result": map[string]any{
					"value": map[string]any{
						"ok": false,
					},
				},
			}, nil
		},
	}

	service := NewTitleGeneratorService()
	title := service.GenerateTitle(context.Background(), "テスト用プロンプト", mockCdp)
	if title == "" {
		t.Fatal("expected non-empty title")
	}
}
