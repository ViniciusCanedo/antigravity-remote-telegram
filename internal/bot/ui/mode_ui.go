package ui

import (
	"context"
	"fmt"
	"strings"
)

// Mode constants matching TypeScript modeService.
var AvailableModes = []string{"fast", "plan"}

var ModeDisplayNames = map[string]string{
	"fast": "Fast",
	"plan": "Planning",
}

var ModeDescriptions = map[string]string{
	"fast": "Agent will execute tasks directly. Use for simple tasks that can be completed faster",
	"plan": "Agent can plan before executing tasks. Use for deep research, complex tasks, or collaborative work",
}

// ModeCdpProvider abstracts reading current mode directly from UI via CDP.
type ModeCdpProvider interface {
	GetCurrentMode(ctx context.Context) (string, error)
}

// ModeServiceProvider abstracts ModeService.
type ModeServiceProvider interface {
	GetCurrentMode() string
	SetMode(mode string)
}

// ModeUIDeps represents optional dependencies for live mode synchronization with CDP.
type ModeUIDeps struct {
	GetCurrentCdp func() ModeCdpProvider
}

// BuildModeUI syncs live mode from CDP (if available) and builds the Mode Management payload.
func BuildModeUI(
	ctx context.Context,
	modeService ModeServiceProvider,
	deps *ModeUIDeps,
) (UIPayload, error) {
	if deps != nil && deps.GetCurrentCdp != nil {
		cdp := deps.GetCurrentCdp()
		if cdp != nil {
			liveMode, err := cdp.GetCurrentMode(ctx)
			if err == nil && liveMode != "" && modeService != nil {
				modeService.SetMode(liveMode)
			}
		}
	}

	currentMode := ""
	if modeService != nil {
		currentMode = modeService.GetCurrentMode()
	}

	modeLines := make([]string, 0, len(AvailableModes))
	for _, m := range AvailableModes {
		icon := "⬜"
		if m == currentMode {
			icon = "✅"
		}
		dName := ModeDisplayNames[m]
		if dName == "" {
			dName = m
		}
		desc := ModeDescriptions[m]
		modeLines = append(modeLines, fmt.Sprintf("%s <b>%s</b> — %s", icon, EscapeHTML(dName), EscapeHTML(desc)))
	}

	curDisp := ModeDisplayNames[currentMode]
	if curDisp == "" {
		curDisp = currentMode
	}
	curDesc := ModeDescriptions[currentMode]

	text := fmt.Sprintf(
		"<b>Mode Management</b>\n\n<b>Current Mode:</b> %s\n%s\n\n<b>Available Modes (%d)</b>\n%s",
		EscapeHTML(curDisp),
		EscapeHTML(curDesc),
		len(AvailableModes),
		strings.Join(modeLines, "\n"),
	)

	keyboard := NewInlineKeyboard()
	for _, m := range AvailableModes {
		dName := ModeDisplayNames[m]
		if dName == "" {
			dName = m
		}
		label := dName
		if m == currentMode {
			label = "✅ " + dName
		}
		keyboard.Text(label, "mode_select:"+m).Row()
	}
	keyboard.Clean()

	return UIPayload{
		Text:     text,
		Keyboard: keyboard,
	}, nil
}

// SendModeUI builds the mode UI payload and sends it via sendFn.
func SendModeUI(
	ctx context.Context,
	sendFn SendUIFunc,
	modeService ModeServiceProvider,
	deps *ModeUIDeps,
) error {
	payload, err := BuildModeUI(ctx, modeService, deps)
	if err != nil {
		return err
	}
	if sendFn == nil {
		return nil
	}
	return sendFn(ctx, payload.Text, payload.Keyboard)
}
