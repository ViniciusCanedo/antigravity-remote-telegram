package monitor

// ResponseSelectorsCatalog groups all 11 JS/IIFE scripts used by Antigravity response monitoring.
type ResponseSelectorsCatalog struct {
	ResponseText       string
	StopButton         string
	PlanningActive     string
	ClickStopButton    string
	DumpAllTexts       string
	ProcessLogs        string
	CombinedPoll       string
	QuotaError         string
	ResponseStructured string
	DomDiagnostic      string
	DomObserver        string
}

// ResponseSelectors is the global singleton containing the 11 response selectors.
var ResponseSelectors = ResponseSelectorsCatalog{
	ResponseText: `(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        const selectors = [
            { sel: '.rendered-markdown', score: 10 },
            { sel: '.leading-relaxed.select-text', score: 9 },
            { sel: '.flex.flex-col.gap-y-3', score: 8 },
            { sel: '[data-message-author-role="assistant"]', score: 7 },
            { sel: '[data-message-role="assistant"]', score: 6 },
            { sel: '[class*="assistant-message"]', score: 5 },
            { sel: '[class*="message-content"]', score: 4 },
            { sel: '[class*="markdown-body"]', score: 3 },
            { sel: '.prose', score: 2 },
        ];

        const looksLikeActivityLog = (text) => {
            const normalized = (text || '').trim().toLowerCase();
            if (!normalized) return false;
            const stripped = normalized.replace(/^[^a-z]+/i, '');
            const activityPattern = /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i;
            if (activityPattern.test(stripped) && normalized.length <= 220) return true;
            if (/^initiating\s/i.test(stripped) && normalized.length <= 500) return true;
            if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
            return false;
        };

        const looksLikeFeedbackFooter = (text) => {
            const normalized = (text || '').trim().toLowerCase().replace(/\s+/g, ' ');
            if (!normalized) return false;
            return normalized === 'good bad' || normalized === 'good' || normalized === 'bad';
        };

        const isInsideExcludedContainer = (node) => {
            if (node.closest('details')) return true;
            if (node.closest('[class*="feedback"], footer')) return true;
            if (node.closest('.notify-user-container')) return true;
            if (node.closest('[role="dialog"]')) return true;
            return false;
        };

        const looksLikeToolOutput = (text) => {
            const first = (text || '').trim().split('\n')[0] || '';
            if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
            if (/^full output written to\b/i.test(first)) return true;
            if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
            var lower = (text || '').trim().toLowerCase();
            if (/^title:\s/.test(lower) && /\surl:\s/.test(lower) && /\ssnippet:\s/.test(lower)) return true;
            if (/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log|ruby|go|rust|java|c|cpp|csharp|php|swift|kotlin)$/i.test(first)) return true;
            return false;
        };

        const looksLikeQuotaPopup = (text) => {
            var lower = (text || '').trim().toLowerCase();
            // Inline error: "Error You have exhausted your quota on this model."
            if (lower.includes('exhausted your quota') || lower.includes('exhausted quota')) return true;
            // Popup: quota keyword + dismiss/upgrade button text
            if (!lower.includes('model quota reached') && !lower.includes('quota exceeded') && !lower.includes('rate limit')) return false;
            return lower.includes('dismiss') || lower.includes('upgrade');
        };

        const combinedSelector = selectors.map((s) => s.sel).join(', ');
        const seen = new Set();

        for (const scope of scopes) {
            const nodes = scope.querySelectorAll(combinedSelector);
            for (let i = nodes.length - 1; i >= 0; i--) {
                const node = nodes[i];
                if (!node || seen.has(node)) continue;
                seen.add(node);
                if (isInsideExcludedContainer(node)) continue;
                const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                if (!text || text.length < 2) continue;
                if (looksLikeActivityLog(text)) continue;
                if (looksLikeFeedbackFooter(text)) continue;
                if (looksLikeToolOutput(text)) continue;
                if (looksLikeQuotaPopup(text)) continue;
                // Prefer recency first: return the newest acceptable node.
                return text;
            }
        }

        return null;
    })()`,

	StopButton: `(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        for (const scope of scopes) {
            const el = scope.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]');
            if (el) return { isGenerating: true };
        }

        const normalize = (value) => (value || '').toLowerCase().replace(/\s+/g, ' ').trim();
        const STOP_PATTERNS = [
            /^stop$/,
            /^stop generating$/,
            /^stop response$/,
            /^停止$/,
            /^生成を停止$/,
            /^応答を停止$/,
        ];
        const isStopLabel = (value) => {
            const normalized = normalize(value);
            if (!normalized) return false;
            return STOP_PATTERNS.some((re) => re.test(normalized));
        };
        for (const scope of scopes) {
            const buttons = scope.querySelectorAll('button, [role="button"]');
            for (let i = 0; i < buttons.length; i++) {
                const btn = buttons[i];
                const labels = [
                    btn.textContent || '',
                    btn.getAttribute('aria-label') || '',
                    btn.getAttribute('title') || '',
                ];
                if (labels.some(isStopLabel)) {
                    return { isGenerating: true };
                }
            }
        }

        return { isGenerating: false };
    })()`,

	PlanningActive: `(() => {
        var container = document.querySelector('.notify-user-container');
        if (!container) return false;
        var buttons = Array.from(container.querySelectorAll('button')).filter(function(btn) { return btn.offsetParent !== null; });
        var hasOpen = buttons.some(function(btn) { return (btn.textContent || '').toLowerCase().trim() === 'open'; });
        var hasProceed = buttons.some(function(btn) { return (btn.textContent || '').toLowerCase().trim() === 'proceed'; });
        return hasOpen && hasProceed;
    })()`,

	ClickStopButton: `(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        for (const scope of scopes) {
            const el = scope.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]');
            if (el && typeof el.click === 'function') {
                el.click();
                return { ok: true, method: 'tooltip-id' };
            }
        }

        const normalize = (value) => (value || '').toLowerCase().replace(/\s+/g, ' ').trim();
        const STOP_PATTERNS = [
            /^stop$/,
            /^stop generating$/,
            /^stop response$/,
            /^停止$/,
            /^生成を停止$/,
            /^応答を停止$/,
        ];
        const isStopLabel = (value) => {
            const normalized = normalize(value);
            if (!normalized) return false;
            return STOP_PATTERNS.some((re) => re.test(normalized));
        };
        for (const scope of scopes) {
            const buttons = scope.querySelectorAll('button, [role="button"]');
            for (let i = 0; i < buttons.length; i++) {
                const btn = buttons[i];
                const labels = [
                    btn.textContent || '',
                    btn.getAttribute('aria-label') || '',
                    btn.getAttribute('title') || '',
                ];
                if (labels.some(isStopLabel) && typeof btn.click === 'function') {
                    btn.click();
                    return { ok: true, method: 'text-fallback' };
                }
            }
        }

        return { ok: false, error: 'Stop button not found' };
    })()`,

	DumpAllTexts: `(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        const selectors = [
            { sel: '.rendered-markdown', score: 10 },
            { sel: '.leading-relaxed.select-text', score: 9 },
            { sel: '.flex.flex-col.gap-y-3', score: 8 },
            { sel: '[data-message-author-role="assistant"]', score: 7 },
            { sel: '[data-message-role="assistant"]', score: 6 },
            { sel: '[class*="assistant-message"]', score: 5 },
            { sel: '[class*="message-content"]', score: 4 },
            { sel: '[class*="markdown-body"]', score: 3 },
            { sel: '.prose', score: 2 },
        ];

        const looksLikeActivityLog = (text) => {
            const normalized = (text || '').trim().toLowerCase();
            if (!normalized) return false;
            const stripped = normalized.replace(/^[^a-z]+/i, '');
            const activityPattern = /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i;
            if (activityPattern.test(stripped) && normalized.length <= 220) return true;
            if (/^initiating\s/i.test(stripped) && normalized.length <= 500) return true;
            if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
            return false;
        };
        const looksLikeFeedbackFooter = (text) => {
            const normalized = (text || '').trim().toLowerCase().replace(/\s+/g, ' ');
            if (!normalized) return false;
            return normalized === 'good bad' || normalized === 'good' || normalized === 'bad';
        };
        const isInsideExcludedContainer = (node) => {
            if (node.closest('details')) return true;
            if (node.closest('[class*="feedback"], footer')) return true;
            if (node.closest('.notify-user-container')) return true;
            if (node.closest('[role="dialog"]')) return true;
            return false;
        };
        const looksLikeToolOutput = (text) => {
            const first = (text || '').trim().split('\n')[0] || '';
            if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
            if (/^full output written to\b/i.test(first)) return true;
            if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
            var lower = (text || '').trim().toLowerCase();
            if (/^title:\s/.test(lower) && /\surl:\s/.test(lower) && /\ssnippet:\s/.test(lower)) return true;
            if (/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log|ruby|go|rust|java|c|cpp|csharp|php|swift|kotlin)$/i.test(first)) return true;
            return false;
        };

        const results = [];
        const seen = new Set();

        for (const scope of scopes) {
            for (const { sel, score } of selectors) {
                const nodes = scope.querySelectorAll(sel);
                for (let i = nodes.length - 1; i >= 0; i--) {
                    const node = nodes[i];
                    if (!node || seen.has(node)) continue;
                    seen.add(node);
                    const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                    let skip = null;
                    if (!text || text.length < 2) skip = 'too-short';
                    else if (isInsideExcludedContainer(node)) skip = 'excluded-container';
                    else if (looksLikeActivityLog(text)) skip = 'activity-log';
                    else if (looksLikeFeedbackFooter(text)) skip = 'feedback-footer';
                    else if (looksLikeToolOutput(text)) skip = 'tool-output';
                    else {
                        var qlower = (text || '').trim().toLowerCase();
                        if (qlower.includes('exhausted your quota') || qlower.includes('exhausted quota')) skip = 'quota-popup';
                        else if ((qlower.includes('model quota reached') || qlower.includes('quota exceeded') || qlower.includes('rate limit'))
                            && (qlower.includes('dismiss') || qlower.includes('upgrade'))) skip = 'quota-popup';
                    }
                    const classes = (node.className || '').toString().slice(0, 80);
                    results.push({
                        sel,
                        score,
                        skip,
                        len: text.length,
                        classes,
                        preview: text.slice(0, 120),
                    });
                }
            }
        }
        return results;
    })()`,

	ProcessLogs: `(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        const selectors = [
            { sel: '.rendered-markdown', score: 10 },
            { sel: '.leading-relaxed.select-text', score: 9 },
            { sel: '.flex.flex-col.gap-y-3', score: 8 },
            { sel: '[data-message-author-role="assistant"]', score: 7 },
            { sel: '[data-message-role="assistant"]', score: 6 },
            { sel: '[class*="assistant-message"]', score: 5 },
            { sel: '[class*="message-content"]', score: 4 },
            { sel: '[class*="markdown-body"]', score: 3 },
            { sel: '.prose', score: 2 },
        ];

        const looksLikeActivityLog = (text) => {
            const normalized = (text || '').trim().toLowerCase();
            if (!normalized) return false;
            const stripped = normalized.replace(/^[^a-z]+/i, '');
            const activityPattern = /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i;
            if (activityPattern.test(stripped) && normalized.length <= 220) return true;
            if (/^initiating\s/i.test(stripped) && normalized.length <= 500) return true;
            if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
            return false;
        };

        const looksLikeToolOutput = (text) => {
            const first = (text || '').trim().split('\n')[0] || '';
            if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
            if (/^full output written to\b/i.test(first)) return true;
            if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
            var lower = (text || '').trim().toLowerCase();
            if (/^title:\s/.test(lower) && /\surl:\s/.test(lower) && /\ssnippet:\s/.test(lower)) return true;
            if (/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log|ruby|go|rust|java|c|cpp|csharp|php|swift|kotlin)$/i.test(first)) return true;
            return false;
        };

        const isInsideExcludedContainer = (node) => {
            if (node.closest('details')) return true;
            if (node.closest('[class*="feedback"], footer')) return true;
            if (node.closest('.notify-user-container')) return true;
            if (node.closest('[role="dialog"]')) return true;
            return false;
        };

        const results = [];
        const seen = new Set();

        for (const scope of scopes) {
            for (const { sel } of selectors) {
                const nodes = scope.querySelectorAll(sel);
                for (let i = 0; i < nodes.length; i++) {
                    const node = nodes[i];
                    if (!node || seen.has(node)) continue;
                    seen.add(node);
                    if (isInsideExcludedContainer(node)) continue;
                    const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                    if (!text || text.length < 4) continue;
                    if (looksLikeActivityLog(text) || looksLikeToolOutput(text)) {
                        results.push(text.slice(0, 300));
                    }
                }
            }
        }

        return results;
    })()`,

	CombinedPoll: `(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        // --- Stop button ---
        let isGenerating = false;
        for (const scope of scopes) {
            const el = scope.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]');
            if (el) { isGenerating = true; break; }
        }
        if (!isGenerating) {
            const normalize = (value) => (value || '').toLowerCase().replace(/\s+/g, ' ').trim();
            const STOP_PATTERNS = [/^stop$/, /^stop generating$/, /^stop response$/, /^停止$/, /^生成を停止$/, /^応答を停止$/];
            const isStopLabel = (value) => { const n = normalize(value); return n ? STOP_PATTERNS.some((re) => re.test(n)) : false; };
            outer: for (const scope of scopes) {
                const buttons = scope.querySelectorAll('button, [role="button"]');
                for (let i = 0; i < buttons.length; i++) {
                    const btn = buttons[i];
                    if ([btn.textContent || '', btn.getAttribute('aria-label') || '', btn.getAttribute('title') || ''].some(isStopLabel)) {
                        isGenerating = true; break outer;
                    }
                }
            }
        }

        // --- Quota error ---
        let quotaError = false;
        const scope = panel || document;
        const QUOTA_KEYWORDS = ['model quota reached', 'rate limit', 'quota exceeded', 'exhausted your quota', 'exhausted quota'];
        const isInsideResponse = (node) =>
            node.closest('.rendered-markdown, .prose, pre, code, [data-message-author-role="assistant"], [data-message-role="assistant"], [class*="message-content"]');
        const headings = scope.querySelectorAll('h3 span, h3');
        for (const el of headings) {
            if (isInsideResponse(el)) continue;
            const text = (el.textContent || '').trim().toLowerCase();
            if (QUOTA_KEYWORDS.some(kw => text.includes(kw))) { quotaError = true; break; }
        }
        if (!quotaError) {
            const inlineSpans = scope.querySelectorAll('span');
            for (const el of inlineSpans) {
                if (isInsideResponse(el)) continue;
                const text = (el.textContent || '').trim().toLowerCase();
                if (text.includes('exhausted your quota') || text.includes('exhausted quota')) { quotaError = true; break; }
            }
        }
        if (!quotaError) {
            const errorSelectors = ['[role="alert"]','[class*="error"]','[class*="warning"]','[class*="toast"]','[class*="banner"]','[class*="notification"]','[class*="alert"]','[class*="quota"]','[class*="rate-limit"]'];
            const errorElements = scope.querySelectorAll(errorSelectors.join(', '));
            for (const el of errorElements) {
                if (isInsideResponse(el)) continue;
                const text = (el.textContent || '').trim().toLowerCase();
                if (QUOTA_KEYWORDS.some(kw => text.includes(kw))) { quotaError = true; break; }
            }
        }

        // --- Planning active ---
        let planningActive = false;
        const container = document.querySelector('.notify-user-container');
        if (container) {
            const buttons = Array.from(container.querySelectorAll('button')).filter(function(btn) { return btn.offsetParent !== null; });
            const hasOpen = buttons.some(function(btn) { return (btn.textContent || '').toLowerCase().trim() === 'open'; });
            const hasProceed = buttons.some(function(btn) { return (btn.textContent || '').toLowerCase().trim() === 'proceed'; });
            planningActive = hasOpen && hasProceed;
        }

        // --- Legacy text extraction ---
        const selectors = [
            { sel: '.rendered-markdown', score: 10 },
            { sel: '.leading-relaxed.select-text', score: 9 },
            { sel: '.flex.flex-col.gap-y-3', score: 8 },
            { sel: '[data-message-author-role="assistant"]', score: 7 },
            { sel: '[data-message-role="assistant"]', score: 6 },
            { sel: '[class*="assistant-message"]', score: 5 },
            { sel: '[class*="message-content"]', score: 4 },
            { sel: '[class*="markdown-body"]', score: 3 },
            { sel: '.prose', score: 2 },
        ];
        const looksLikeActivityLog = (text) => {
            const normalized = (text || '').trim().toLowerCase();
            if (!normalized) return false;
            const stripped = normalized.replace(/^[^a-z]+/i, '');
            const activityPattern = /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i;
            if (activityPattern.test(stripped) && normalized.length <= 220) return true;
            if (/^initiating\s/i.test(stripped) && normalized.length <= 500) return true;
            if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
            return false;
        };
        const looksLikeFeedbackFooter = (text) => {
            const normalized = (text || '').trim().toLowerCase().replace(/\s+/g, ' ');
            if (!normalized) return false;
            return normalized === 'good bad' || normalized === 'good' || normalized === 'bad';
        };
        const isInsideExcludedContainer = (node) => {
            if (node.closest('details')) return true;
            if (node.closest('[class*="feedback"], footer')) return true;
            if (node.closest('.notify-user-container')) return true;
            if (node.closest('[role="dialog"]')) return true;
            return false;
        };
        const looksLikeToolOutput = (text) => {
            const first = (text || '').trim().split('\n')[0] || '';
            if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
            if (/^full output written to\b/i.test(first)) return true;
            if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
            var lower = (text || '').trim().toLowerCase();
            if (/^title:\s/.test(lower) && /\surl:\s/.test(lower) && /\ssnippet:\s/.test(lower)) return true;
            if (/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log|ruby|go|rust|java|c|cpp|csharp|php|swift|kotlin)$/i.test(first)) return true;
            return false;
        };
        const looksLikeQuotaPopup = (text) => {
            var lower = (text || '').trim().toLowerCase();
            if (lower.includes('exhausted your quota') || lower.includes('exhausted quota')) return true;
            if (!lower.includes('model quota reached') && !lower.includes('quota exceeded') && !lower.includes('rate limit')) return false;
            return lower.includes('dismiss') || lower.includes('upgrade');
        };
        const combinedSelector = selectors.map((s) => s.sel).join(', ');
        const seen = new Set();
        let responseText = null;
        for (const s of scopes) {
            const nodes = s.querySelectorAll(combinedSelector);
            for (let i = nodes.length - 1; i >= 0; i--) {
                const node = nodes[i];
                if (!node || seen.has(node)) continue;
                seen.add(node);
                if (isInsideExcludedContainer(node)) continue;
                const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                if (!text || text.length < 2) continue;
                if (looksLikeActivityLog(text)) continue;
                if (looksLikeFeedbackFooter(text)) continue;
                if (looksLikeToolOutput(text)) continue;
                if (looksLikeQuotaPopup(text)) continue;
                responseText = text;
                break;
            }
            if (responseText !== null) break;
        }

        // --- Process logs ---
        const logSeen = new Set();
        const processLogs = [];
        for (const s of scopes) {
            for (const { sel } of selectors) {
                const nodes = s.querySelectorAll(sel);
                for (let i = 0; i < nodes.length; i++) {
                    const node = nodes[i];
                    if (!node || logSeen.has(node)) continue;
                    logSeen.add(node);
                    if (isInsideExcludedContainer(node)) continue;
                    const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                    if (!text || text.length < 4) continue;
                    if (looksLikeActivityLog(text) || looksLikeToolOutput(text)) {
                        processLogs.push(text.slice(0, 300));
                    }
                }
            }
        }

        return { isGenerating, quotaError, planningActive, responseText, processLogs };
    })()`,

	QuotaError: `(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scope = panel || document;
        const QUOTA_KEYWORDS = ['model quota reached', 'rate limit', 'quota exceeded', 'exhausted your quota', 'exhausted quota'];
        const isInsideResponse = (node) =>
            node.closest('.rendered-markdown, .prose, pre, code, [data-message-author-role="assistant"], [data-message-role="assistant"], [class*="message-content"]');

        // Primary: text-based detection via h3 span (Tailwind-only popup)
        const headings = scope.querySelectorAll('h3 span, h3');
        for (const el of headings) {
            if (isInsideResponse(el)) continue;
            const text = (el.textContent || '').trim().toLowerCase();
            if (QUOTA_KEYWORDS.some(kw => text.includes(kw))) return true;
        }

        // Inline error: "Error You have exhausted your quota on this model."
        // Appears in process log area as a span inside flex containers
        const inlineSpans = scope.querySelectorAll('span');
        for (const el of inlineSpans) {
            if (isInsideResponse(el)) continue;
            const text = (el.textContent || '').trim().toLowerCase();
            if (text.includes('exhausted your quota') || text.includes('exhausted quota')) return true;
        }

        // Fallback: semantic class-based detection
        const errorSelectors = [
            '[role="alert"]',
            '[class*="error"]',
            '[class*="warning"]',
            '[class*="toast"]',
            '[class*="banner"]',
            '[class*="notification"]',
            '[class*="alert"]',
            '[class*="quota"]',
            '[class*="rate-limit"]',
        ];
        const errorElements = scope.querySelectorAll(errorSelectors.join(', '));
        for (const el of errorElements) {
            if (isInsideResponse(el)) continue;
            const text = (el.textContent || '').trim().toLowerCase();
            if (QUOTA_KEYWORDS.some(kw => text.includes(kw))) return true;
        }
        return false;
    })()`,

	ResponseStructured: ExtractAssistantSegmentsPayloadScript(),

	DomDiagnostic: `(() => {
        var panel = document.querySelector('.antigravity-agent-side-panel');
        var scope = panel || document;
        var diag = { detailsCount: 0, detailsDump: [], activityNodes: [], allTextNodes: [] };

        // 1. Dump all <details> elements
        var details = scope.querySelectorAll('details');
        diag.detailsCount = details.length;
        for (var i = 0; i < Math.min(details.length, 5); i++) {
            diag.detailsDump.push({
                outerHTML: details[i].outerHTML.slice(0, 500),
                summaryText: (details[i].querySelector('summary') || {}).textContent || '(no summary)',
                childCount: details[i].children.length
            });
        }

        // 2. Find all text nodes that look like activity
        var selectors = '.rendered-markdown, .leading-relaxed.select-text, .flex.flex-col.gap-y-3, [data-message-author-role="assistant"], [data-message-role="assistant"], [class*="assistant-message"], [class*="message-content"], [class*="markdown-body"], .prose';
        var nodes = scope.querySelectorAll(selectors);
        for (var j = 0; j < nodes.length; j++) {
            var text = (nodes[j].innerText || nodes[j].textContent || '').trim();
            if (!text || text.length < 2) continue;
            diag.allTextNodes.push({
                tag: nodes[j].tagName,
                className: (nodes[j].className || '').toString().slice(0, 100),
                text: text.slice(0, 200),
                insideDetails: !!nodes[j].closest('details'),
                length: text.length
            });
        }

        // 3. Broader scan: any element with activity-like text
        var allEls = scope.querySelectorAll('*');
        for (var k = 0; k < allEls.length; k++) {
            var el = allEls[k];
            if (el.children.length > 2) continue; // only leaf-ish nodes
            var t = (el.textContent || '').trim();
            if (!t || t.length < 5 || t.length > 300) continue;
            var lower = t.toLowerCase();
            if (/^(?:analy[sz]|read|writ|run|search|think|process|execut|debug|test)/i.test(lower) || /\//.test(t)) {
                diag.activityNodes.push({
                    tag: el.tagName,
                    className: (el.className || '').toString().slice(0, 100),
                    text: t.slice(0, 200),
                    parentTag: el.parentElement ? el.parentElement.tagName : null,
                    parentClass: el.parentElement ? (el.parentElement.className || '').toString().slice(0, 100) : null,
                    insideDetails: !!el.closest('details')
                });
            }
        }
        return diag;
    })()`,

	DomObserver: `(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel') || document.body;
        // Clean up previous observer if exists
        if (window._antigravity-telegram-remoteObserver) window._antigravity-telegram-remoteObserver.disconnect();
        
        let debounceTimer;
        const notify = () => {
            if (typeof window.onantigravity-telegram-remoteDOMChange === 'function') {
                window.onantigravity-telegram-remoteDOMChange('changed');
            }
        };

        window._antigravity-telegram-remoteObserver = new MutationObserver((mutations) => {
            clearTimeout(debounceTimer);
            // Throttle to 100ms to avoid flooding CDP during rapid streaming
            debounceTimer = setTimeout(notify, 100);
        });

        window._antigravity-telegram-remoteObserver.observe(panel, {
            childList: true,
            subtree: true,
            characterData: true
        });
        return { ok: true, observing: panel.className || 'body' };
    })()`,
}
