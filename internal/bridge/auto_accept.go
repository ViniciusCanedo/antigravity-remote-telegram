package bridge

import (
	"fmt"
	"strings"
	"sync"
)

// AutoAcceptAction represents the requested auto-accept action.
type AutoAcceptAction string

const (
	AutoAcceptOn     AutoAcceptAction = "on"
	AutoAcceptOff    AutoAcceptAction = "off"
	AutoAcceptStatus AutoAcceptAction = "status"
)

// AutoAcceptCommandResult holds the outcome of an auto-accept command.
type AutoAcceptCommandResult struct {
	Success bool
	Enabled bool
	Changed bool
	Message string
}

// AutoAcceptService manages the auto-accept state for dialog confirmations.
type AutoAcceptService struct {
	mu      sync.RWMutex
	enabled bool
}

// NewAutoAcceptService creates a new AutoAcceptService, optionally initialized with enabled status.
func NewAutoAcceptService(initialEnabled ...bool) *AutoAcceptService {
	enabled := false
	if len(initialEnabled) > 0 {
		enabled = initialEnabled[0]
	}
	return &AutoAcceptService{enabled: enabled}
}

// IsEnabled returns the current auto-accept status.
func (s *AutoAcceptService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// Handle processes an auto-accept command action ('on', 'off', 'status' or aliases).
func (s *AutoAcceptService) Handle(rawAction ...string) AutoAcceptCommandResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	var actionInput string
	if len(rawAction) > 0 {
		actionInput = rawAction[0]
	}

	action := normalizeAutoAcceptAction(actionInput)
	if action == nil {
		return AutoAcceptCommandResult{
			Success: false,
			Enabled: s.enabled,
			Changed: false,
			Message: "⚠️ Invalid argument. Usage: `/autoaccept [on/off/status]`",
		}
	}

	if *action == AutoAcceptStatus {
		statusStr := "OFF"
		if s.enabled {
			statusStr = "ON"
		}
		return AutoAcceptCommandResult{
			Success: true,
			Enabled: s.enabled,
			Changed: false,
			Message: fmt.Sprintf("⚙️ Auto-accept mode: **%s**", statusStr),
		}
	}

	if *action == AutoAcceptOn {
		if s.enabled {
			return AutoAcceptCommandResult{
				Success: true,
				Enabled: true,
				Changed: false,
				Message: "ℹ️ Auto-accept mode is already **ON**.",
			}
		}
		s.enabled = true
		return AutoAcceptCommandResult{
			Success: true,
			Enabled: true,
			Changed: true,
			Message: "✅ Auto-accept mode turned **ON**. Future dialogs will be auto-allowed.",
		}
	}

	// Action is 'off'
	if !s.enabled {
		return AutoAcceptCommandResult{
			Success: true,
			Enabled: false,
			Changed: false,
			Message: "ℹ️ Auto-accept mode is already **OFF**.",
		}
	}

	s.enabled = false
	return AutoAcceptCommandResult{
		Success: true,
		Enabled: false,
		Changed: true,
		Message: "✅ Auto-accept mode turned **OFF**. Returned to manual approval.",
	}
}

func normalizeAutoAcceptAction(rawAction string) *AutoAcceptAction {
	trimmed := strings.TrimSpace(rawAction)
	if trimmed == "" {
		status := AutoAcceptStatus
		return &status
	}

	normalized := strings.ToLower(trimmed)
	switch normalized {
	case "on", "enable", "enabled", "true", "1":
		on := AutoAcceptOn
		return &on
	case "off", "disable", "disabled", "false", "0":
		off := AutoAcceptOff
		return &off
	case "status", "state", "show":
		status := AutoAcceptStatus
		return &status
	default:
		return nil
	}
}
