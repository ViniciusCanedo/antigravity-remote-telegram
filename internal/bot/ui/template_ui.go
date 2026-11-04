package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	TemplateBtnPrefix   = "template_btn_"
	MaxPromptPreviewLen = 60
	MaxTemplateButtons  = 25
)

// TemplateItem represents a template database record from TemplateRepository.
type TemplateItem struct {
	ID        int
	Name      string
	Prompt    string
	CreatedAt string
}

// ParseTemplateButtonID extracts the numeric ID from "template_btn_<id>".
// Returns (id, true) if valid, or (0, false) if invalid (equivalent to NaN in TypeScript).
func ParseTemplateButtonID(customID string) (int, bool) {
	if !strings.HasPrefix(customID, TemplateBtnPrefix) {
		return 0, false
	}
	idStr := strings.TrimPrefix(customID, TemplateBtnPrefix)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, false
	}
	return id, true
}

// BuildTemplateUI constructs the HTML text and inline keyboard for the template management menu.
func BuildTemplateUI(templates []TemplateItem) UIPayload {
	if len(templates) == 0 {
		text := "<b>Template Management</b>\n\n" +
			"No templates registered.\n\n" +
			"Use /template_add to add one."
		return UIPayload{
			Text:     text,
			Keyboard: NewInlineKeyboard(),
		}
	}

	truncate := func(text string, max int) string {
		r := []rune(text)
		if len(r) > max {
			return string(r[:max-3]) + "..."
		}
		return text
	}

	limit := len(templates)
	if limit > MaxTemplateButtons {
		limit = MaxTemplateButtons
	}
	displayTemplates := templates[:limit]
	hasMore := len(templates) > MaxTemplateButtons

	descriptions := make([]string, len(displayTemplates))
	for i, tpl := range displayTemplates {
		descriptions[i] = fmt.Sprintf(
			"<b>%d. %s</b>\n  %s",
			i+1,
			EscapeHTML(tpl.Name),
			EscapeHTML(truncate(tpl.Prompt, MaxPromptPreviewLen)),
		)
	}

	footerText := "\n\nTap a button to execute the template."
	if hasMore {
		footerText = fmt.Sprintf("\n\n%d templates are hidden.", len(templates)-MaxTemplateButtons)
	}

	text := fmt.Sprintf(
		"<b>Template Management</b>\n\n<b>Registered Templates (%d)</b>\n\n%s%s",
		len(templates),
		strings.Join(descriptions, "\n\n"),
		footerText,
	)

	keyboard := NewInlineKeyboard()
	for _, tpl := range displayTemplates {
		nameRunes := []rune(tpl.Name)
		safeLabel := tpl.Name
		if len(nameRunes) > 40 {
			safeLabel = string(nameRunes[:37]) + "..."
		}
		keyboard.Text(fmt.Sprintf("▶ %s", safeLabel), fmt.Sprintf("%s%d", TemplateBtnPrefix, tpl.ID)).Row()
	}
	keyboard.Clean()

	return UIPayload{
		Text:     text,
		Keyboard: keyboard,
	}
}

// SendTemplateUI builds and dispatches the template management UI via sendFn.
func SendTemplateUI(
	ctx context.Context,
	sendFn SendUIFunc,
	templates []TemplateItem,
) error {
	payload := BuildTemplateUI(templates)
	if sendFn == nil {
		return nil
	}
	return sendFn(ctx, payload.Text, payload.Keyboard)
}
