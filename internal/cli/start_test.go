package cli

import (
	"context"
	"errors"
	"testing"

	"antigravity-telegram-remote/internal/logger"
)

func TestStartAction_Success(t *testing.T) {
	lockAcquired := false
	updateChecked := false
	botStarted := false
	var receivedLevel *logger.LogLevel

	deps := &StartDeps{
		AcquireLock: func() error {
			lockAcquired = true
			return nil
		},
		CheckForUpdates: func(ctx context.Context, currentVersion string) {
			updateChecked = true
		},
		StartBot: func(ctx context.Context, cliLogLevel *logger.LogLevel) error {
			botStarted = true
			receivedLevel = cliLogLevel
			return nil
		},
	}

	opts := RootOptions{Verbose: true}
	err := StartAction(context.Background(), opts, deps)
	if err != nil {
		t.Fatalf("unexpected StartAction error: %v", err)
	}

	if !lockAcquired {
		t.Error("expected AcquireLock to be called")
	}
	if !updateChecked {
		t.Error("expected CheckForUpdates to be called")
	}
	if !botStarted {
		t.Error("expected StartBot to be called")
	}
	if receivedLevel == nil || *receivedLevel != logger.LevelDebug {
		t.Errorf("expected LevelDebug, got %v", receivedLevel)
	}
}

func TestStartAction_LockError(t *testing.T) {
	deps := &StartDeps{
		AcquireLock: func() error {
			return errors.New("lock already held")
		},
	}

	err := StartAction(context.Background(), RootOptions{}, deps)
	if err == nil {
		t.Fatal("expected lock error to propagate, got nil")
	}
	if err.Error() != "lock already held" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStartAction_BotError(t *testing.T) {
	deps := &StartDeps{
		AcquireLock: func() error { return nil },
		StartBot: func(ctx context.Context, cliLogLevel *logger.LogLevel) error {
			return errors.New("failed connecting to telegram")
		},
	}

	err := StartAction(context.Background(), RootOptions{}, deps)
	if err == nil {
		t.Fatal("expected bot error to propagate, got nil")
	}
	if err.Error() != "failed connecting to telegram" {
		t.Errorf("unexpected error: %v", err)
	}
}
