package foundation_test

import (
	"testing"

	"antigravity-telegram-remote/internal/foundation"
)

func TestPinnedDependencies(t *testing.T) {
	expected := []string{
		"github.com/coder/websocket",
		"github.com/go-telegram/bot",
		"github.com/joho/godotenv",
		"github.com/robfig/cron/v3",
		"github.com/spf13/cobra",
		"golang.org/x/term",
		"modernc.org/sqlite",
	}

	if len(foundation.PinnedDependencies) != len(expected) {
		t.Fatalf("expected %d dependencies, got %d", len(expected), len(foundation.PinnedDependencies))
	}

	for i, dep := range expected {
		if foundation.PinnedDependencies[i] != dep {
			t.Errorf("expected dependency %q at index %d, got %q", dep, i, foundation.PinnedDependencies[i])
		}
	}
}
