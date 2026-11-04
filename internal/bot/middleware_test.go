package bot

import (
	"context"
	"testing"

	"antigravity-telegram-remote/internal/config"
)

func TestWithAuth_AllowedUser(t *testing.T) {
	called := 0
	next := func() {
		called++
	}

	allowed := []string{"123", "456"}
	WithAuth("123", allowed, next)

	if called != 1 {
		t.Fatalf("expected next to be called once, got %d", called)
	}
}

func TestWithAuth_DeniedUser(t *testing.T) {
	called := 0
	next := func() {
		called++
	}

	allowed := []string{"123", "456"}
	WithAuth("789", allowed, next)

	if called != 0 {
		t.Fatalf("expected next not to be called, got %d", called)
	}
}

func TestAuthMiddleware_DeniedCallbackQuery(t *testing.T) {
	botInstance := &Bot{
		Config: &config.AppConfig{
			AllowedUserIds: []string{"123", "456"},
		},
	}

	answerText := ""
	answerCallback := func(text string) error {
		answerText = text
		return nil
	}

	nextCalled := false
	next := func(ctx context.Context) error {
		nextCalled = true
		return nil
	}

	err := botInstance.AuthMiddleware(context.Background(), "999", true, answerCallback, next)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextCalled {
		t.Fatal("expected next not to be called for unauthorized callback query")
	}
	if answerText != "You do not have permission." {
		t.Fatalf("expected 'You do not have permission.', got %q", answerText)
	}
}

func TestAuthMiddleware_DeniedMessage(t *testing.T) {
	botInstance := &Bot{
		Config: &config.AppConfig{
			AllowedUserIds: []string{"123", "456"},
		},
	}

	callbackCalled := false
	answerCallback := func(text string) error {
		callbackCalled = true
		return nil
	}

	nextCalled := false
	next := func(ctx context.Context) error {
		nextCalled = true
		return nil
	}

	err := botInstance.AuthMiddleware(context.Background(), "999", false, answerCallback, next)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextCalled {
		t.Fatal("expected next not to be called for unauthorized message")
	}
	if callbackCalled {
		t.Fatal("expected callback not to be called for unauthorized message")
	}
}

func TestAuthMiddleware_AllowedUser(t *testing.T) {
	botInstance := &Bot{
		Config: &config.AppConfig{
			AllowedUserIds: []string{"123", "456"},
		},
	}

	callbackCalled := false
	answerCallback := func(text string) error {
		callbackCalled = true
		return nil
	}

	nextCalled := false
	next := func(ctx context.Context) error {
		nextCalled = true
		return nil
	}

	err := botInstance.AuthMiddleware(context.Background(), "123", true, answerCallback, next)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nextCalled {
		t.Fatal("expected next to be called for authorized user")
	}
	if callbackCalled {
		t.Fatal("expected callback not to be called for authorized user")
	}
}
