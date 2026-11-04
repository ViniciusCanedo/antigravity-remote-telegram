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

// PlanningInfo represents planning mode button and metadata extracted from the DOM.
// Corresponds to interface PlanningInfo in src/services/planningDetector.ts.
type PlanningInfo struct {
	// OpenText is the text of the Open button (e.g. "Open").
	OpenText string `json:"openText"`
	// ProceedText is the text of the Proceed button (e.g. "Proceed").
	ProceedText string `json:"proceedText"`
	// PlanTitle is the plan title (file name shown in the card).
	PlanTitle string `json:"planTitle"`
	// PlanSummary is the plan summary text extracted from span.text-sm.
	PlanSummary string `json:"planSummary"`
	// Description is the plan description with code/style blocks stripped.
	Description string `json:"description"`
}

// PlanningDetectorOptions configures a PlanningDetector instance.
// Corresponds to interface PlanningDetectorOptions in src/services/planningDetector.ts.
type PlanningDetectorOptions struct {
	// CdpService is the CDP connection evaluator instance.
	CdpService CdpEvaluator
	// PollInterval is the interval between polling cycles (default: 2000ms if <= 0).
	PollInterval time.Duration
	// OnPlanningRequired is invoked when planning buttons are detected.
	OnPlanningRequired func(info PlanningInfo)
	// OnResolved is an optional callback invoked when planning buttons disappear from the DOM.
	OnResolved func()
	// Logger is an optional slog.Logger (defaults to slog.Default() if nil).
	Logger *slog.Logger
	// NowFunc is an optional clock function for deterministic cooldown testing (defaults to time.Now if nil).
	NowFunc func() time.Time
}

const (
	// DefaultPlanningPollInterval is the default poll interval for PlanningDetector (2000ms).
	DefaultPlanningPollInterval = 2000 * time.Millisecond

	// PlanningCooldown is the cooldown period suppressing duplicate notifications (5000ms).
	PlanningCooldown = 5000 * time.Millisecond

	// DefaultOpenFallbackText is the fallback text for ClickOpenButton ("Open").
	DefaultOpenFallbackText = "Open"

	// DefaultProceedFallbackText is the fallback text for ClickProceedButton ("Proceed").
	DefaultProceedFallbackText = "Proceed"
)

// DetectPlanningScript detects planning buttons inside .notify-user-container.
// Source: src/services/planningDetector.ts lines 36-95.
const DetectPlanningScript = `(() => {
    const OPEN_PATTERNS = ['open'];
    const PROCEED_PATTERNS = ['proceed'];

    const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();

    // Find the notify container that holds planning UI
    const container = document.querySelector('.notify-user-container');
    if (!container) return null;

    const allButtons = Array.from(container.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    const openBtn = allButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return OPEN_PATTERNS.some(p => t === p || t.includes(p));
    }) || null;

    const proceedBtn = allButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return PROCEED_PATTERNS.some(p => t === p || t.includes(p));
    }) || null;

    // Both buttons must exist for this to be a planning UI
    if (!openBtn || !proceedBtn) return null;

    const openText = (openBtn.textContent || '').trim();
    const proceedText = (proceedBtn.textContent || '').trim();

    // Extract plan title from .inline-flex.break-all
    const titleEl = container.querySelector('span.inline-flex.break-all, .inline-flex.break-all');
    const planTitle = titleEl ? (titleEl.textContent || '').trim() : '';

    // Extract plan summary from span.text-sm (excluding buttons text)
    const summaryEls = Array.from(container.querySelectorAll('span.text-sm'));
    const planSummary = summaryEls
        .map(el => (el.textContent || '').trim())
        .filter(text => text.length > 0 && text !== openText && text !== proceedText)
        .join(' ');

    // Extract description from leading-relaxed container, skipping code/style blocks
    const descEl = container.querySelector('.leading-relaxed.select-text');
    let description = '';
    if (descEl) {
        const SKIP_TAGS = new Set(['PRE', 'CODE', 'STYLE', 'SCRIPT']);
        const parts = [];
        const walk = (node) => {
            if (node.nodeType === 3) {
                const t = node.textContent || '';
                if (t.trim()) parts.push(t.trim());
            } else if (node.nodeType === 1 && !SKIP_TAGS.has(node.tagName)) {
                for (const child of node.childNodes) walk(child);
            }
        };
        walk(descEl);
        description = parts.join(' ').slice(0, 500);
    }

    return { openText, proceedText, planTitle, planSummary, description };
})()`

// ExtractPlanContentScript extracts rendered plan content as markdown.
// Source: src/services/planningDetector.ts lines 103-164.
const ExtractPlanContentScript = `(() => {
    // Simple HTML-to-Markdown converter for plan content
    const htmlToMd = (el) => {
        const parts = [];
        const process = (node) => {
            if (node.nodeType === 3) {
                parts.push(node.textContent || '');
                return;
            }
            if (node.nodeType !== 1) return;
            const tag = node.tagName;
            if (tag === 'H1') { parts.push('\n# '); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'H2') { parts.push('\n## '); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'H3') { parts.push('\n### '); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'H4') { parts.push('\n#### '); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'STRONG' || tag === 'B') { parts.push('**'); node.childNodes.forEach(process); parts.push('**'); return; }
            if (tag === 'EM' || tag === 'I') { parts.push('*'); node.childNodes.forEach(process); parts.push('*'); return; }
            if (tag === 'PRE') {
                const code = node.querySelector('code');
                const text = code ? (code.textContent || '') : (node.textContent || '');
                parts.push('\n` + "```" + `\n' + text + '\n` + "```" + `\n');
                return;
            }
            if (tag === 'CODE') { parts.push('` + "`" + `' + (node.textContent || '') + '` + "`" + `'); return; }
            if (tag === 'A') {
                const href = node.getAttribute('href') || '';
                parts.push('['); node.childNodes.forEach(process); parts.push('](' + href + ')');
                return;
            }
            if (tag === 'LI') { parts.push('\n- '); node.childNodes.forEach(process); return; }
            if (tag === 'BR') { parts.push('\n'); return; }
            if (tag === 'P') { parts.push('\n\n'); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'UL' || tag === 'OL') { node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'STYLE' || tag === 'SCRIPT') return;
            node.childNodes.forEach(process);
        };
        process(el);
        return parts.join('').replace(/\n{3,}/g, '\n\n').trim();
    };

    // Primary selector: plan content container
    const contentContainer = document.querySelector(
        'div.relative.pl-4.pr-4.py-1, div.relative.pl-4.pr-4'
    );
    if (contentContainer) {
        const textEl = contentContainer.querySelector('.leading-relaxed.select-text');
        if (textEl) {
            return htmlToMd(textEl);
        }
    }

    // Fallback: any leading-relaxed.select-text with significant content
    const allLeading = Array.from(document.querySelectorAll('.leading-relaxed.select-text'));
    for (const el of allLeading) {
        const md = htmlToMd(el);
        if (md.length > 100) {
            return md;
        }
    }

    return null;
})()`

// PlanningDetector detects planning mode buttons (Open/Proceed) in the Antigravity UI via polling.
// Source: src/services/planningDetector.ts lines 174-353.
type PlanningDetector struct {
	cdpService         CdpEvaluator
	pollInterval       time.Duration
	onPlanningRequired func(info PlanningInfo)
	onResolved         func()
	logger             *slog.Logger
	nowFunc            func() time.Time

	mu               sync.RWMutex
	isRunning        bool
	cancelPoll       context.CancelFunc
	pollDone         chan struct{}
	lastDetectedKey  *string
	lastDetectedInfo *PlanningInfo
	lastNotifiedAt   time.Time
}

// NewPlanningDetector constructs a new PlanningDetector.
func NewPlanningDetector(options PlanningDetectorOptions) *PlanningDetector {
	pollInterval := options.PollInterval
	if pollInterval <= 0 {
		pollInterval = DefaultPlanningPollInterval
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	nowFunc := options.NowFunc
	if nowFunc == nil {
		nowFunc = time.Now
	}

	return &PlanningDetector{
		cdpService:         options.CdpService,
		pollInterval:       pollInterval,
		onPlanningRequired: options.OnPlanningRequired,
		onResolved:         options.OnResolved,
		logger:             logger,
		nowFunc:            nowFunc,
	}
}

// Start begins background monitoring.
func (d *PlanningDetector) Start() {
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
func (d *PlanningDetector) Stop() error {
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
func (d *PlanningDetector) IsActive() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isRunning
}

// GetLastDetectedInfo returns a copy of the last detected PlanningInfo, or nil if none.
func (d *PlanningDetector) GetLastDetectedInfo() *PlanningInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.lastDetectedInfo == nil {
		return nil
	}
	infoCopy := *d.lastDetectedInfo
	return &infoCopy
}

// ClickOpenButton clicks the Open button via CDP.
func (d *PlanningDetector) ClickOpenButton(ctx context.Context, buttonText ...string) (bool, error) {
	text := ""
	if len(buttonText) > 0 && buttonText[0] != "" {
		text = buttonText[0]
	} else if last := d.GetLastDetectedInfo(); last != nil && last.OpenText != "" {
		text = last.OpenText
	} else {
		text = DefaultOpenFallbackText
	}
	return d.clickButton(ctx, text), nil
}

// ClickProceedButton clicks the Proceed button via CDP.
func (d *PlanningDetector) ClickProceedButton(ctx context.Context, buttonText ...string) (bool, error) {
	text := ""
	if len(buttonText) > 0 && buttonText[0] != "" {
		text = buttonText[0]
	} else if last := d.GetLastDetectedInfo(); last != nil && last.ProceedText != "" {
		text = last.ProceedText
	} else {
		text = DefaultProceedFallbackText
	}
	return d.clickButton(ctx, text), nil
}

// ExtractPlanContent extracts plan markdown from the DOM after Open has been clicked.
func (d *PlanningDetector) ExtractPlanContent(ctx context.Context) (*string, error) {
	val, err := d.runEvaluateScript(ctx, ExtractPlanContentScript)
	if err != nil {
		d.logger.Error("[PlanningDetector] Error extracting plan content", "error", err)
		return nil, nil
	}
	if len(val) == 0 || string(val) == "null" {
		return nil, nil
	}
	var content string
	if err := json.Unmarshal(val, &content); err != nil {
		return nil, nil
	}
	return &content, nil
}

// PollOnce executes a single polling iteration against the CDP session.
func (d *PlanningDetector) PollOnce(ctx context.Context) {
	val, err := d.runEvaluateScript(ctx, DetectPlanningScript)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "WebSocket is not connected") || strings.Contains(msg, "WebSocket disconnected") {
			return
		}
		d.logger.Error("[PlanningDetector] Error during polling", "error", err)
		return
	}

	var info *PlanningInfo
	if len(val) > 0 && string(val) != "null" {
		var parsed PlanningInfo
		if err := json.Unmarshal(val, &parsed); err == nil {
			info = &parsed
		}
	}

	if info != nil {
		key := fmt.Sprintf("%s::%s", info.OpenText, info.ProceedText)
		now := d.nowFunc()

		var shouldNotify bool
		d.mu.Lock()
		withinCooldown := !d.lastNotifiedAt.IsZero() && now.Sub(d.lastNotifiedAt) < PlanningCooldown
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

		if shouldNotify && d.onPlanningRequired != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						d.logger.Error("[PlanningDetector] onPlanningRequired callback panicked", "panic", r)
					}
				}()
				d.onPlanningRequired(*info)
			}()
		}
	} else {
		var wasDetected bool
		d.mu.Lock()
		wasDetected = d.lastDetectedKey != nil
		d.lastDetectedKey = nil
		d.lastDetectedInfo = nil
		// Note: d.lastNotifiedAt is preserved across disappearances to enforce 5s cooldown
		d.mu.Unlock()

		if wasDetected && d.onResolved != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						d.logger.Error("[PlanningDetector] onResolved callback panicked", "panic", r)
					}
				}()
				d.onResolved()
			}()
		}
	}
}

// clickButton generates BuildClickScript for buttonText and executes it via CDP.
func (d *PlanningDetector) clickButton(ctx context.Context, buttonText string) bool {
	val, err := d.runEvaluateScript(ctx, BuildClickScript(buttonText))
	if err != nil {
		d.logger.Error("[PlanningDetector] Error while clicking button", "error", err)
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
func (d *PlanningDetector) runEvaluateScript(ctx context.Context, expression string) (json.RawMessage, error) {
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
