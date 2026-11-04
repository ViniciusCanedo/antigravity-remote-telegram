package ui

import (
	"context"
	"strings"

	"github.com/go-telegram/bot/models"
)

// InlineKeyboardButton represents a single button of a Telegram inline keyboard.
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

// InlineKeyboardMarkup represents the matrix of inline buttons in reply_markup.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// NewInlineKeyboard creates an empty inline keyboard.
func NewInlineKeyboard() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{},
	}
}

// Text adds a button to the current row (creating the first row if needed).
func (k *InlineKeyboardMarkup) Text(label, callbackData string) *InlineKeyboardMarkup {
	if len(k.InlineKeyboard) == 0 {
		k.InlineKeyboard = append(k.InlineKeyboard, []InlineKeyboardButton{})
	}
	lastRow := len(k.InlineKeyboard) - 1
	k.InlineKeyboard[lastRow] = append(k.InlineKeyboard[lastRow], InlineKeyboardButton{
		Text:         label,
		CallbackData: callbackData,
	})
	return k
}

// Row ends the current row and prepares a new row for subsequent buttons.
func (k *InlineKeyboardMarkup) Row() *InlineKeyboardMarkup {
	if len(k.InlineKeyboard) == 0 || len(k.InlineKeyboard[len(k.InlineKeyboard)-1]) > 0 {
		k.InlineKeyboard = append(k.InlineKeyboard, []InlineKeyboardButton{})
	}
	return k
}

// Clean removes any empty rows from the keyboard.
func (k *InlineKeyboardMarkup) Clean() *InlineKeyboardMarkup {
	if k == nil {
		return nil
	}
	filtered := make([][]InlineKeyboardButton, 0, len(k.InlineKeyboard))
	for _, row := range k.InlineKeyboard {
		if len(row) > 0 {
			filtered = append(filtered, row)
		}
	}
	k.InlineKeyboard = filtered
	return k
}

// ToTelegramModel converts the keyboard to *models.InlineKeyboardMarkup.
func (k *InlineKeyboardMarkup) ToTelegramModel() *models.InlineKeyboardMarkup {
	if k == nil {
		return nil
	}
	res := &models.InlineKeyboardMarkup{
		InlineKeyboard: make([][]models.InlineKeyboardButton, 0, len(k.InlineKeyboard)),
	}
	for _, row := range k.InlineKeyboard {
		if len(row) == 0 {
			continue
		}
		modelRow := make([]models.InlineKeyboardButton, len(row))
		for i, btn := range row {
			modelRow[i] = models.InlineKeyboardButton{
				Text:         btn.Text,
				CallbackData: btn.CallbackData,
			}
		}
		res.InlineKeyboard = append(res.InlineKeyboard, modelRow)
	}
	return res
}

// UIPayload encapsulates formatted HTML text and the resulting inline keyboard.
type UIPayload struct {
	Text     string
	Keyboard *InlineKeyboardMarkup
}

// SendUIFunc is the sending callback used by UI senders.
type SendUIFunc func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error

// EscapeHTML escapes special HTML characters (&, <, >) for Telegram HTML mode.
func EscapeHTML(text string) string {
	r := strings.ReplaceAll(text, "&", "&amp;")
	r = strings.ReplaceAll(r, "<", "&lt;")
	r = strings.ReplaceAll(r, ">", "&gt;")
	return r
}
