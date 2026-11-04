package ui

import "context"

const (
	AutoAcceptBtnOn      = "autoaccept_btn_on"
	AutoAcceptBtnOff     = "autoaccept_btn_off"
	AutoAcceptBtnRefresh = "autoaccept_btn_refresh"
)

// AutoAcceptStateProvider abstracts the auto-accept state query.
type AutoAcceptStateProvider interface {
	IsEnabled() bool
}

// BuildAutoAcceptUI builds the HTML text and inline keyboard for Auto-accept Management.
func BuildAutoAcceptUI(autoAcceptService AutoAcceptStateProvider) UIPayload {
	enabled := autoAcceptService != nil && autoAcceptService.IsEnabled()

	status := "⚪ OFF"
	if enabled {
		status = "🟢 ON"
	}

	text := "<b>Auto-accept Management</b>\n\n" +
		"<b>Current Status:</b> " + status + "\n\n" +
		"ON: approval dialogs are automatically allowed.\n" +
		"OFF: approval dialogs require manual action."

	onLabel := "Turn ON"
	if enabled {
		onLabel = "✅ Turn ON"
	}

	offLabel := "Turn OFF"
	if !enabled {
		offLabel = "🔴 Turn OFF"
	}

	keyboard := NewInlineKeyboard().
		Text(onLabel, AutoAcceptBtnOn).
		Text(offLabel, AutoAcceptBtnOff).
		Row().
		Text("🔄 Refresh", AutoAcceptBtnRefresh)

	return UIPayload{
		Text:     text,
		Keyboard: keyboard,
	}
}

// SendAutoAcceptUI builds and sends the auto-accept management UI via sendFn.
func SendAutoAcceptUI(
	ctx context.Context,
	sendFn SendUIFunc,
	autoAcceptService AutoAcceptStateProvider,
) error {
	payload := BuildAutoAcceptUI(autoAcceptService)
	if sendFn == nil {
		return nil
	}
	return sendFn(ctx, payload.Text, payload.Keyboard)
}
