package bridge

import (
	"fmt"
	"strings"
	"sync"
)

// Mode represents the Antigravity execution mode.
type Mode string

const (
	ModeFast Mode = "fast"
	ModePlan Mode = "plan"

	DefaultMode Mode = ModeFast
)

// AvailableModes lists all supported execution modes.
var AvailableModes = []Mode{ModeFast, ModePlan}

// ModeDisplayNames maps modes to their display string.
var ModeDisplayNames = map[Mode]string{
	ModeFast: "⚡ Fast",
	ModePlan: "📋 Plan",
}

// ModeUINames maps internal mode names to Antigravity UI display names.
var ModeUINames = map[Mode]string{
	ModeFast: "Fast",
	ModePlan: "Planning",
}

// ModeUINameReverse maps UI display names back to Mode.
var ModeUINameReverse = map[string]Mode{
	"fast":     ModeFast,
	"planning": ModePlan,
}

// GetModeDescription returns the description for the given mode.
func GetModeDescription(mode Mode) string {
	switch mode {
	case ModeFast:
		return "Fast Mode — for simple tasks"
	case ModePlan:
		return "Plan Mode — for complex step-by-step tasks"
	default:
		return ""
	}
}

// ModeSetResult represents the result of setting an execution mode.
type ModeSetResult struct {
	Success bool
	Mode    Mode
	Error   string
}

// ModeService manages execution modes with thread-safe access.
type ModeService struct {
	mu          sync.RWMutex
	currentMode Mode
}

// NewModeService initializes ModeService with the default mode (fast).
func NewModeService() *ModeService {
	return &ModeService{currentMode: DefaultMode}
}

// GetCurrentMode returns the current execution mode.
func (s *ModeService) GetCurrentMode() Mode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentMode
}

// SetMode switches the execution mode case-insensitively.
func (s *ModeService) SetMode(modeName string) ModeSetResult {
	if strings.TrimSpace(modeName) == "" {
		return ModeSetResult{
			Success: false,
			Error:   "⚠️ Mode name not specified. Available modes: " + strings.Join(s.GetAvailableModes(), ", "),
		}
	}

	normalized := Mode(strings.ToLower(strings.TrimSpace(modeName)))
	valid := false
	for _, m := range AvailableModes {
		if m == normalized {
			valid = true
			break
		}
	}

	if !valid {
		return ModeSetResult{
			Success: false,
			Error:   fmt.Sprintf("⚠️ Invalid mode \"%s\". Available modes: %s", modeName, strings.Join(s.GetAvailableModes(), ", ")),
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentMode = normalized
	return ModeSetResult{
		Success: true,
		Mode:    s.currentMode,
	}
}

// GetAvailableModes returns the list of available mode names as strings.
func (s *ModeService) GetAvailableModes() []string {
	res := make([]string, len(AvailableModes))
	for i, m := range AvailableModes {
		res[i] = string(m)
	}
	return res
}
