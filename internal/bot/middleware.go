package bot

import (
	"context"
	"fmt"
	"strings"

	"antigravity-telegram-remote/internal/logger"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// WithAuth mirrors src/middleware/auth.ts for backwards compatibility and unit testing.
func WithAuth(userID string, allowedUserIDs []string, next func()) {
	if IsUserAllowed(userID, allowedUserIDs) {
		next()
	}
}

// IsUserAllowed checks if userID is in the allowedUserIDs whitelist.
func IsUserAllowed(userID string, allowedUserIDs []string) bool {
	for _, id := range allowedUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// AuthMiddleware processes an authorization check for incoming Telegram events.
// If the user is unauthorized:
//   - For callback queries, answerCallback is invoked with "You do not have permission."
//   - For standard messages, the event is silently dropped without invoking next.
func (b *Bot) AuthMiddleware(
	ctx context.Context,
	userID string,
	isCallbackQuery bool,
	answerCallback func(text string) error,
	next func(ctx context.Context) error,
) error {
	allowed := false
	if b != nil && b.Config != nil {
		allowed = IsUserAllowed(userID, b.Config.AllowedUserIds)
	}

	if !allowed {
		if isCallbackQuery && answerCallback != nil {
			return answerCallback("You do not have permission.")
		}
		return nil
	}

	if next != nil {
		return next(ctx)
	}
	return nil
}

// TelegramAuthMiddleware wraps the authorization logic into a tgbot.Middleware handler.
func (b *Bot) TelegramAuthMiddleware(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
		var userID string
		isCallback := false
		var callbackID string

		if update.Message != nil && update.Message.From != nil {
			userID = fmt.Sprintf("%d", update.Message.From.ID)
		} else if update.CallbackQuery != nil {
			isCallback = true
			callbackID = update.CallbackQuery.ID
			if update.CallbackQuery.From.ID != 0 {
				userID = fmt.Sprintf("%d", update.CallbackQuery.From.ID)
			}
		}

		var allowedList []string
		if b != nil && b.Config != nil {
			allowedList = b.Config.AllowedUserIds
		}
		logger.Debug(fmt.Sprintf("[Auth] User attempt: %s (Allowed: %s)", userID, strings.Join(allowedList, ", ")))

		answerCb := func(text string) error {
			if callbackID == "" {
				return nil
			}
			_, err := botClient.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: callbackID,
				Text:            text,
			})
			return err
		}

		_ = b.AuthMiddleware(ctx, userID, isCallback, answerCb, func(innerCtx context.Context) error {
			next(innerCtx, botClient, update)
			return nil
		})
	}
}
