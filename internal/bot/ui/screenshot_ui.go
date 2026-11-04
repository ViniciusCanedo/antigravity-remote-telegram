package ui

import (
	"context"
	"fmt"
)

// ScreenshotResult represents the output of ScreenshotService.Capture.
type ScreenshotResult struct {
	Success bool
	Buffer  []byte
	Error   string
}

// ScreenshotCapturer abstracts capturing a screenshot via CDP.
type ScreenshotCapturer interface {
	Capture(ctx context.Context, format string) (ScreenshotResult, error)
}

// SendPhotoFunc sends an image file (filename + bytes) with an optional caption.
type SendPhotoFunc func(ctx context.Context, filename string, data []byte, caption string) error

// SendTextFunc sends a plain text message.
type SendTextFunc func(ctx context.Context, text string) error

// HandleScreenshot captures a PNG screenshot of Antigravity and sends it via sendPhoto, or sends an error via sendText.
func HandleScreenshot(
	ctx context.Context,
	sendPhoto SendPhotoFunc,
	sendText SendTextFunc,
	capturer ScreenshotCapturer,
) error {
	if capturer == nil {
		if sendText != nil {
			return sendText(ctx, "Not connected to Antigravity.")
		}
		return nil
	}

	result, err := capturer.Capture(ctx, "png")
	if err != nil {
		if sendText != nil {
			return sendText(ctx, fmt.Sprintf("Screenshot error: %s", err.Error()))
		}
		return nil
	}

	if result.Success && len(result.Buffer) > 0 {
		if sendPhoto != nil {
			return sendPhoto(ctx, "screenshot.png", result.Buffer, "📸 Screenshot")
		}
		return nil
	}

	if sendText != nil {
		return sendText(ctx, fmt.Sprintf("Screenshot failed: %s", result.Error))
	}
	return nil
}
