package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Constants for ErrorPopupDetector.
const (
	// DefaultErrorPopupPollInterval is the default poll interval for ErrorPopupDetector (3000ms).
	DefaultErrorPopupPollInterval = 3000 * time.Millisecond

	// ErrorPopupCooldown is the cooldown period suppressing duplicate notifications (10000ms).
	ErrorPopupCooldown = 10000 * time.Millisecond

	// ErrorPopupBodyKeyMaxLen is the maximum length of the body snippet used for deduplication key.
	ErrorPopupBodyKeyMaxLen = 100

	// ErrorPopupTelegramBodyMaxLen is the maximum character limit for body text sent to Telegram.
	ErrorPopupTelegramBodyMaxLen = 3800

	// Button labels clicked by ErrorPopupDetector.
	DismissButtonText       = "Dismiss"
	CopyDebugInfoButtonText = "Copy debug info"
	RetryButtonText         = "Retry"
)

// DetectErrorPopupScript searches for dialog/modal containers containing error patterns in the Antigravity UI.
// Source: src/services/errorPopupDetector.ts lines 32-94.
const DetectErrorPopupScript = `(() => {
    const ERROR_PATTERNS = [
        'agent terminated',
        'terminated due to error',
        'unexpected error',
        'something went wrong',
        'an error occurred',
    ];

    const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();

    // Try dialog/modal first
    const dialogs = Array.from(document.querySelectorAll(
        '[role="dialog"], [role="alertdialog"], .modal, .dialog'
    )).filter(el => el.offsetParent !== null || el.getAttribute('aria-modal') === 'true');

    // Fallback: look for fixed/absolute positioned overlays
    if (dialogs.length === 0) {
        const overlays = Array.from(document.querySelectorAll('div[class*="fixed"], div[class*="absolute"]'))
            .filter(el => {
                const style = window.getComputedStyle(el);
                return (style.position === 'fixed' || style.position === 'absolute')
                    && style.zIndex && parseInt(style.zIndex, 10) > 10
                    && el.querySelector('button');
            });
        dialogs.push(...overlays);
    }

    for (const dialog of dialogs) {
        const fullText = normalize(dialog.textContent || '');
        const isError = ERROR_PATTERNS.some(p => fullText.includes(p));
        if (!isError) continue;

        // Extract title from heading elements or first prominent text
        const headingEl = dialog.querySelector('h1, h2, h3, h4, [class*="title"], [class*="heading"]');
        const title = headingEl ? (headingEl.textContent || '').trim() : '';

        // Extract body text (excluding button text and title)
        const allButtons = Array.from(dialog.querySelectorAll('button'))
            .filter(btn => btn.offsetParent !== null);
        const buttonTexts = new Set(allButtons.map(btn => (btn.textContent || '').trim()));

        const bodyParts = [];
        const walker = document.createTreeWalker(dialog, NodeFilter.SHOW_TEXT);
        let node;
        while ((node = walker.nextNode())) {
            const text = (node.textContent || '').trim();
            if (!text) continue;
            if (buttonTexts.has(text)) continue;
            if (text === title) continue;
            bodyParts.push(text);
        }
        const body = bodyParts.join(' ').slice(0, 1000);

        const buttons = allButtons.map(btn => (btn.textContent || '').trim()).filter(t => t.length > 0);

        if (buttons.length === 0) continue;

        return { title: title || 'Error', body, buttons };
    }

    return null;
})()`

// ReadClipboardScript reads clipboard content asynchronously via navigator.clipboard.readText().
// Requires awaitPromise=true since clipboard API returns a Promise.
// Source: src/services/errorPopupDetector.ts lines 100-107.
const ReadClipboardScript = `(async () => {
    try {
        const text = await navigator.clipboard.readText();
        return text || null;
    } catch (e) {
        return null;
    }
})()`

// ErrorPopupInfo represents error popup information extracted from the DOM.
// Source: src/services/errorPopupDetector.ts lines 6-13.
type ErrorPopupInfo struct {
	// Title is the error popup title text.
	Title string `json:"title"`
	// Body is the error popup body/description text.
	Body string `json:"body"`
	// Buttons contains the visible button labels found in the popup.
	Buttons []string `json:"buttons"`
}

// ErrorPopupDetectorOptions configures an ErrorPopupDetector instance.
// Source: src/services/errorPopupDetector.ts lines 15-24.
type ErrorPopupDetectorOptions struct {
	// CdpService is the CDP connection evaluator instance.
	CdpService CdpEvaluator
	// PollInterval is the interval between polling cycles (default: 3000ms if <= 0).
	PollInterval time.Duration
	// OnErrorPopup is invoked when an error popup is detected.
	OnErrorPopup func(info ErrorPopupInfo)
	// OnResolved is an optional callback invoked when the popup disappears from the DOM.
	OnResolved func()
	// Logger is an optional slog.Logger (defaults to slog.Default() if nil).
	Logger *slog.Logger
	// NowFunc is an optional clock function for deterministic cooldown testing (defaults to time.Now if nil).
	NowFunc func() time.Time
}

// ErrorPopupDetector detects error popup dialogs in the Antigravity UI via polling.
// Source: src/services/errorPopupDetector.ts lines 119-303.
type ErrorPopupDetector struct {
	cdpService   CdpEvaluator
	pollInterval time.Duration
	onErrorPopup func(info ErrorPopupInfo)
	onResolved   func()
	logger       *slog.Logger
	nowFunc      func() time.Time

	mu               sync.RWMutex
	isRunning        bool
	cancelPoll       context.CancelFunc
	pollDone         chan struct{}
	lastDetectedKey  *string
	lastDetectedInfo *ErrorPopupInfo
	lastNotifiedAt   time.Time
}

// NewErrorPopupDetector constructs a new ErrorPopupDetector.
func NewErrorPopupDetector(options ErrorPopupDetectorOptions) *ErrorPopupDetector {
	pollInterval := options.PollInterval
	if pollInterval <= 0 {
		pollInterval = DefaultErrorPopupPollInterval
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	nowFunc := options.NowFunc
	if nowFunc == nil {
		nowFunc = time.Now
	}

	return &ErrorPopupDetector{
		cdpService:   options.CdpService,
		pollInterval: pollInterval,
		onErrorPopup: options.OnErrorPopup,
		onResolved:   options.OnResolved,
		logger:       logger,
		nowFunc:      nowFunc,
	}
}

// Start begins background monitoring.
func (d *ErrorPopupDetector) Start() {
	d.mu.Lock()
	if d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = true
	d.lastDetectedKey = nil
	d.lastDetectedInfo = nil
	d.lastNotifiedAt = time.Time{}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancelPoll = cancel
	done := make(chan struct{})
	d.pollDone = done
	d.mu.Unlock()

	go func() {
		defer close(done)
		timer := time.NewTimer(d.pollInterval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if !d.IsActive() {
					return
				}
				d.PollOnce(ctx)
				if !d.IsActive() {
					return
				}
				timer.Reset(d.pollInterval)
			}
		}
	}()
}

// Stop stops the background monitoring goroutine.
func (d *ErrorPopupDetector) Stop() error {
	d.mu.Lock()
	if !d.isRunning {
		d.mu.Unlock()
		return nil
	}
	d.isRunning = false
	cancel := d.cancelPoll
	done := d.pollDone
	d.cancelPoll = nil
	d.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	return nil
}

// IsActive returns whether monitoring is currently running.
func (d *ErrorPopupDetector) IsActive() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isRunning
}

// GetLastDetectedInfo returns a copy of the last detected ErrorPopupInfo, or nil if none.
func (d *ErrorPopupDetector) GetLastDetectedInfo() *ErrorPopupInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.lastDetectedInfo == nil {
		return nil
	}
	infoCopy := *d.lastDetectedInfo
	return &infoCopy
}

// ClickDismissButton clicks the Dismiss button via CDP.
func (d *ErrorPopupDetector) ClickDismissButton(ctx context.Context) (bool, error) {
	return d.clickButton(ctx, DismissButtonText), nil
}

// ClickCopyDebugInfoButton clicks the "Copy debug info" button via CDP.
func (d *ErrorPopupDetector) ClickCopyDebugInfoButton(ctx context.Context) (bool, error) {
	return d.clickButton(ctx, CopyDebugInfoButtonText), nil
}

// ClickRetryButton clicks the Retry button via CDP.
func (d *ErrorPopupDetector) ClickRetryButton(ctx context.Context) (bool, error) {
	return d.clickButton(ctx, RetryButtonText), nil
}

// ReadClipboard reads clipboard content from the browser via navigator.clipboard.readText().
// Requires awaitPromise=true. Returns nil if content is unavailable or on CDP error.
func (d *ErrorPopupDetector) ReadClipboard(ctx context.Context) (*string, error) {
	val, err := d.runEvaluateScript(ctx, ReadClipboardScript, true)
	if err != nil {
		d.logger.Error("[ErrorPopupDetector] Error reading clipboard", "error", err)
		return nil, nil
	}
	if len(val) == 0 || string(val) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(val, &text); err != nil {
		return nil, nil
	}
	return &text, nil
}

// PollOnce executes a single polling iteration against the CDP session.
func (d *ErrorPopupDetector) PollOnce(ctx context.Context) {
	val, err := d.runEvaluateScript(ctx, DetectErrorPopupScript, false)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "WebSocket is not connected") || strings.Contains(msg, "WebSocket disconnected") {
			return
		}
		d.logger.Error("[ErrorPopupDetector] Error during polling", "error", err)
		return
	}

	var info *ErrorPopupInfo
	if len(val) > 0 && string(val) != "null" {
		var parsed ErrorPopupInfo
		if err := json.Unmarshal(val, &parsed); err == nil {
			info = &parsed
		}
	}

	if info != nil {
		bodyPrefix := info.Body
		runes := []rune(bodyPrefix)
		if len(runes) > ErrorPopupBodyKeyMaxLen {
			bodyPrefix = string(runes[:ErrorPopupBodyKeyMaxLen])
		}
		key := fmt.Sprintf("%s::%s", info.Title, bodyPrefix)
		now := d.nowFunc()

		var shouldNotify bool
		d.mu.Lock()
		withinCooldown := !d.lastNotifiedAt.IsZero() && now.Sub(d.lastNotifiedAt) < ErrorPopupCooldown
		if (d.lastDetectedKey == nil || *d.lastDetectedKey != key) && !withinCooldown {
			d.lastDetectedKey = &key
			infoCopy := *info
			d.lastDetectedInfo = &infoCopy
			d.lastNotifiedAt = now
			shouldNotify = true
		} else if d.lastDetectedKey != nil && *d.lastDetectedKey == key {
			infoCopy := *info
			d.lastDetectedInfo = &infoCopy
		}
		d.mu.Unlock()

		if shouldNotify && d.onErrorPopup != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						d.logger.Error("[ErrorPopupDetector] onErrorPopup callback panicked", "panic", r)
					}
				}()
				d.onErrorPopup(*info)
			}()
		}
	} else {
		var wasDetected bool
		d.mu.Lock()
		wasDetected = d.lastDetectedKey != nil
		d.lastDetectedKey = nil
		d.lastDetectedInfo = nil
		// Note: d.lastNotifiedAt is preserved across disappearances to enforce 10s cooldown
		d.mu.Unlock()

		if wasDetected && d.onResolved != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						d.logger.Error("[ErrorPopupDetector] onResolved callback panicked", "panic", r)
					}
				}()
				d.onResolved()
			}()
		}
	}
}

// clickButton generates BuildClickScript for buttonText and executes it via CDP.
func (d *ErrorPopupDetector) clickButton(ctx context.Context, buttonText string) bool {
	val, err := d.runEvaluateScript(ctx, BuildClickScript(buttonText), false)
	if err != nil {
		d.logger.Error("[ErrorPopupDetector] Error while clicking button", "error", err)
		return false
	}
	if len(val) == 0 || string(val) == "null" {
		return false
	}
	var res clickScriptResult
	if err := json.Unmarshal(val, &res); err != nil {
		return false
	}
	return res.OK
}

// runEvaluateScript evaluates expression in the primary execution context and returns result.value.
func (d *ErrorPopupDetector) runEvaluateScript(ctx context.Context, expression string, awaitPromise bool) (json.RawMessage, error) {
	contextID := d.cdpService.GetPrimaryContextId()
	callParams := map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  awaitPromise,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	raw, err := d.cdpService.Call(ctx, "Runtime.evaluate", callParams)
	if err != nil {
		return nil, err
	}

	var resp runtimeEvaluateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return resp.Result.Value, nil
}
