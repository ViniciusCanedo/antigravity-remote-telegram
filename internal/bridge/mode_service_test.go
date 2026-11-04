package bridge

import (
	"reflect"
	"testing"
)

func TestModeService_GetCurrentMode_DefaultFast(t *testing.T) {
	service := NewModeService()
	if service.GetCurrentMode() != DefaultMode {
		t.Fatalf("expected default mode %q, got %q", DefaultMode, service.GetCurrentMode())
	}
}

func TestModeService_SetMode_SwitchesValidMode(t *testing.T) {
	service := NewModeService()
	result := service.SetMode("plan")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Mode != ModePlan {
		t.Fatalf("expected mode %q, got %q", ModePlan, result.Mode)
	}
	if service.GetCurrentMode() != ModePlan {
		t.Fatalf("expected current mode %q, got %q", ModePlan, service.GetCurrentMode())
	}
}

func TestModeService_SetMode_RetainsLastModeAfterMultipleSwitches(t *testing.T) {
	service := NewModeService()
	service.SetMode("plan")
	service.SetMode("fast")
	if service.GetCurrentMode() != ModeFast {
		t.Fatalf("expected current mode %q, got %q", ModeFast, service.GetCurrentMode())
	}
}

func TestModeService_SetMode_InvalidModeReturnsError(t *testing.T) {
	service := NewModeService()
	result := service.SetMode("invalid_mode")
	if result.Success {
		t.Fatal("expected failure for invalid mode, got success")
	}
	if result.Error == "" {
		t.Fatal("expected error message to be defined")
	}
	if service.GetCurrentMode() != DefaultMode {
		t.Fatalf("expected current mode to remain %q, got %q", DefaultMode, service.GetCurrentMode())
	}
}

func TestModeService_SetMode_CaseInsensitive(t *testing.T) {
	service := NewModeService()
	result := service.SetMode("FAST")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Mode != ModeFast {
		t.Fatalf("expected mode %q, got %q", ModeFast, result.Mode)
	}
}

func TestModeService_SetMode_EmptyStringReturnsError(t *testing.T) {
	service := NewModeService()
	result := service.SetMode("")
	if result.Success {
		t.Fatal("expected failure for empty string, got success")
	}
	if result.Error == "" {
		t.Fatal("expected error message to be defined")
	}
}

func TestModeService_GetAvailableModes(t *testing.T) {
	service := NewModeService()
	modes := service.GetAvailableModes()
	expected := []string{"fast", "plan"}
	if !reflect.DeepEqual(modes, expected) {
		t.Fatalf("expected %v, got %v", expected, modes)
	}
	if len(modes) != 2 {
		t.Fatalf("expected 2 modes, got %d", len(modes))
	}
}

func TestModeService_GetModeDescription(t *testing.T) {
	descFast := GetModeDescription(ModeFast)
	if descFast == "" {
		t.Fatal("expected non-empty description for ModeFast")
	}
	descPlan := GetModeDescription(ModePlan)
	if descPlan == "" {
		t.Fatal("expected non-empty description for ModePlan")
	}
	descUnknown := GetModeDescription("unknown")
	if descUnknown != "" {
		t.Fatalf("expected empty description for unknown mode, got %q", descUnknown)
	}
}
