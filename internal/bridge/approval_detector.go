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

// CdpEvaluator abstracts CDP evaluation operations required by background detectors.
// Corresponds to cdpService.call('Runtime.evaluate', ...) and getPrimaryContextId().
type CdpEvaluator interface {
	// Call executes an arbitrary CDP command (e.g. "Runtime.evaluate") and returns the raw JSON payload.
	Call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error)
	// GetPrimaryContextId returns the execution context ID of the primary Antigravity window,
	// or nil if no primary context is registered (in which case contextId is omitted).
	GetPrimaryContextId() *int64
}

// runtimeEvaluateResponse represents the standard Chrome DevTools Protocol response envelope
// for Runtime.evaluate calls with returnByValue: true.
type runtimeEvaluateResponse struct {
	Result struct {
		Value json.RawMessage `json:"value"`
	} `json:"result"`
}

// clickScriptResult represents the return value of BuildClickScript and ExpandAlwaysAllowMenuScript.
type clickScriptResult struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ApprovalInfo represents approval button information extracted from the DOM.
// Corresponds to interface ApprovalInfo in src/services/approvalDetector.ts.
type ApprovalInfo struct {
	// ApproveText is the text of the approval button (e.g. "Allow", "Allow Once", "許可").
	ApproveText string `json:"approveText"`
	// AlwaysAllowText is the text of the per-conversation allow button (e.g. "Allow This Conversation").
	// Empty string when not directly visible in the DOM.
	AlwaysAllowText string `json:"alwaysAllowText,omitempty"`
	// DenyText is the text of the denial button (e.g. "Deny", "拒否").
	DenyText string `json:"denyText"`
	// Description is the description of the requested action (e.g. "write to file.ts").
	Description string `json:"description"`
}

// ApprovalDetectorOptions configures an ApprovalDetector instance.
// Corresponds to interface ApprovalDetectorOptions in src/services/approvalDetector.ts.
type ApprovalDetectorOptions struct {
	// CdpService is the CDP connection evaluator instance.
	CdpService CdpEvaluator
	// PollInterval is the interval between polling cycles (default: 1500ms if <= 0).
	PollInterval time.Duration
	// OnApprovalRequired is invoked when a new approval button is detected.
	OnApprovalRequired func(info ApprovalInfo)
	// OnResolved is an optional callback invoked when a previously detected approval disappears from the DOM.
	OnResolved func()
	// Logger is an optional slog.Logger (defaults to slog.Default() if nil).
	Logger *slog.Logger
	// SleepFunc is an optional deterministic sleep function for tests (defaults to context-aware timer sleep if nil).
	SleepFunc func(ctx context.Context, d time.Duration) error
}

const (
	// DefaultApprovalPollInterval is the default poll interval for ApprovalDetector (1500ms).
	DefaultApprovalPollInterval = 1500 * time.Millisecond

	// DefaultBridgeApprovalPollInterval is the poll interval when instantiated by CdpBridgeManager (2000ms).
	DefaultBridgeApprovalPollInterval = 2000 * time.Millisecond

	// AlwaysAllowExpandMaxRetries is the number of retry iterations after expanding the dropdown menu (5).
	AlwaysAllowExpandMaxRetries = 5

	// AlwaysAllowExpandRetryDelay is the wait duration between retries after dropdown expansion (120ms).
	AlwaysAllowExpandRetryDelay = 120 * time.Millisecond

	// DefaultApproveFallbackText is the fallback text for ApproveButton when no argument or prior detection exists ("Allow").
	DefaultApproveFallbackText = "Allow"

	// DefaultDenyFallbackText is the fallback text for DenyButton when no argument or prior detection exists ("Deny").
	DefaultDenyFallbackText = "Deny"
)

// DefaultAlwaysAllowCandidates defines the candidate button labels tried in order by AlwaysAllowButton().
var DefaultAlwaysAllowCandidates = []string{
	"Allow This Conversation",
	"Allow This Chat",
	"この会話を許可",
	"Always Allow",
	"常に許可",
}

// DetectApprovalScript is the DOM evaluation script that detects approval dialog buttons.
// Source: src/services/approvalDetector.ts lines 32-121.
const DetectApprovalScript = `(() => {
    const ALLOW_ONCE_PATTERNS = ['allow once', 'allow one time', '今回のみ許可', '1回のみ許可', '一度許可'];
    const ALWAYS_ALLOW_PATTERNS = [
        'allow this conversation',
        'allow this chat',
        'always allow',
        '常に許可',
        'この会話を許可',
    ];
    const ALLOW_PATTERNS = ['allow', 'permit', '許可', '承認', '確認'];
    const DENY_PATTERNS = ['deny', '拒否', 'decline'];

    const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();

    const allButtons = Array.from(document.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    let approveBtn = allButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return ALLOW_ONCE_PATTERNS.some(p => t.includes(p));
    }) || null;

    if (!approveBtn) {
        approveBtn = allButtons.find(btn => {
            const t = normalize(btn.textContent || '');
            const isAlways = ALWAYS_ALLOW_PATTERNS.some(p => t.includes(p));
            return !isAlways && ALLOW_PATTERNS.some(p => t.includes(p));
        }) || null;
    }

    if (!approveBtn) return null;

    const container = approveBtn.closest('[role="dialog"], .modal, .dialog, .approval-container, .permission-dialog')
        || approveBtn.parentElement?.parentElement
        || approveBtn.parentElement
        || document.body;

    const containerButtons = Array.from(container.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    const denyBtn = containerButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return DENY_PATTERNS.some(p => t.includes(p));
    }) || null;

    if (!denyBtn) return null;

    const alwaysAllowBtn = containerButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return ALWAYS_ALLOW_PATTERNS.some(p => t.includes(p));
    }) || null;

    const approveText = (approveBtn.textContent || '').trim();
    const alwaysAllowText = alwaysAllowBtn ? (alwaysAllowBtn.textContent || '').trim() : '';
    const denyText = (denyBtn.textContent || '').trim();

    // Description extraction (multiple fallbacks)
    let description = '';

    // 1. p or .description inside dialog/modal
    const dialog = container;
    if (dialog) {
        const descEl = dialog.querySelector('p, .description, [data-testid="description"]');
        if (descEl) {
            description = (descEl.textContent || '').trim();
        }
    }

    // 2. Parent element text (excluding button text)
    if (!description) {
        const parent = approveBtn.parentElement?.parentElement || approveBtn.parentElement;
        if (parent) {
            const clone = parent.cloneNode(true);
            const buttons = clone.querySelectorAll('button');
            buttons.forEach(b => b.remove());
            const parentText = (clone.textContent || '').trim();
            if (parentText.length > 5 && parentText.length < 500) {
                description = parentText;
            }
        }
    }

    // 3. aria-label fallback
    if (!description) {
        const ariaLabel = approveBtn.getAttribute('aria-label') || '';
        if (ariaLabel) description = ariaLabel;
    }

    return { approveText, alwaysAllowText, denyText, description };
})()`

// ExpandAlwaysAllowMenuScript presses the toggle chevron on the right side of Allow Once to expand Always Allow.
// Source: src/services/approvalDetector.ts lines 126-194.
const ExpandAlwaysAllowMenuScript = `(() => {
    const ALLOW_ONCE_PATTERNS = ['allow once', 'allow one time', '今回のみ許可', '1回のみ許可', '一度許可'];
    const ALWAYS_ALLOW_PATTERNS = [
        'allow this conversation',
        'allow this chat',
        'always allow',
        '常に許可',
        'この会話を許可',
    ];

    const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();
    const visibleButtons = Array.from(document.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    const directAlways = visibleButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return ALWAYS_ALLOW_PATTERNS.some(p => t.includes(p));
    });
    if (directAlways) return { ok: true, reason: 'already-visible' };

    const allowOnceBtn = visibleButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return ALLOW_ONCE_PATTERNS.some(p => t.includes(p));
    });
    if (!allowOnceBtn) return { ok: false, error: 'allow-once button not found' };

    const container = allowOnceBtn.closest('[role="dialog"], .modal, .dialog, .approval-container, .permission-dialog')
        || allowOnceBtn.parentElement?.parentElement
        || allowOnceBtn.parentElement
        || document.body;

    const containerButtons = Array.from(container.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    const toggleBtn = containerButtons.find(btn => {
        if (btn === allowOnceBtn) return false;
        const text = normalize(btn.textContent || '');
        const aria = normalize(btn.getAttribute('aria-label') || '');
        const hasPopup = btn.getAttribute('aria-haspopup');
        if (hasPopup === 'menu' || hasPopup === 'listbox') return true;
        if (text === '') return true;
        return /menu|more|expand|options|dropdown|chevron|arrow/.test(aria);
    });

    if (toggleBtn) {
        toggleBtn.click();
        return { ok: true, reason: 'toggle-button' };
    }

    const rect = allowOnceBtn.getBoundingClientRect();
    if (!rect || rect.width <= 0 || rect.height <= 0) {
        return { ok: false, error: 'allow-once button rect unavailable' };
    }

    const clickX = rect.right - Math.max(4, Math.min(12, rect.width * 0.15));
    const clickY = rect.top + rect.height / 2;

    const events = ['pointerdown', 'mousedown', 'mouseup', 'click'];
    for (const type of events) {
        allowOnceBtn.dispatchEvent(new MouseEvent(type, {
            bubbles: true,
            cancelable: true,
            view: window,
            clientX: clickX,
            clientY: clickY,
        }));
    }
    return { ok: true, reason: 'allow-once-right-edge' };
})()`

// BuildClickScript generates a CDP IIFE script that locates a visible button by text or aria-label and clicks it.
// Uses json.Marshal to escape buttonText identically to JSON.stringify(buttonText).
// Source: src/services/approvalDetector.ts lines 201-221.
func BuildClickScript(buttonText string) string {
	safeBytes, _ := json.Marshal(buttonText)
	safeText := string(safeBytes)
	return `(() => {
        const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();
        const text = ` + safeText + `;
        const wanted = normalize(text);
        const allButtons = Array.from(document.querySelectorAll('button'));
        const target = allButtons.find(btn => {
            if (!btn.offsetParent) return false;
            const buttonText = normalize(btn.textContent || '');
            const ariaLabel = normalize(btn.getAttribute('aria-label') || '');
            return buttonText === wanted ||
                ariaLabel === wanted ||
                buttonText.includes(wanted) ||
                ariaLabel.includes(wanted);
        });
        if (!target) return { ok: false, error: 'Button not found: ' + text };
        target.click();
        return { ok: true };
    })()`
}

// ApprovalDetector detects approval buttons in the Antigravity UI via polling.
// Source: src/services/approvalDetector.ts lines 229-428.
type ApprovalDetector struct {
	cdpService         CdpEvaluator
	pollInterval       time.Duration
	onApprovalRequired func(info ApprovalInfo)
	onResolved         func()
	logger             *slog.Logger
	sleepFunc          func(ctx context.Context, d time.Duration) error

	mu               sync.RWMutex
	isRunning        bool
	cancelPoll       context.CancelFunc
	pollDone         chan struct{}
	lastDetectedKey  *string
	lastDetectedInfo *ApprovalInfo
}

// NewApprovalDetector constructs a new ApprovalDetector.
func NewApprovalDetector(options ApprovalDetectorOptions) *ApprovalDetector {
	pollInterval := options.PollInterval
	if pollInterval <= 0 {
		pollInterval = DefaultApprovalPollInterval
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sleepFunc := options.SleepFunc
	if sleepFunc == nil {
		sleepFunc = func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}

	return &ApprovalDetector{
		cdpService:         options.CdpService,
		pollInterval:       pollInterval,
		onApprovalRequired: options.OnApprovalRequired,
		onResolved:         options.OnResolved,
		logger:             logger,
		sleepFunc:          sleepFunc,
	}
}

// Start begins the background polling loop. It is safe to call multiple times.
func (d *ApprovalDetector) Start() {
	d.mu.Lock()
	if d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = true
	d.lastDetectedKey = nil
	d.lastDetectedInfo = nil

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

// Stop stops the background polling loop and waits for any in-flight poll to complete.
func (d *ApprovalDetector) Stop() error {
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
func (d *ApprovalDetector) IsActive() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isRunning
}

// GetLastDetectedInfo returns a copy of the last detected ApprovalInfo, or nil if none.
func (d *ApprovalDetector) GetLastDetectedInfo() *ApprovalInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.lastDetectedInfo == nil {
		return nil
	}
	infoCopy := *d.lastDetectedInfo
	return &infoCopy
}

// ApproveButton clicks the approval button with the given text (or detected approveText, or "Allow").
func (d *ApprovalDetector) ApproveButton(ctx context.Context, buttonText ...string) (bool, error) {
	text := ""
	if len(buttonText) > 0 && buttonText[0] != "" {
		text = buttonText[0]
	} else if last := d.GetLastDetectedInfo(); last != nil && last.ApproveText != "" {
		text = last.ApproveText
	} else {
		text = DefaultApproveFallbackText
	}
	return d.clickButton(ctx, text), nil
}

// DenyButton clicks the denial button with the given text (or detected denyText, or "Deny").
func (d *ApprovalDetector) DenyButton(ctx context.Context, buttonText ...string) (bool, error) {
	text := ""
	if len(buttonText) > 0 && buttonText[0] != "" {
		text = buttonText[0]
	} else if last := d.GetLastDetectedInfo(); last != nil && last.DenyText != "" {
		text = last.DenyText
	} else {
		text = DefaultDenyFallbackText
	}
	return d.clickButton(ctx, text), nil
}

// AlwaysAllowButton attempts to click "Allow This Conversation / Always Allow".
// If not directly visible, expands the Allow Once dropdown and retries up to 5 times.
func (d *ApprovalDetector) AlwaysAllowButton(ctx context.Context) (bool, error) {
	var directCandidates []string
	if last := d.GetLastDetectedInfo(); last != nil && strings.TrimSpace(last.AlwaysAllowText) != "" {
		directCandidates = append(directCandidates, last.AlwaysAllowText)
	}
	directCandidates = append(directCandidates, DefaultAlwaysAllowCandidates...)

	// Phase 1: Direct candidates attempt
	for _, candidate := range directCandidates {
		if d.clickButton(ctx, candidate) {
			return true, nil
		}
	}

	// Phase 2: Expand dropdown menu
	val, err := d.runEvaluateScript(ctx, ExpandAlwaysAllowMenuScript)
	if err != nil {
		return false, nil
	}
	var expanded clickScriptResult
	if err := json.Unmarshal(val, &expanded); err != nil || !expanded.OK {
		return false, nil
	}

	// Phase 3: Retry polling post-expansion
	for i := 0; i < AlwaysAllowExpandMaxRetries; i++ {
		for _, candidate := range directCandidates {
			if d.clickButton(ctx, candidate) {
				return true, nil
			}
		}
		_ = d.sleepFunc(ctx, AlwaysAllowExpandRetryDelay)
	}

	return false, nil
}

// PollOnce executes a single polling iteration against the CDP session.
func (d *ApprovalDetector) PollOnce(ctx context.Context) {
	val, err := d.runEvaluateScript(ctx, DetectApprovalScript)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "WebSocket is not connected") || strings.Contains(msg, "WebSocket disconnected") {
			return
		}
		d.logger.Error("[ApprovalDetector] Error during polling", "error", err)
		return
	}

	var info *ApprovalInfo
	if len(val) > 0 && string(val) != "null" {
		var parsed ApprovalInfo
		if err := json.Unmarshal(val, &parsed); err == nil {
			info = &parsed
		}
	}

	if info != nil {
		key := fmt.Sprintf("%s::%s", info.ApproveText, info.Description)
		var shouldNotify bool
		d.mu.Lock()
		if d.lastDetectedKey == nil || *d.lastDetectedKey != key {
			d.lastDetectedKey = &key
			infoCopy := *info
			d.lastDetectedInfo = &infoCopy
			shouldNotify = true
		}
		d.mu.Unlock()

		if shouldNotify && d.onApprovalRequired != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						d.logger.Error("[ApprovalDetector] onApprovalRequired callback panicked", "panic", r)
					}
				}()
				d.onApprovalRequired(*info)
			}()
		}
	} else {
		var wasDetected bool
		d.mu.Lock()
		wasDetected = d.lastDetectedKey != nil
		d.lastDetectedKey = nil
		d.lastDetectedInfo = nil
		d.mu.Unlock()

		if wasDetected && d.onResolved != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						d.logger.Error("[ApprovalDetector] onResolved callback panicked", "panic", r)
					}
				}()
				d.onResolved()
			}()
		}
	}
}

// clickButton generates BuildClickScript for buttonText and executes it via CDP.
func (d *ApprovalDetector) clickButton(ctx context.Context, buttonText string) bool {
	val, err := d.runEvaluateScript(ctx, BuildClickScript(buttonText))
	if err != nil {
		d.logger.Error("[ApprovalDetector] Error while clicking button", "error", err)
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
func (d *ApprovalDetector) runEvaluateScript(ctx context.Context, expression string) (json.RawMessage, error) {
	contextID := d.cdpService.GetPrimaryContextId()
	callParams := map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  false,
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
