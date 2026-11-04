package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Constants for UserMessageDetector.
const (
	// DefaultUserMessagePollInterval is the default poll interval for UserMessageDetector (2000ms).
	DefaultUserMessagePollInterval = 2000 * time.Millisecond

	// MaxSeenHashes is the maximum capacity of the FIFO seenHashes set (50).
	MaxSeenHashes = 50

	// DefaultEchoHashTTL is the default time-to-live for echo hashes (60s).
	DefaultEchoHashTTL = 60000 * time.Millisecond

	// NormalizeHashMaxLen is the maximum character limit for text normalization before hashing.
	NormalizeHashMaxLen = 200

	// EchoHashHexLen is the character length of the truncated hex SHA-256 digest (16).
	EchoHashHexLen = 16
)

// DetectUserMessageScript detects the latest user message in the Antigravity chat DOM.
// Source: src/services/userMessageDetector.ts lines 37-73.
const DetectUserMessageScript = `(() => {
    const panel = document.querySelector('.antigravity-agent-side-panel');
    const scope = panel || document;

    // Strategy A (primary): Query .whitespace-pre-wrap elements directly inside
    // user bubble containers. This avoids the parent-container problem where
    // querySelectorAll matches a wrapper that contains multiple bubbles.
    const textEls = scope.querySelectorAll(
        '[class*="bg-gray-500/15"][class*="select-text"] .whitespace-pre-wrap'
    );

    if (textEls.length > 0) {
        const lastTextEl = textEls[textEls.length - 1];
        const text = (lastTextEl.textContent || '').trim();
        if (text.length > 0) return { text };
    }

    // Strategy B (fallback): Find individual bubble containers, filtering out
    // any element that itself contains nested bubble elements (i.e., a parent wrapper).
    const userBubbles = Array.from(scope.querySelectorAll(
        '[class*="bg-gray-500/15"][class*="rounded-lg"][class*="select-text"]'
    )).filter(el => !el.querySelector('[class*="bg-gray-500/15"][class*="select-text"]'));

    if (userBubbles.length === 0) return null;

    const lastBubble = userBubbles[userBubbles.length - 1];
    const textEl = lastBubble.querySelector('.whitespace-pre-wrap')
        || lastBubble.querySelector('[style*="word-break"]');

    const text = textEl
        ? (textEl.textContent || '').trim()
        : (lastBubble.textContent || '').trim();

    if (!text || text.length < 1) return null;

    return { text };
})()`

// UserMessageInfo represents a user message detected from the DOM.
// Source: src/services/userMessageDetector.ts lines 6-9.
type UserMessageInfo struct {
	// Text is the message text content.
	Text string `json:"text"`
}

// UserMessageDetectorOptions configures a UserMessageDetector instance.
// Source: src/services/userMessageDetector.ts lines 11-18.
type UserMessageDetectorOptions struct {
	// CdpService is the CDP connection evaluator instance.
	CdpService CdpEvaluator
	// PollInterval is the interval between polling cycles (default: 2000ms if <= 0).
	PollInterval time.Duration
	// OnUserMessage is invoked when a new user message is detected.
	OnUserMessage func(info UserMessageInfo)
	// EchoTTL is an optional override for echo hash expiration (default: 60s if <= 0).
	EchoTTL time.Duration
	// Logger is an optional slog.Logger (defaults to slog.Default() if nil).
	Logger *slog.Logger
}

// UserMessageDetector detects user messages posted directly in the Antigravity UI.
// Source: src/services/userMessageDetector.ts lines 94-249.
type UserMessageDetector struct {
	cdpService    CdpEvaluator
	pollInterval  time.Duration
	onUserMessage func(info UserMessageInfo)
	echoTTL       time.Duration
	logger        *slog.Logger

	mu               sync.RWMutex
	isRunning        bool
	cancelPoll       context.CancelFunc
	pollDone         chan struct{}
	lastDetectedHash *string
	echoHashes       map[string]int      // ref-count per hash for multiple echo timers
	seenHashesOrder  []string            // FIFO order of seen hashes
	seenHashesSet    map[string]struct{} // lookup set for seen hashes
	isPriming        bool
}

// NormalizeForHash normalizes text for echo hash comparison by trimming,
// collapsing whitespace to a single space, and slicing up to 200 characters.
// Source: src/services/userMessageDetector.ts lines 79-81.
func NormalizeForHash(text string) string {
	trimmed := strings.TrimSpace(text)
	var b strings.Builder
	inSpace := false
	for _, r := range trimmed {
		if unicode.IsSpace(r) {
			if !inSpace {
				b.WriteRune(' ')
				inSpace = true
			}
		} else {
			b.WriteRune(r)
			inSpace = false
		}
	}
	s := b.String()
	runes := []rune(s)
	if len(runes) > NormalizeHashMaxLen {
		return string(runes[:NormalizeHashMaxLen])
	}
	return s
}

// ComputeEchoHash computes a 16-character hexadecimal SHA-256 digest of normalized text.
// Source: src/services/userMessageDetector.ts lines 86-88.
func ComputeEchoHash(text string) string {
	sum := sha256.Sum256([]byte(NormalizeForHash(text)))
	hexStr := hex.EncodeToString(sum[:])
	if len(hexStr) > EchoHashHexLen {
		return hexStr[:EchoHashHexLen]
	}
	return hexStr
}

// NewUserMessageDetector constructs a new UserMessageDetector.
func NewUserMessageDetector(options UserMessageDetectorOptions) *UserMessageDetector {
	pollInterval := options.PollInterval
	if pollInterval <= 0 {
		pollInterval = DefaultUserMessagePollInterval
	}
	echoTTL := options.EchoTTL
	if echoTTL <= 0 {
		echoTTL = DefaultEchoHashTTL
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &UserMessageDetector{
		cdpService:      options.CdpService,
		pollInterval:    pollInterval,
		onUserMessage:   options.OnUserMessage,
		echoTTL:         echoTTL,
		logger:          logger,
		echoHashes:      make(map[string]int),
		seenHashesOrder: make([]string, 0, MaxSeenHashes),
		seenHashesSet:   make(map[string]struct{}),
	}
}

// AddEchoHash registers a message as an echo sent by the bot.
// When detected in the DOM, it will be skipped. Expires automatically after echoTTL.
func (d *UserMessageDetector) AddEchoHash(text string) {
	hash := ComputeEchoHash(text)
	d.mu.Lock()
	d.echoHashes[hash]++
	d.mu.Unlock()

	time.AfterFunc(d.echoTTL, func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if count, exists := d.echoHashes[hash]; exists {
			if count <= 1 {
				delete(d.echoHashes, hash)
			} else {
				d.echoHashes[hash] = count - 1
			}
		}
	})
}

// HasEchoHash reports whether the given hash is currently recorded as an echo.
func (d *UserMessageDetector) HasEchoHash(hash string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.echoHashes[hash] > 0
}

// Start begins background monitoring. The first poll seeds current DOM state without firing callback.
func (d *UserMessageDetector) Start() {
	d.mu.Lock()
	if d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = true
	d.lastDetectedHash = nil
	d.seenHashesOrder = d.seenHashesOrder[:0]
	d.seenHashesSet = make(map[string]struct{})
	d.isPriming = true
	// echoHashes are intentionally NOT cleared -- they preserve their own 60s TTL.

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
func (d *UserMessageDetector) Stop() {
	d.mu.Lock()
	if !d.isRunning {
		d.mu.Unlock()
		return
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
}

// IsActive returns whether monitoring is currently running.
func (d *UserMessageDetector) IsActive() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isRunning
}

// addToSeenHashesLocked adds a hash to seenHashes, evicting the oldest if at capacity.
// Must be called while holding d.mu.Lock.
func (d *UserMessageDetector) addToSeenHashesLocked(hash string) {
	if _, exists := d.seenHashesSet[hash]; exists {
		return
	}
	if len(d.seenHashesOrder) >= MaxSeenHashes {
		oldest := d.seenHashesOrder[0]
		d.seenHashesOrder = d.seenHashesOrder[1:]
		delete(d.seenHashesSet, oldest)
	}
	d.seenHashesOrder = append(d.seenHashesOrder, hash)
	d.seenHashesSet[hash] = struct{}{}
}

// PollOnce executes a single polling iteration against the CDP session.
func (d *UserMessageDetector) PollOnce(ctx context.Context) {
	contextID := d.cdpService.GetPrimaryContextId()
	callParams := map[string]any{
		"expression":    DetectUserMessageScript,
		"returnByValue": true,
		"awaitPromise":  false,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	raw, err := d.cdpService.Call(ctx, "Runtime.evaluate", callParams)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "WebSocket is not connected") || strings.Contains(msg, "WebSocket disconnected") {
			return
		}
		d.logger.Error("[UserMessageDetector] Error during polling", "error", err)
		return
	}

	var resp runtimeEvaluateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return
	}

	var info *UserMessageInfo
	if len(resp.Result.Value) > 0 && string(resp.Result.Value) != "null" {
		var parsed UserMessageInfo
		if err := json.Unmarshal(resp.Result.Value, &parsed); err == nil {
			info = &parsed
		}
	}

	d.mu.Lock()
	// Clear priming flag even if DOM is empty
	if d.isPriming && (info == nil || info.Text == "") {
		d.isPriming = false
		d.logger.Debug("[UserMessageDetector] Primed with empty DOM")
		d.mu.Unlock()
		return
	}

	if info != nil && info.Text != "" {
		hash := ComputeEchoHash(info.Text)

		// First poll: seed current DOM state without firing callback
		if d.isPriming {
			d.isPriming = false
			d.lastDetectedHash = &hash
			d.addToSeenHashesLocked(hash)
			d.logger.Debug("[UserMessageDetector] Primed with existing message")
			d.mu.Unlock()
			return
		}

		// Skip if same as last detected message
		if d.lastDetectedHash != nil && *d.lastDetectedHash == hash {
			d.mu.Unlock()
			return
		}

		// Skip if already seen
		if _, seen := d.seenHashesSet[hash]; seen {
			d.lastDetectedHash = &hash
			d.logger.Debug("[UserMessageDetector] seenHash hit, skipping")
			d.mu.Unlock()
			return
		}

		// Skip if this is an echo
		if count := d.echoHashes[hash]; count > 0 {
			d.lastDetectedHash = &hash
			d.addToSeenHashesLocked(hash)
			d.logger.Debug("[UserMessageDetector] Echo hash match, skipping")
			d.mu.Unlock()
			return
		}

		d.lastDetectedHash = &hash
		d.addToSeenHashesLocked(hash)
		d.logger.Debug("[UserMessageDetector] New message detected")
		d.mu.Unlock()

		if d.onUserMessage != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						d.logger.Error("[UserMessageDetector] onUserMessage callback panicked", "panic", r)
					}
				}()
				d.onUserMessage(*info)
			}()
		}
		return
	}
	d.mu.Unlock()
}
