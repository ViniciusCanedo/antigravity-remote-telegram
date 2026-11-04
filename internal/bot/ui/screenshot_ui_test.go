package ui

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

type mockScreenshotCapturer struct {
	result ScreenshotResult
	err    error
}

func (m *mockScreenshotCapturer) Capture(ctx context.Context, format string) (ScreenshotResult, error) {
	return m.result, m.err
}

func TestHandleScreenshot_NilCdp(t *testing.T) {
	ctx := context.Background()
	photoCalled := false
	var sentText string

	sendPhoto := func(ctx context.Context, filename string, data []byte, caption string) error {
		photoCalled = true
		return nil
	}
	sendText := func(ctx context.Context, text string) error {
		sentText = text
		return nil
	}

	err := HandleScreenshot(ctx, sendPhoto, sendText, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if photoCalled {
		t.Error("expected sendPhoto not to be called")
	}
	if sentText != "Not connected to Antigravity." {
		t.Errorf("expected 'Not connected to Antigravity.', got %q", sentText)
	}
}

func TestHandleScreenshot_Success(t *testing.T) {
	ctx := context.Background()
	mockBuf := []byte("fake-png")
	capturer := &mockScreenshotCapturer{
		result: ScreenshotResult{
			Success: true,
			Buffer:  mockBuf,
		},
	}

	photoCalled := false
	var sentFilename, sentCaption string
	var sentData []byte
	textCalled := false

	sendPhoto := func(ctx context.Context, filename string, data []byte, caption string) error {
		photoCalled = true
		sentFilename = filename
		sentData = data
		sentCaption = caption
		return nil
	}
	sendText := func(ctx context.Context, text string) error {
		textCalled = true
		return nil
	}

	err := HandleScreenshot(ctx, sendPhoto, sendText, capturer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !photoCalled {
		t.Error("expected sendPhoto to be called")
	}
	if textCalled {
		t.Error("expected sendText not to be called")
	}
	if sentFilename != "screenshot.png" {
		t.Errorf("expected filename 'screenshot.png', got %q", sentFilename)
	}
	if sentCaption != "📸 Screenshot" {
		t.Errorf("expected caption '📸 Screenshot', got %q", sentCaption)
	}
	if !bytes.Equal(sentData, mockBuf) {
		t.Errorf("expected buffer %v, got %v", mockBuf, sentData)
	}
}

func TestHandleScreenshot_CaptureFailed(t *testing.T) {
	ctx := context.Background()
	capturer := &mockScreenshotCapturer{
		result: ScreenshotResult{
			Success: false,
			Error:   "No page found",
		},
	}

	photoCalled := false
	var sentText string

	sendPhoto := func(ctx context.Context, filename string, data []byte, caption string) error {
		photoCalled = true
		return nil
	}
	sendText := func(ctx context.Context, text string) error {
		sentText = text
		return nil
	}

	err := HandleScreenshot(ctx, sendPhoto, sendText, capturer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if photoCalled {
		t.Error("expected sendPhoto not to be called")
	}
	if sentText != "Screenshot failed: No page found" {
		t.Errorf("expected 'Screenshot failed: No page found', got %q", sentText)
	}
}

func TestHandleScreenshot_CaptureError(t *testing.T) {
	ctx := context.Background()
	capturer := &mockScreenshotCapturer{
		err: errors.New("CDP timeout"),
	}

	photoCalled := false
	var sentText string

	sendPhoto := func(ctx context.Context, filename string, data []byte, caption string) error {
		photoCalled = true
		return nil
	}
	sendText := func(ctx context.Context, text string) error {
		sentText = text
		return nil
	}

	err := HandleScreenshot(ctx, sendPhoto, sendText, capturer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if photoCalled {
		t.Error("expected sendPhoto not to be called")
	}
	if sentText != "Screenshot error: CDP timeout" {
		t.Errorf("expected 'Screenshot error: CDP timeout', got %q", sentText)
	}
}
