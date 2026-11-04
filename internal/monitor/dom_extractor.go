package monitor

import (
	"encoding/json"
	"strings"

	"antigravity-telegram-remote/internal/formatter"
)

// AssistantSegmentKind enumerates the 6 segment types extracted from the DOM.
type AssistantSegmentKind string

const (
	SegmentKindAssistantBody   AssistantSegmentKind = "assistant-body"
	SegmentKindThinking        AssistantSegmentKind = "thinking"
	SegmentKindThinkingContent AssistantSegmentKind = "thinking-content"
	SegmentKindToolCall        AssistantSegmentKind = "tool-call"
	SegmentKindToolResult      AssistantSegmentKind = "tool-result"
	SegmentKindFeedback        AssistantSegmentKind = "feedback"
)

// AssistantDomSegment represents an individual segment extracted from an assistant turn in the DOM.
type AssistantDomSegment struct {
	Kind         AssistantSegmentKind `json:"kind"`
	Text         string               `json:"text"`
	Role         string               `json:"role"` // always "assistant"
	MessageIndex int                  `json:"messageIndex"`
	DomPath      string               `json:"domPath"`
}

// AssistantDomSegmentPayload represents the payload returned by the DOM extraction IIFE via CDP.
type AssistantDomSegmentPayload struct {
	Source      string                `json:"source"` // must be "dom-structured"
	ExtractedAt int64                 `json:"extractedAt"`
	Segments    []AssistantDomSegment `json:"segments"`
}

// ClassifyDiagnostics contains diagnostic metadata about segment classification.
type ClassifyDiagnostics struct {
	Source         string         `json:"source"` // "dom-structured" | "legacy-fallback"
	SegmentCounts  map[string]int `json:"segmentCounts"`
	FallbackReason string         `json:"fallbackReason,omitempty"`
}

// ClassifyResult represents the final output of ClassifyAssistantSegments.
type ClassifyResult struct {
	FinalOutputText string              `json:"finalOutputText"`
	ActivityLines   []string            `json:"activityLines"`
	ThinkingLines   []string            `json:"thinkingLines"`
	Feedback        []string            `json:"feedback"`
	Diagnostics     ClassifyDiagnostics `json:"diagnostics"`
}

// ExtractAssistantSegmentsPayloadScript returns the literal JavaScript IIFE string evaluated in
// the browser context via CDP Runtime.evaluate.
func ExtractAssistantSegmentsPayloadScript() string {
	return `(() => {
    var panel = document.querySelector('.antigravity-agent-side-panel');
    var scope = panel || document;

    // Same selectors as RESPONSE_TEXT — ordered by specificity
    var selectors = [
        '.rendered-markdown',
        '.leading-relaxed.select-text',
        '.flex.flex-col.gap-y-3',
        '[data-message-author-role="assistant"]',
        '[data-message-role="assistant"]',
        '[class*="assistant-message"]',
        '[class*="message-content"]',
        '[class*="markdown-body"]',
        '.prose',
    ];

    var looksLikeActivityLog = function(text) {
        var normalized = (text || '').trim().toLowerCase();
        if (!normalized) return false;
        // Strip leading emoji/non-letter chars for matching
        var stripped = normalized.replace(/^[^a-z]+/i, '');
        if (/^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i.test(stripped) && normalized.length <= 220) return true;
        // "Initiating..." removed — too broad; catches response body text like
        // "Initiating Task Execution I'm now ready to..." causing false activity entries.
        if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
        return false;
    };

    var looksLikeFeedbackFooter = function(text) {
        var normalized = (text || '').trim().toLowerCase().replace(/\s+/g, ' ');
        return normalized === 'good bad' || normalized === 'good' || normalized === 'bad';
    };

    var looksLikeToolOutput = function(text) {
        var first = (text || '').trim().split('\n')[0] || '';
        if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
        if (/^full output written to\b/i.test(first)) return true;
        if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
        return false;
    };

    var looksLikeThinking = function(text) {
        var stripped = (text || '').trim().replace(/^[^a-zA-Z]+/, '').toLowerCase();
        if (!stripped) return false;
        return /^(?:thought for|thinking)\b/i.test(stripped);
    };

    var isInsideExcludedContainer = function(node) {
        if (node.closest('details')) return true;
        if (node.closest('[class*="feedback"], footer')) return true;
        if (node.closest('.notify-user-container')) return true;
        if (node.closest('[role="dialog"]')) return true;
        return false;
    };

    var segments = [];
    var seen = new Set();
    var bodyFound = false;

    // Pass 1: Find assistant body — last non-excluded content node (recency first)
    var combinedSelector = selectors.join(', ');
    var nodes = scope.querySelectorAll(combinedSelector);

    for (var i = nodes.length - 1; i >= 0; i--) {
        var node = nodes[i];
        if (!node || seen.has(node)) continue;
        seen.add(node);
        if (isInsideExcludedContainer(node)) continue;

        var text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
        if (!text || text.length < 2) continue;
        if (looksLikeActivityLog(text)) continue;
        if (looksLikeFeedbackFooter(text)) continue;
        if (looksLikeToolOutput(text)) continue;

        // This is the assistant body — normalize code blocks then extract innerHTML
        // AG wraps code in <pre><div class="..."> with a header div for language label,
        // embedded <style> tags, and code-line divs — NOT standard <pre><code>.
        // We normalize to <pre><code> for htmlToTelegramHtml.
        var clone = node.cloneNode(true);
        var pres = clone.querySelectorAll('pre');
        for (var pi = 0; pi < pres.length; pi++) {
            var pre = pres[pi];
            // Extract language from header div (font-sans text-sm class)
            var langDiv = pre.querySelector('.font-sans.text-sm, [class*="text-sm"][class*="opacity"]');
            var lang = langDiv ? (langDiv.textContent || '').trim() : '';

            // Remove <style> tags before extracting text (AG injects code-block CSS)
            var styles = pre.querySelectorAll('style');
            for (var si = 0; si < styles.length; si++) {
                styles[si].parentNode.removeChild(styles[si]);
            }

            // Remove the header bar (language label + copy button)
            var headerBar = pre.querySelector('[class*="rounded-t"][class*="border-b"]');
            if (headerBar) headerBar.parentNode.removeChild(headerBar);

            // Extract code text: prefer code-line elements for reliable newlines
            var codeLines = pre.querySelectorAll('.code-line, [class*="code-line"]');
            var codeText;
            if (codeLines.length > 0) {
                var lineTexts = [];
                for (var cli = 0; cli < codeLines.length; cli++) {
                    lineTexts.push(codeLines[cli].textContent || '');
                }
                codeText = lineTexts.join('\n');
            } else {
                // Fallback: use innerText of cleaned pre
                codeText = (pre.innerText || '').trim();
                if (lang && codeText.startsWith(lang)) {
                    codeText = codeText.slice(lang.length).trim();
                }
            }
            // Remove trailing "Copy" or clipboard button text
            codeText = codeText.replace(/\nCopy$/i, '').replace(/\ncopy code$/i, '').trim();
            // Replace with simple <pre><code>
            var newPre = document.createElement('pre');
            var newCode = document.createElement('code');
            if (lang) newCode.setAttribute('class', 'language-' + lang);
            newCode.textContent = codeText;
            newPre.appendChild(newCode);
            pre.parentNode.replaceChild(newPre, pre);
        }
        // Also remove any top-level <style> tags from the clone
        var topStyles = clone.querySelectorAll('style');
        for (var tsi = 0; tsi < topStyles.length; tsi++) {
            topStyles[tsi].parentNode.removeChild(topStyles[tsi]);
        }
        var bodyHtml = clone.innerHTML;
        if (bodyHtml && bodyHtml.trim()) {
            segments.push({
                kind: 'assistant-body',
                text: bodyHtml,
                role: 'assistant',
                messageIndex: 0,
                domPath: 'multi-selector'
            });
            bodyFound = true;
            break; // Only take the last (most recent) output node
        }
    }

    // Pass 2: Extract thinking + tool segments from <details>
    var details = scope.querySelectorAll('details');
    for (var di = 0; di < details.length; di++) {
        var detail = details[di];
        var summary = detail.querySelector('summary');
        var isThinkingBlock = false;
        if (summary) {
            var summaryText = (summary.textContent || '').trim();
            // Strip leading emoji/non-letter chars for matching
            var summaryStripped = summaryText.replace(/^[^a-zA-Z]+/, '');
            isThinkingBlock = /^(?:thought for|thinking)\b/i.test(summaryStripped);
            if (summaryText) {
                segments.push({
                    kind: isThinkingBlock ? 'thinking' : 'tool-call',
                    text: summaryText,
                    role: 'assistant',
                    messageIndex: 0,
                    domPath: 'details:nth(' + di + ') summary'
                });
            }
        }
        // Force-open collapsed details to access hidden content
        var wasOpen = detail.open;
        if (!wasOpen) {
            detail.open = true;
        }
        // Extract child content inside <details>
        var children = detail.children;
        for (var ci = 0; ci < children.length; ci++) {
            var child = children[ci];
            if (child.tagName === 'SUMMARY' || child.tagName === 'STYLE') continue;
            var childText = (child.innerText || child.textContent || '').trim();
            if (!childText || childText.length < 5) continue;
            // Skip junk: pure numbers, single words under 8 chars
            if (/^\d+$/.test(childText)) continue;
            if (isThinkingBlock) {
                // Thinking body content — preserve more text for display
                segments.push({
                    kind: 'thinking-content',
                    text: childText.slice(0, 2000),
                    role: 'assistant',
                    messageIndex: 0,
                    domPath: 'details:nth(' + di + ') child:nth(' + ci + ')'
                });
            } else {
                var childKind = looksLikeToolOutput(childText) ? 'tool-result' : 'tool-call';
                segments.push({
                    kind: childKind,
                    text: childText.slice(0, 300),
                    role: 'assistant',
                    messageIndex: 0,
                    domPath: 'details:nth(' + di + ') child:nth(' + ci + ')'
                });
            }
        }
        // Restore original collapsed state
        if (!wasOpen) {
            detail.open = false;
        }
    }

    // Pass 2.5: Broad activity scan - leaf-ish elements with activity-like text
    // Uses querySelectorAll('*') to find activity nodes that don't match content
    // selectors (e.g. "Analyzed package.json#L1-75" inside <div class="flex flex-row">).
    // Excludes nodes inside response body containers to avoid false positives.
    // Uses ancestor dedup to prevent capturing both parent and child activity nodes.
    var actSeen = new Set();
    var allEls = scope.querySelectorAll('*');
    for (var ai = 0; ai < allEls.length; ai++) {
        var el = allEls[ai];
        if (el.children.length > 3) continue;       // leaf-ish only
        if (seen.has(el)) continue;                  // already captured in Pass 1/2
        if (isInsideExcludedContainer(el)) continue;
        // Skip nodes inside response body containers (prevents capturing inline words)
        if (el.closest('.leading-relaxed, .rendered-markdown, .prose, .animate-markdown, [data-message-role], [data-message-author-role]')) continue;
        var aText = (el.innerText || el.textContent || '').replace(/\r/g, '').trim();
        if (!aText || aText.length < 5 || aText.length > 300) continue;
        // Skip junk: pure numbers, single short words
        if (/^\d+$/.test(aText)) continue;
        // Skip model selector / dropdown UI text (contains multiple model names)
        var aLower = aText.toLowerCase();
        if ((aLower.includes('gemini') || aLower.includes('claude') || aLower.includes('gpt')) && aLower.includes('send')) continue;
        var isThinking = looksLikeThinking(aText);
        if (isThinking || looksLikeActivityLog(aText) || looksLikeToolOutput(aText)) {
            // When we find a short activity verb (e.g. "Analyzed" alone), the file
            // reference may be in a sibling element.  Walk up to the nearest ancestor
            // (max 3 levels) whose innerText contains the verb AND more context
            // (e.g. "Analyzed package.json#L1-75").
            var bestText = aText;
            if (!isThinking && looksLikeActivityLog(aText) && aText.length < 60) {
                var ancestor = el.parentElement;
                for (var up = 0; up < 3 && ancestor && ancestor !== scope; up++) {
                    // Don't cross into excluded containers
                    if (ancestor.closest && ancestor.closest('.leading-relaxed, .rendered-markdown, .prose, .animate-markdown, [data-message-role], [data-message-author-role]')) break;
                    var ancestorText = (ancestor.innerText || ancestor.textContent || '').replace(/\r/g, '').trim();
                    // Accept if it still looks like activity, is longer, and not too long
                    if (ancestorText && ancestorText.length > bestText.length && ancestorText.length <= 300 && looksLikeActivityLog(ancestorText)) {
                        bestText = ancestorText;
                        // Mark ancestor as seen to prevent child duplication
                        actSeen.add(ancestor);
                    }
                    ancestor = ancestor.parentElement;
                }
            }
            // Ancestor dedup: skip if a parent was already captured as activity
            var dup = false;
            var p = el.parentElement;
            while (p && p !== scope) {
                if (actSeen.has(p)) { dup = true; break; }
                p = p.parentElement;
            }
            if (dup) continue;
            actSeen.add(el);
            var aKind = isThinking ? 'thinking' : (looksLikeToolOutput(bestText) ? 'tool-result' : 'tool-call');
            segments.push({
                kind: aKind,
                text: bestText.slice(0, isThinking ? 2000 : 300),
                role: 'assistant',
                messageIndex: 0,
                domPath: 'activity-scan:nth(' + ai + ')'
            });
        }
    }

    // Pass 3: Extract feedback buttons
    var feedbackBtns = scope.querySelectorAll('[class*="feedback"] button, footer button');
    for (var fi = 0; fi < feedbackBtns.length; fi++) {
        var btnText = (feedbackBtns[fi].textContent || '').trim();
        if (btnText === 'Good' || btnText === 'Bad') {
            segments.push({
                kind: 'feedback',
                text: btnText,
                role: 'assistant',
                messageIndex: 0,
                domPath: 'feedback button.' + btnText.toLowerCase()
            });
        }
    }

    if (!bodyFound && segments.length === 0) return null;

    return {
        source: 'dom-structured',
        extractedAt: Date.now(),
        segments: segments
    };
})()`
}

// isValidPayload validates and converts an arbitrary payload value into *AssistantDomSegmentPayload.
func isValidPayload(value any) (*AssistantDomSegmentPayload, bool) {
	if value == nil {
		return nil, false
	}

	switch v := value.(type) {
	case *AssistantDomSegmentPayload:
		if v != nil && v.Source == "dom-structured" && v.Segments != nil {
			return v, true
		}
		return nil, false
	case AssistantDomSegmentPayload:
		if v.Source == "dom-structured" && v.Segments != nil {
			return &v, true
		}
		return nil, false
	case json.RawMessage:
		var p AssistantDomSegmentPayload
		if err := json.Unmarshal(v, &p); err == nil && p.Source == "dom-structured" && p.Segments != nil {
			return &p, true
		}
		return nil, false
	case []byte:
		var p AssistantDomSegmentPayload
		if err := json.Unmarshal(v, &p); err == nil && p.Source == "dom-structured" && p.Segments != nil {
			return &p, true
		}
		return nil, false
	case string:
		var p AssistantDomSegmentPayload
		if err := json.Unmarshal([]byte(v), &p); err == nil && p.Source == "dom-structured" && p.Segments != nil {
			return &p, true
		}
		return nil, false
	case map[string]any:
		src, ok := v["source"].(string)
		if !ok || src != "dom-structured" {
			return nil, false
		}
		if _, ok := v["segments"]; !ok {
			return nil, false
		}
		data, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var p AssistantDomSegmentPayload
		if err := json.Unmarshal(data, &p); err == nil && p.Source == "dom-structured" && p.Segments != nil {
			return &p, true
		}
		return nil, false
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		var p AssistantDomSegmentPayload
		if err := json.Unmarshal(data, &p); err == nil && p.Source == "dom-structured" && p.Segments != nil {
			return &p, true
		}
		return nil, false
	}
}

// ClassifyAssistantSegments classifies a payload (AssistantDomSegmentPayload, *AssistantDomSegmentPayload,
// map[string]any from json.Unmarshal, or nil) into Telegram HTML formatted output text, activity lines,
// thinking lines, feedback actions, and diagnostics.
// If payload is invalid, it returns source="legacy-fallback" and fallbackReason="invalid-payload".
func ClassifyAssistantSegments(payload any) ClassifyResult {
	p, ok := isValidPayload(payload)
	if !ok {
		return ClassifyResult{
			FinalOutputText: "",
			ActivityLines:   []string{},
			ThinkingLines:   []string{},
			Feedback:        []string{},
			Diagnostics: ClassifyDiagnostics{
				Source:         "legacy-fallback",
				SegmentCounts:  map[string]int{},
				FallbackReason: "invalid-payload",
			},
		}
	}

	bodyTexts := make([]string, 0)
	activityLines := make([]string, 0)
	thinkingLines := make([]string, 0)
	feedbackTexts := make([]string, 0)
	segmentCounts := make(map[string]int)

	for _, seg := range p.Segments {
		segmentCounts[string(seg.Kind)]++

		textTrimmed := strings.TrimSpace(seg.Text)
		switch seg.Kind {
		case SegmentKindAssistantBody:
			if textTrimmed != "" {
				bodyTexts = append(bodyTexts, seg.Text)
			}
		case SegmentKindThinking, SegmentKindThinkingContent:
			if textTrimmed != "" {
				thinkingLines = append(thinkingLines, textTrimmed)
			}
		case SegmentKindToolCall, SegmentKindToolResult:
			if textTrimmed != "" {
				activityLines = append(activityLines, textTrimmed)
			}
		case SegmentKindFeedback:
			if textTrimmed != "" {
				feedbackTexts = append(feedbackTexts, textTrimmed)
			}
		}
	}

	rawBody := strings.Join(bodyTexts, "\n\n")
	finalOutputText := formatter.HtmlToTelegramHtml(rawBody)

	return ClassifyResult{
		FinalOutputText: finalOutputText,
		ActivityLines:   activityLines,
		ThinkingLines:   thinkingLines,
		Feedback:        feedbackTexts,
		Diagnostics: ClassifyDiagnostics{
			Source:        "dom-structured",
			SegmentCounts: segmentCounts,
		},
	}
}
