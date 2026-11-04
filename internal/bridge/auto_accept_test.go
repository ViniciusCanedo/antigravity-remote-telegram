package bridge

import (
	"strings"
	"testing"
)

func TestAutoAcceptService_InitialState(t *testing.T) {
	if NewAutoAcceptService().IsEnabled() {
		t.Fatal("expected default to be false")
	}
	if !NewAutoAcceptService(true).IsEnabled() {
		t.Fatal("expected initialized to true")
	}
}

func TestAutoAcceptService_EnablesWithOn(t *testing.T) {
	service := NewAutoAcceptService(false)
	result := service.Handle("on")

	if !result.Success {
		t.Fatal("expected success to be true")
	}
	if !result.Enabled {
		t.Fatal("expected enabled to be true")
	}
	if !result.Changed {
		t.Fatal("expected changed to be true")
	}
	if !service.IsEnabled() {
		t.Fatal("expected service.IsEnabled() to be true")
	}
}

func TestAutoAcceptService_DisablesWithOff(t *testing.T) {
	service := NewAutoAcceptService(true)
	result := service.Handle("off")

	if !result.Success {
		t.Fatal("expected success to be true")
	}
	if result.Enabled {
		t.Fatal("expected enabled to be false")
	}
	if !result.Changed {
		t.Fatal("expected changed to be true")
	}
	if service.IsEnabled() {
		t.Fatal("expected service.IsEnabled() to be false")
	}
}

func TestAutoAcceptService_StatusDoesNotChangeState(t *testing.T) {
	service := NewAutoAcceptService(true)
	result := service.Handle("status")

	if !result.Success {
		t.Fatal("expected success to be true")
	}
	if !result.Enabled {
		t.Fatal("expected enabled to be true")
	}
	if result.Changed {
		t.Fatal("expected changed to be false")
	}
	if !service.IsEnabled() {
		t.Fatal("expected service.IsEnabled() to remain true")
	}

	// Empty string defaults to status
	resultEmpty := service.Handle("")
	if !resultEmpty.Success || !resultEmpty.Enabled || resultEmpty.Changed {
		t.Fatalf("unexpected empty handle result: %v", resultEmpty)
	}
}

func TestAutoAcceptService_AlreadyOnOrOff(t *testing.T) {
	serviceOn := NewAutoAcceptService(true)
	resOn := serviceOn.Handle("on")
	if !resOn.Success || !resOn.Enabled || resOn.Changed {
		t.Fatalf("expected already on result with changed=false: %v", resOn)
	}

	serviceOff := NewAutoAcceptService(false)
	resOff := serviceOff.Handle("off")
	if !resOff.Success || resOff.Enabled || resOff.Changed {
		t.Fatalf("expected already off result with changed=false: %v", resOff)
	}
}

func TestAutoAcceptService_Aliases(t *testing.T) {
	service := NewAutoAcceptService(false)

	for _, alias := range []string{"enable", "enabled", "true", "1"} {
		res := service.Handle(alias)
		if !res.Success || !res.Enabled {
			t.Fatalf("expected alias %q to enable auto accept", alias)
		}
	}

	for _, alias := range []string{"disable", "disabled", "false", "0"} {
		res := service.Handle(alias)
		if !res.Success || res.Enabled {
			t.Fatalf("expected alias %q to disable auto accept", alias)
		}
	}

	for _, alias := range []string{"state", "show"} {
		res := service.Handle(alias)
		if !res.Success || res.Changed {
			t.Fatalf("expected alias %q to show status", alias)
		}
	}
}

func TestAutoAcceptService_InvalidArgumentReturnsError(t *testing.T) {
	service := NewAutoAcceptService(false)
	result := service.Handle("invalid")

	if result.Success {
		t.Fatal("expected success to be false")
	}
	if result.Enabled {
		t.Fatal("expected enabled to be false")
	}
	if result.Changed {
		t.Fatal("expected changed to be false")
	}
	if service.IsEnabled() {
		t.Fatal("expected service.IsEnabled() to be false")
	}
	if !strings.Contains(result.Message, "on/off/status") {
		t.Fatalf("expected message to contain 'on/off/status', got %q", result.Message)
	}
}
