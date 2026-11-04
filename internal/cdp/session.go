package cdp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SessionCDPClient defines the subset of Client methods used by ChatSessionService.
type SessionCDPClient interface {
	Call(method string, params any) (json.RawMessage, error)
	GetContexts() []Context
	WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool
}

// SessionListItem mirrors SessionListItem (src/services/chatSessionService.ts:4-9).
type SessionListItem struct {
	Title    string `json:"title"`
	IsActive bool   `json:"isActive"`
}

// ChatSessionInfo mirrors ChatSessionInfo (src/services/chatSessionService.ts:12-17).
type ChatSessionInfo struct {
	Title         string `json:"title"`
	HasActiveChat bool   `json:"hasActiveChat"`
}

// SessionActionResult mirrors the action result { ok: boolean, error?: string }.
type SessionActionResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ActivateSessionOptions mirrors options in activateSessionByTitle.
type ActivateSessionOptions struct {
	MaxWait       *time.Duration
	RetryInterval *time.Duration
}

type newChatButtonState struct {
	Found   bool   `json:"found"`
	Enabled bool   `json:"enabled"`
	Cursor  string `json:"cursor,omitempty"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

type coordsState struct {
	Found bool `json:"found"`
	X     int  `json:"x"`
	Y     int  `json:"y"`
}

const (
	// ActivateSessionMaxWait is the default maximum wait time for activating a session.
	ActivateSessionMaxWait = 30000 * time.Millisecond
	// ActivateSessionRetryInterval is the default retry interval for activating a session.
	ActivateSessionRetryInterval = 800 * time.Millisecond
	// ListSessionsTarget is the maximum number of sessions to return in ListAllSessions.
	ListSessionsTarget = 20
)

const getNewChatButtonScript = `(() => {
    const btn = document.querySelector('[data-tooltip-id="new-conversation-tooltip"]');
    if (!btn) return { found: false };
    const cursor = window.getComputedStyle(btn).cursor;
    const rect = btn.getBoundingClientRect();
    return {
        found: true,
        enabled: cursor === 'pointer',
        cursor,
        x: Math.round(rect.x + rect.width / 2),
        y: Math.round(rect.y + rect.height / 2),
    };
})()`

const getChatTitleScript = `(() => {
    const panel = document.querySelector('.antigravity-agent-side-panel');
    if (!panel) return { title: '', hasActiveChat: false };
    const header = panel.querySelector('div[class*="border-b"]');
    if (!header) return { title: '', hasActiveChat: false };
    const titleEl = header.querySelector('div[class*="text-ellipsis"]');
    const title = titleEl ? (titleEl.textContent || '').trim() : '';
    // "Agent" is the default empty chat title
    const hasActiveChat = title.length > 0 && title !== 'Agent';
    return { title: title || '(Untitled)', hasActiveChat };
})()`

const findPastConversationsButtonScript = `(() => {
    const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
    const getRect = (el) => {
        const rect = el.getBoundingClientRect();
        return { found: true, x: Math.round(rect.x + rect.width / 2), y: Math.round(rect.y + rect.height / 2) };
    };

    // Strategy 1 (primary): data-past-conversations-toggle attribute
    const toggle = document.querySelector('[data-past-conversations-toggle]');
    if (toggle && isVisible(toggle)) return getRect(toggle);

    // Strategy 2: data-tooltip-id containing "history"
    const tooltipEls = Array.from(document.querySelectorAll('[data-tooltip-id]'));
    for (const el of tooltipEls) {
        if (!isVisible(el)) continue;
        const tid = (el.getAttribute('data-tooltip-id') || '').toLowerCase();
        if (tid.includes('history') || tid.includes('past-conversations')) {
            return getRect(el);
        }
    }

    // Strategy 3: SVG with lucide-history class
    const icons = Array.from(document.querySelectorAll('svg.lucide-history, svg[class*="lucide-history"]'));
    for (const icon of icons) {
        const parent = icon.closest('a, button, [role="button"], div[class*="cursor-pointer"]');
        const target = parent instanceof HTMLElement && isVisible(parent) ? parent : icon;
        if (isVisible(target)) return getRect(target);
    }

    return { found: false, x: 0, y: 0 };
})()`

const scrapePastConversationsScript = `(() => {
    const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
    const normalize = (text) => (text || '').trim();

    const items = [];
    const seen = new Set();

    // Find the scrollable conversation list container
    const containers = Array.from(document.querySelectorAll('div[class*="overflow-auto"], div[class*="overflow-y-scroll"]'));
    const container = containers.find((c) => isVisible(c) && c.querySelectorAll('div[class*="cursor-pointer"]').length > 0) || document;

    // Detect the "Other Conversations" section boundary.
    // Sessions below this header belong to other projects and must be excluded.
    let boundaryTop = Infinity;
    const headerCandidates = container.querySelectorAll('div[class*="text-xs"][class*="opacity"]');
    for (const el of headerCandidates) {
        if (!isVisible(el)) continue;
        const t = normalize(el.textContent || '');
        if (/^Other\\s+Conversations?$/i.test(t) || /^Other\\s+Conversations?$/i.test(t)) {
            boundaryTop = el.getBoundingClientRect().top;
            break;
        }
    }

    // Each session row is a div with cursor-pointer
    const rows = Array.from(container.querySelectorAll('div[class*="cursor-pointer"]'));
    for (const row of rows) {
        if (!isVisible(row)) continue;
        // Skip rows that are below the "Other Conversations" boundary
        if (row.getBoundingClientRect().top >= boundaryTop) continue;
        // Find the session title — nested span within the row
        const spans = Array.from(row.querySelectorAll('span.text-sm span, span.text-sm'));
        let title = '';
        for (const span of spans) {
            const t = normalize(span.textContent || '');
            // Skip timestamp labels like "1 hr ago", "7 mins ago"
            if (/^\\d+\\s+(min|hr|hour|day|sec|week|month|year)s?\\s+ago$/i.test(t)) continue;
            // Skip very short or action-like labels
            if (t.length < 2 || t.length > 200) continue;
            if (/^(show\\s+\\d+\\s+more|new|past|history|settings|close|menu)\\b/i.test(t)) continue;
            title = t;
            break;
        }
        if (!title || seen.has(title)) continue;
        seen.add(title);
        // Detect if this is the active/current session (has focusBackground class)
        const isActive = /focusBackground/i.test(row.className || '');
        items.push({ title, isActive });
    }
    return { sessions: items };
})()`

const findShowMoreButtonScript = `(() => {
    const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
    const els = Array.from(document.querySelectorAll('div, span'));
    for (const el of els) {
        if (!isVisible(el)) continue;
        const text = (el.textContent || '').trim();
        if (/^Show\\s+\\d+\\s+more/i.test(text)) {
            const rect = el.getBoundingClientRect();
            return { found: true, x: Math.round(rect.x + rect.width / 2), y: Math.round(rect.y + rect.height / 2) };
        }
    }
    return { found: false, x: 0, y: 0 };
})()`

// BuildActivateChatByTitleScript builds a script that activates an existing chat in the side panel by its title.
func BuildActivateChatByTitleScript(title string) string {
	safeTitle, _ := json.Marshal(title)
	return fmt.Sprintf(`(() => {
        const wantedRaw = %s;
        const wanted = (wantedRaw || '').toLowerCase().replace(/\s+/g, ' ').trim();
        if (!wanted) return { ok: false, error: 'Empty target title' };

        const panel = document.querySelector('.antigravity-agent-side-panel') || document;
        const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();
        const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
        const clickTarget = (el) => {
            const clickable = el.closest('button, [role="button"], a, li, [data-testid*="conversation"]') || el;
            if (!(clickable instanceof HTMLElement)) return false;
            clickable.click();
            return true;
        };

        const nodes = Array.from(panel.querySelectorAll('button, [role="button"], a, li, div, span'))
            .filter(isVisible);

        const exact = [];
        const includes = [];
        for (const node of nodes) {
            const text = normalize(node.textContent || '');
            if (!text) continue;
            if (text === wanted) {
                exact.push({ node, textLength: text.length });
            } else if (text.includes(wanted)) {
                includes.push({ node, textLength: text.length });
            }
        }

        const pick = (list) => {
            if (list.length === 0) return null;
            list.sort((a, b) => a.textLength - b.textLength);
            return list[0].node;
        };

        const target = pick(exact) || pick(includes);
        if (!target) return { ok: false, error: 'Chat title not found in side panel' };
        if (!clickTarget(target)) return { ok: false, error: 'Matched element is not clickable' };
        return { ok: true };
    })()`, string(safeTitle))
}

// BuildActivateViaPastConversationsScript builds a script that opens Past Conversations and selects a conversation by title.
func BuildActivateViaPastConversationsScript(title string) string {
	safeTitle, _ := json.Marshal(title)
	return fmt.Sprintf(`(() => {
        const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
        const wantedRaw = %s;
        const normalize = (text) => (text || '')
            .normalize('NFKC')
            .toLowerCase()
            .replace(/[\u2018\u2019\u201C\u201D'"`+"`"+`]/g, '')
            .replace(/\s+/g, ' ')
            .trim();
        const normalizeLoose = (text) => normalize(text).replace(/[^a-z0-9\u3040-\u30ff\u4e00-\u9faf\s]/g, '').replace(/\s+/g, ' ').trim();

        const wanted = normalize(wantedRaw || '');
        const wantedLoose = normalizeLoose(wantedRaw || '');
        if (!wanted) return { ok: false, error: 'Empty target title' };

        const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
        const asArray = (nodeList) => Array.from(nodeList || []);
        const getLabelText = (el) => {
            if (!el || !(el instanceof Element)) return '';
            const parts = [
                el.textContent || '',
                el.getAttribute('aria-label') || '',
                el.getAttribute('title') || '',
                el.getAttribute('placeholder') || '',
                el.getAttribute('data-tooltip-content') || '',
                el.getAttribute('data-testid') || '',
            ];
            return parts.filter(Boolean).join(' ');
        };
        const getClickable = (el) => {
            if (!el || !(el instanceof Element)) return null;
            const clickable = el.closest('button, [role="button"], a, li, [role="option"], [data-testid*="conversation"]');
            return clickable instanceof HTMLElement ? clickable : (el instanceof HTMLElement ? el : null);
        };
        const pickBest = (elements, patterns) => {
            const matched = [];
            for (const el of elements) {
                if (!isVisible(el)) continue;
                const text = normalize(getLabelText(el));
                const textLoose = normalizeLoose(getLabelText(el));
                if (!text) continue;
                for (const pattern of patterns) {
                    if (!pattern) continue;
                    const p = normalize(pattern);
                    const pLoose = normalizeLoose(pattern);
                    if (
                        text === p ||
                        text.includes(p) ||
                        (pLoose && (textLoose === pLoose || textLoose.includes(pLoose)))
                    ) {
                        matched.push({ el, score: Math.abs(text.length - pattern.length) });
                        break;
                    }
                }
            }
            if (matched.length === 0) return null;
            matched.sort((a, b) => a.score - b.score);
            return matched[0].el;
        };
        const clickByPatterns = (patterns, selector) => {
            const nodes = asArray(document.querySelectorAll('button, [role="button"], a, li, div, span'));
            const scopedNodes = selector ? asArray(document.querySelectorAll(selector)) : [];
            const source = scopedNodes.length > 0 ? scopedNodes : nodes;
            const target = pickBest(source, patterns);
            const clickable = getClickable(target);
            if (!clickable) return false;
            clickable.click();
            return true;
        };
        const setInputValue = (el, value) => {
            if (!el) return false;
            if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) {
                el.focus();
                el.value = value;
                el.dispatchEvent(new Event('input', { bubbles: true }));
                el.dispatchEvent(new Event('change', { bubbles: true }));
                return true;
            }
            if (el instanceof HTMLElement) {
                el.focus();
                if (el.isContentEditable) {
                    el.textContent = value;
                    el.dispatchEvent(new Event('input', { bubbles: true }));
                    return true;
                }
            }
            return false;
        };
        const clickIconHistoryButton = () => {
            const iconTargets = asArray(document.querySelectorAll('svg, i, span, div'));
            const patterns = ['history', 'clock', 'conversation', 'past'];
            for (const icon of iconTargets) {
                const descriptor = normalize([
                    icon.getAttribute?.('class') || '',
                    icon.getAttribute?.('data-testid') || '',
                    icon.getAttribute?.('data-icon') || '',
                    icon.getAttribute?.('aria-label') || '',
                    icon.getAttribute?.('title') || '',
                    icon.getAttribute?.('data-tooltip-id') || '',
                ].join(' '));
                if (!descriptor) continue;
                if (!patterns.some((p) => descriptor.includes(p))) continue;
                const clickable = getClickable(icon);
                if (clickable && isVisible(clickable)) {
                    clickable.click();
                    return true;
                }
            }
            return false;
        };
        const openMenuThenClickPast = async () => {
            const openedMenu = clickByPatterns(
                ['more', 'options', 'menu', 'actions', '...', 'ellipsis', '設定', '操作'],
                'button[aria-haspopup], [role="button"][aria-haspopup], button, [role="button"]',
            );
            if (!openedMenu) return false;
            await wait(180);
            return clickByPatterns([
                'past conversations',
                'past conversation',
                'conversation history',
                'past chats',
                '過去の会話',
                'chat history',
            ], '[role="menuitem"], [role="option"], button, [role="button"], li, div, span');
        };
        const pressEnter = (el) => {
            if (!(el instanceof HTMLElement)) return;
            el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', bubbles: true }));
            el.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', code: 'Enter', bubbles: true }));
        };
        const findSearchInput = () => {
            const inputs = asArray(document.querySelectorAll('input, textarea, [role="combobox"], [role="searchbox"], [contenteditable="true"]'));
            const strongPatterns = ['select a conversation', 'search conversation', 'search chats', 'search'];
            const placeholders = [];
            for (const el of inputs) {
                if (!isVisible(el)) continue;
                const placeholder = normalize(el.getAttribute('placeholder') || '');
                const ariaLabel = normalize(el.getAttribute('aria-label') || '');
                const text = normalize(getLabelText(el));
                const combined = [placeholder, ariaLabel, text].filter(Boolean).join(' ');
                placeholders.push({ el, combined });
            }
            for (const p of strongPatterns) {
                const found = placeholders.find((x) => x.combined.includes(p));
                if (found) return found.el;
            }
            return placeholders[0]?.el || null;
        };

        return (async () => {
            // Primary: click via data-past-conversations-toggle attribute
            let opened = false;
            const toggleBtn = document.querySelector('[data-past-conversations-toggle]');
            if (toggleBtn && isVisible(toggleBtn)) {
                const clickable = getClickable(toggleBtn);
                if (clickable) { clickable.click(); opened = true; }
            }
            if (!opened) {
                // Fallback: data-tooltip-id containing "history"
                const tooltipEls = asArray(document.querySelectorAll('[data-tooltip-id]'));
                for (const el of tooltipEls) {
                    if (!isVisible(el)) continue;
                    const tid = normalize(el.getAttribute('data-tooltip-id') || '');
                    if (tid.includes('history') || tid.includes('past-conversations')) {
                        const cl = getClickable(el);
                        if (cl) { cl.click(); opened = true; break; }
                    }
                }
            }
            if (!opened) {
                opened = clickByPatterns([
                    'past conversations',
                    'past conversation',
                    'conversation history',
                    'past chats',
                    '過去の会話',
                    'chat history',
                ]);
            }
            if (!opened) {
                opened = clickIconHistoryButton();
            }
            if (!opened) {
                opened = await openMenuThenClickPast();
            }
            if (!opened) {
                return { ok: false, error: 'Past Conversations button not found' };
            }

            await wait(320);

            // In some UI states "Select a conversation" itself is a trigger.
            clickByPatterns(['select a conversation', 'select conversation', 'conversation'], '[role="button"], button, [aria-haspopup], [data-testid*="conversation"]');
            await wait(220);

            const input = findSearchInput();
            if (input) {
                setInputValue(input, wantedRaw);
                await wait(260);
            }

            let selected = clickByPatterns([wanted, wantedLoose], '[role="option"], li, button, [data-testid*="conversation"]');
            if (!selected && input) {
                pressEnter(input);
                await wait(220);
                selected = true;
            }
            if (!selected) {
                return { ok: false, error: 'Conversation not found in Past Conversations' };
            }
            return { ok: true };
        })();
    })()`, string(safeTitle))
}

// ChatSessionService provides session management on Antigravity via CDP.
type ChatSessionService struct {
	// SleepFunc allows overriding time.Sleep for fast deterministic testing.
	SleepFunc func(d time.Duration)
}

// NewChatSessionService creates a new ChatSessionService instance.
func NewChatSessionService() *ChatSessionService {
	return &ChatSessionService{
		SleepFunc: time.Sleep,
	}
}

func (s *ChatSessionService) sleep(d time.Duration) {
	if s.SleepFunc != nil {
		s.SleepFunc(d)
	} else {
		time.Sleep(d)
	}
}

func extractEvaluateValue(raw json.RawMessage) (json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var outer struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &outer); err == nil {
		var val json.RawMessage
		if len(outer.Result.Value) > 0 {
			val = outer.Result.Value
		} else if len(outer.Value) > 0 {
			val = outer.Value
		}
		if len(val) > 0 && !bytes.Equal(val, []byte("null")) {
			return val, true
		}
	}
	return nil, false
}

func (s *ChatSessionService) evaluateOnAnyContext(cdpService SessionCDPClient, expression string, awaitPromise bool) (json.RawMessage, bool) {
	contexts := cdpService.GetContexts()
	for _, ctx := range contexts {
		raw, err := cdpService.Call("Runtime.evaluate", map[string]any{
			"expression":    expression,
			"returnByValue": true,
			"awaitPromise":  awaitPromise,
			"contextId":     ctx.ID,
		})
		if err != nil {
			continue
		}
		if val, ok := extractEvaluateValue(raw); ok {
			return val, true
		}
	}
	return nil, false
}

func (s *ChatSessionService) cdpMouseClick(cdpService SessionCDPClient, x, y int) error {
	_, err := cdpService.Call("Input.dispatchMouseEvent", map[string]any{
		"type": "mouseMoved",
		"x":    x,
		"y":    y,
	})
	if err != nil {
		return err
	}
	_, err = cdpService.Call("Input.dispatchMouseEvent", map[string]any{
		"type":       "mousePressed",
		"x":          x,
		"y":          y,
		"button":     "left",
		"clickCount": 1,
	})
	if err != nil {
		return err
	}
	_, err = cdpService.Call("Input.dispatchMouseEvent", map[string]any{
		"type":       "mouseReleased",
		"x":          x,
		"y":          y,
		"button":     "left",
		"clickCount": 1,
	})
	return err
}

func (s *ChatSessionService) getNewChatButtonState(cdpService SessionCDPClient, contexts []Context) newChatButtonState {
	for _, ctx := range contexts {
		raw, err := cdpService.Call("Runtime.evaluate", map[string]any{
			"expression":    getNewChatButtonScript,
			"returnByValue": true,
			"contextId":     ctx.ID,
		})
		if err != nil {
			continue
		}
		val, ok := extractEvaluateValue(raw)
		if !ok {
			continue
		}
		var st struct {
			Found   bool   `json:"found"`
			Enabled bool   `json:"enabled"`
			Cursor  string `json:"cursor"`
			X       int    `json:"x"`
			Y       int    `json:"y"`
		}
		if err := json.Unmarshal(val, &st); err == nil && st.Found {
			return newChatButtonState{
				Found:   true,
				Enabled: st.Enabled,
				Cursor:  st.Cursor,
				X:       st.X,
				Y:       st.Y,
			}
		}
	}
	return newChatButtonState{Found: false, Enabled: false, X: 0, Y: 0}
}

// ListAllSessions lists recent sessions by opening the Past Conversations panel.
func (s *ChatSessionService) ListAllSessions(cdpService SessionCDPClient) []SessionListItem {
	defer func() {
		_ = recover()
	}()

	// Step 1: Find Past Conversations button
	val, ok := s.evaluateOnAnyContext(cdpService, findPastConversationsButtonScript, false)
	if !ok {
		return []SessionListItem{}
	}
	var btnState coordsState
	if err := json.Unmarshal(val, &btnState); err != nil || !btnState.Found {
		return []SessionListItem{}
	}

	// Step 2: Click via CDP mouse events
	if err := s.cdpMouseClick(cdpService, btnState.X, btnState.Y); err != nil {
		return []SessionListItem{}
	}

	// Step 3: Wait for panel to render
	s.sleep(500 * time.Millisecond)

	// Step 4: Scrape sessions
	scrapeRaw, ok := s.evaluateOnAnyContext(cdpService, scrapePastConversationsScript, false)
	var sessions []SessionListItem
	if ok {
		var res struct {
			Sessions []SessionListItem `json:"sessions"`
		}
		_ = json.Unmarshal(scrapeRaw, &res)
		sessions = res.Sessions
	}
	if sessions == nil {
		sessions = []SessionListItem{}
	}

	// Step 5: If fewer than TARGET, click "Show N more..."
	if len(sessions) < ListSessionsTarget {
		showMoreRaw, found := s.evaluateOnAnyContext(cdpService, findShowMoreButtonScript, false)
		if found {
			var showMore coordsState
			if err := json.Unmarshal(showMoreRaw, &showMore); err == nil && showMore.Found {
				_ = s.cdpMouseClick(cdpService, showMore.X, showMore.Y)
				s.sleep(500 * time.Millisecond)

				// Step 6: Re-scrape
				scrapeAgainRaw, okAgain := s.evaluateOnAnyContext(cdpService, scrapePastConversationsScript, false)
				if okAgain {
					var resAgain struct {
						Sessions []SessionListItem `json:"sessions"`
					}
					_ = json.Unmarshal(scrapeAgainRaw, &resAgain)
					sessions = resAgain.Sessions
				}
			}
		}
	}

	// Step 7: Close panel with Escape
	_, _ = cdpService.Call("Input.dispatchKeyEvent", map[string]any{
		"type":                  "keyDown",
		"key":                   "Escape",
		"code":                  "Escape",
		"windowsVirtualKeyCode": 27,
		"nativeVirtualKeyCode":  27,
	})
	_, _ = cdpService.Call("Input.dispatchKeyEvent", map[string]any{
		"type":                  "keyUp",
		"key":                   "Escape",
		"code":                  "Escape",
		"windowsVirtualKeyCode": 27,
		"nativeVirtualKeyCode":  27,
	})

	if len(sessions) > ListSessionsTarget {
		return sessions[:ListSessionsTarget]
	}
	if sessions == nil {
		return []SessionListItem{}
	}
	return sessions
}

// StartNewChat starts a new chat session in the Antigravity UI.
func (s *ChatSessionService) StartNewChat(cdpService SessionCDPClient) (result SessionActionResult) {
	defer func() {
		if r := recover(); r != nil {
			result = SessionActionResult{OK: false, Error: fmt.Sprintf("%v", r)}
		}
	}()

	contexts := cdpService.GetContexts()
	if len(contexts) == 0 {
		ready := cdpService.WaitForCascadePanelReady(10000*time.Millisecond, 500*time.Millisecond)
		if !ready {
			return SessionActionResult{OK: false, Error: "No contexts available (timed out)"}
		}
		contexts = cdpService.GetContexts()
	}

	btnState := s.getNewChatButtonState(cdpService, contexts)
	if !btnState.Found {
		const maxRetries = 5
		for i := 0; i < maxRetries && !btnState.Found; i++ {
			s.sleep(1000 * time.Millisecond)
			contexts = cdpService.GetContexts()
			btnState = s.getNewChatButtonState(cdpService, contexts)
		}
	}

	if !btnState.Found {
		return SessionActionResult{OK: false, Error: "New chat button not found"}
	}

	if !btnState.Enabled {
		return SessionActionResult{OK: true}
	}

	if err := s.cdpMouseClick(cdpService, btnState.X, btnState.Y); err != nil {
		return SessionActionResult{OK: false, Error: err.Error()}
	}

	s.sleep(1500 * time.Millisecond)

	afterState := s.getNewChatButtonState(cdpService, contexts)
	if afterState.Found && !afterState.Enabled {
		return SessionActionResult{OK: true}
	}

	return SessionActionResult{OK: false, Error: "Clicked new chat button but state did not change"}
}

// GetCurrentSessionInfo gets the current chat session information from the Cascade panel header.
func (s *ChatSessionService) GetCurrentSessionInfo(cdpService SessionCDPClient) ChatSessionInfo {
	defer func() {
		_ = recover()
	}()

	contexts := cdpService.GetContexts()
	for _, ctx := range contexts {
		raw, err := cdpService.Call("Runtime.evaluate", map[string]any{
			"expression":    getChatTitleScript,
			"returnByValue": true,
			"contextId":     ctx.ID,
		})
		if err != nil {
			continue
		}
		val, ok := extractEvaluateValue(raw)
		if !ok {
			continue
		}
		var info struct {
			Title         string `json:"title"`
			HasActiveChat *bool  `json:"hasActiveChat"`
		}
		if err := json.Unmarshal(val, &info); err == nil && info.Title != "" {
			hasActive := false
			if info.HasActiveChat != nil {
				hasActive = *info.HasActiveChat
			}
			return ChatSessionInfo{
				Title:         info.Title,
				HasActiveChat: hasActive,
			}
		}
	}
	return ChatSessionInfo{
		Title:         "(Failed to retrieve)",
		HasActiveChat: false,
	}
}

// ActivateSessionByTitle activates an existing chat by title.
func (s *ChatSessionService) ActivateSessionByTitle(cdpService SessionCDPClient, title string, opts ...ActivateSessionOptions) (result SessionActionResult) {
	defer func() {
		if r := recover(); r != nil {
			result = SessionActionResult{OK: false, Error: fmt.Sprintf("%v", r)}
		}
	}()

	if strings.TrimSpace(title) == "" {
		return SessionActionResult{OK: false, Error: "Session title is empty"}
	}

	current := s.GetCurrentSessionInfo(cdpService)
	if strings.EqualFold(strings.TrimSpace(current.Title), strings.TrimSpace(title)) {
		return SessionActionResult{OK: true}
	}

	maxWait := ActivateSessionMaxWait
	retryInterval := ActivateSessionRetryInterval
	if len(opts) > 0 {
		if opts[0].MaxWait != nil {
			maxWait = *opts[0].MaxWait
		}
		if opts[0].RetryInterval != nil {
			retryInterval = *opts[0].RetryInterval
		}
	}

	usedPastConversations := false
	directResult := SessionActionResult{OK: false, Error: "not attempted"}
	var pastResult *SessionActionResult
	clicked := false
	startedAt := time.Now()
	attempts := 0

	for time.Since(startedAt) <= maxWait {
		attempts++
		directResult = s.tryActivateByDirectSidePanel(cdpService, title)
		clicked = directResult.OK

		if !clicked {
			pr := s.tryActivateByPastConversations(cdpService, title)
			pastResult = &pr
			clicked = pr.OK
			usedPastConversations = pr.OK
		}

		if clicked {
			break
		}

		if time.Since(startedAt) <= maxWait {
			s.sleep(retryInterval)
		}
	}

	if !clicked {
		directErr := directResult.Error
		if directErr == "" {
			directErr = "direct search failed"
		}
		pastErr := "past conversations search failed"
		if pastResult != nil && pastResult.Error != "" {
			pastErr = pastResult.Error
		}
		return SessionActionResult{
			OK: false,
			Error: fmt.Sprintf("Failed to activate session \"%s\" after %d attempt(s) (direct: %s; past: %s)",
				title, attempts, directErr, pastErr),
		}
	}

	// Wait briefly for DOM state transition and verify destination chat.
	s.sleep(500 * time.Millisecond)
	after := s.GetCurrentSessionInfo(cdpService)
	if strings.EqualFold(strings.TrimSpace(after.Title), strings.TrimSpace(title)) {
		return SessionActionResult{OK: true}
	}

	// If direct side-panel activation hit the wrong row, try the explicit Past Conversations flow.
	if !usedPastConversations {
		viaPast := s.tryActivateByPastConversations(cdpService, title)
		if viaPast.OK {
			s.sleep(500 * time.Millisecond)
			afterPast := s.GetCurrentSessionInfo(cdpService)
			if strings.EqualFold(strings.TrimSpace(afterPast.Title), strings.TrimSpace(title)) {
				return SessionActionResult{OK: true}
			}
			return SessionActionResult{
				OK: false,
				Error: fmt.Sprintf("Past Conversations selected a different chat (expected=\"%s\", actual=\"%s\")",
					title, afterPast.Title),
			}
		}
		pastErr := viaPast.Error
		if pastErr == "" {
			pastErr = "unknown"
		}
		return SessionActionResult{
			OK: false,
			Error: fmt.Sprintf("Activated chat did not match target title (expected=\"%s\", actual=\"%s\") and Past Conversations fallback failed (%s)",
				title, after.Title, pastErr),
		}
	}

	return SessionActionResult{
		OK: false,
		Error: fmt.Sprintf("Activated chat did not match target title (expected=\"%s\", actual=\"%s\")",
			title, after.Title),
	}
}

func (s *ChatSessionService) tryActivateByDirectSidePanel(cdpService SessionCDPClient, title string) SessionActionResult {
	return s.tryActivateWithScript(cdpService, BuildActivateChatByTitleScript(title), false)
}

func (s *ChatSessionService) tryActivateByPastConversations(cdpService SessionCDPClient, title string) SessionActionResult {
	return s.tryActivateWithScript(cdpService, BuildActivateViaPastConversationsScript(title), true)
}

func (s *ChatSessionService) tryActivateWithScript(cdpService SessionCDPClient, script string, awaitPromise bool) SessionActionResult {
	contexts := cdpService.GetContexts()
	lastError := "Activation script returned no match"
	for _, ctx := range contexts {
		raw, err := cdpService.Call("Runtime.evaluate", map[string]any{
			"expression":    script,
			"returnByValue": true,
			"awaitPromise":  awaitPromise,
			"contextId":     ctx.ID,
		})
		if err != nil {
			lastError = err.Error()
			continue
		}
		val, ok := extractEvaluateValue(raw)
		if !ok {
			continue
		}
		var res struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(val, &res); err == nil {
			if res.OK {
				return SessionActionResult{OK: true}
			}
			if res.Error != "" {
				lastError = res.Error
			}
		}
	}
	return SessionActionResult{OK: false, Error: lastError}
}
