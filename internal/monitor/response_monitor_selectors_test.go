package monitor

import (
	"strings"
	"testing"
)

func TestResponseSelectors_StopButtonAndClickStopButton(t *testing.T) {
	// Test 1: STOP_BUTTON contains tooltip-id
	if !strings.Contains(ResponseSelectors.StopButton, "input-send-button-cancel-tooltip") {
		t.Errorf("expected STOP_BUTTON to contain input-send-button-cancel-tooltip")
	}

	// Test 2: STOP_BUTTON does NOT contain svg
	if strings.Contains(strings.ToLower(ResponseSelectors.StopButton), "svg") {
		t.Errorf("expected STOP_BUTTON to NOT contain svg")
	}

	// Test 3: STOP_BUTTON does NOT contain getBoundingClientRect
	if strings.Contains(ResponseSelectors.StopButton, "getBoundingClientRect") {
		t.Errorf("expected STOP_BUTTON to NOT contain getBoundingClientRect")
	}

	// Test 4: STOP_BUTTON does NOT contain getComputedStyle
	if strings.Contains(ResponseSelectors.StopButton, "getComputedStyle") {
		t.Errorf("expected STOP_BUTTON to NOT contain getComputedStyle")
	}

	// Test 5: STOP_BUTTON does NOT contain composer
	if strings.Contains(strings.ToLower(ResponseSelectors.StopButton), "composer") {
		t.Errorf("expected STOP_BUTTON to NOT contain composer")
	}

	// Test 6: CLICK_STOP_BUTTON contains tooltip-id
	if !strings.Contains(ResponseSelectors.ClickStopButton, "input-send-button-cancel-tooltip") {
		t.Errorf("expected CLICK_STOP_BUTTON to contain input-send-button-cancel-tooltip")
	}

	// Test 7: CLICK_STOP_BUTTON does NOT contain svg
	if strings.Contains(strings.ToLower(ResponseSelectors.ClickStopButton), "svg") {
		t.Errorf("expected CLICK_STOP_BUTTON to NOT contain svg")
	}

	// Test 8: CLICK_STOP_BUTTON does NOT contain heuristic
	if strings.Contains(strings.ToLower(ResponseSelectors.ClickStopButton), "heuristic") {
		t.Errorf("expected CLICK_STOP_BUTTON to NOT contain heuristic")
	}

	// Test 8b: STOP_BUTTON does NOT use broad substring includes matching
	if strings.Contains(ResponseSelectors.StopButton, "blob.includes") {
		t.Errorf("expected STOP_BUTTON to NOT contain blob.includes")
	}
	if strings.Contains(ResponseSelectors.StopButton, "includes(w)") {
		t.Errorf("expected STOP_BUTTON to NOT contain includes(w)")
	}

	// Test 8c: STOP_BUTTON includes Japanese stop labels
	if !strings.Contains(ResponseSelectors.StopButton, "停止") {
		t.Errorf("expected STOP_BUTTON to contain 停止")
	}
}

func TestResponseSelectors_ResponseTextAndProcessLogsAndDumpAllTexts(t *testing.T) {
	lowerRT := strings.ToLower(ResponseSelectors.ResponseText)

	// Test 12: scored selector approach
	if !strings.Contains(lowerRT, "score") && !strings.Contains(lowerRT, "priority") && !strings.Contains(lowerRT, "weight") {
		t.Errorf("expected RESPONSE_TEXT to contain score, priority or weight")
	}

	// Test 13: details exclusion via closest
	if !strings.Contains(ResponseSelectors.ResponseText, "closest('details')") {
		t.Errorf("expected RESPONSE_TEXT to contain closest('details')")
	}

	// Test 14: feedback and footer exclusion
	if !strings.Contains(lowerRT, "feedback") || !strings.Contains(lowerRT, "footer") {
		t.Errorf("expected RESPONSE_TEXT to contain feedback and footer")
	}

	// Test 15: tool output pattern filter
	if !strings.Contains(lowerRT, "looksliketooloutput") {
		t.Errorf("expected RESPONSE_TEXT to contain looksliketooloutput")
	}

	// Test 19b: RESPONSE_TEXT excludes .notify-user-container
	if !strings.Contains(ResponseSelectors.ResponseText, ".notify-user-container") {
		t.Errorf("expected RESPONSE_TEXT to contain .notify-user-container")
	}

	// Test 19c: DUMP_ALL_TEXTS excludes .notify-user-container
	if !strings.Contains(ResponseSelectors.DumpAllTexts, ".notify-user-container") {
		t.Errorf("expected DUMP_ALL_TEXTS to contain .notify-user-container")
	}

	// Test 19d: PROCESS_LOGS excludes .notify-user-container
	if !strings.Contains(ResponseSelectors.ProcessLogs, ".notify-user-container") {
		t.Errorf("expected PROCESS_LOGS to contain .notify-user-container")
	}

	// Test 20: looksLikeQuotaPopup filter
	if !strings.Contains(lowerRT, "lookslikequotapopup") {
		t.Errorf("expected RESPONSE_TEXT to contain lookslikequotapopup")
	}

	// Test 21: dismiss and upgrade keywords
	if !strings.Contains(lowerRT, "dismiss") || !strings.Contains(lowerRT, "upgrade") {
		t.Errorf("expected RESPONSE_TEXT to contain dismiss and upgrade")
	}

	// Test 22: DUMP_ALL_TEXTS classifies quota popup
	lowerDump := strings.ToLower(ResponseSelectors.DumpAllTexts)
	if !strings.Contains(lowerDump, "quota-popup") {
		t.Errorf("expected DUMP_ALL_TEXTS to contain quota-popup")
	}

	// Test 25: RESPONSE_TEXT catches inline exhausted quota pattern
	if !strings.Contains(lowerRT, "exhausted your quota") {
		t.Errorf("expected RESPONSE_TEXT to contain 'exhausted your quota'")
	}

	// Test 26: DUMP_ALL_TEXTS catches inline exhausted quota pattern
	if !strings.Contains(lowerDump, "exhausted your quota") {
		t.Errorf("expected DUMP_ALL_TEXTS to contain 'exhausted your quota'")
	}
}

func TestResponseSelectors_QuotaError(t *testing.T) {
	// Test 16: h3 span text-based detection
	if !strings.Contains(ResponseSelectors.QuotaError, "h3 span") || !strings.Contains(ResponseSelectors.QuotaError, "h3") {
		t.Errorf("expected QUOTA_ERROR to contain h3 span and h3")
	}

	// Test 17: quota keywords
	lowerQE := strings.ToLower(ResponseSelectors.QuotaError)
	if !strings.Contains(lowerQE, "model quota reached") || !strings.Contains(lowerQE, "rate limit") || !strings.Contains(lowerQE, "quota exceeded") {
		t.Errorf("expected QUOTA_ERROR to contain quota keywords")
	}

	// Test 18: excludes rendered-markdown and prose
	if !strings.Contains(ResponseSelectors.QuotaError, ".rendered-markdown") || !strings.Contains(ResponseSelectors.QuotaError, ".prose") {
		t.Errorf("expected QUOTA_ERROR to contain .rendered-markdown and .prose")
	}

	// Test 19: class-based fallback selectors
	if !strings.Contains(ResponseSelectors.QuotaError, `[role="alert"]`) || !strings.Contains(ResponseSelectors.QuotaError, `[class*="error"]`) {
		t.Errorf("expected QUOTA_ERROR to contain [role=\"alert\"] and [class*=\"error\"]")
	}

	// Test 23: inline exhausted quota pattern
	if !strings.Contains(lowerQE, "exhausted your quota") {
		t.Errorf("expected QUOTA_ERROR to contain 'exhausted your quota'")
	}

	// Test 24: queries span elements
	if !strings.Contains(ResponseSelectors.QuotaError, "querySelectorAll('span')") {
		t.Errorf("expected QUOTA_ERROR to querySelectorAll('span')")
	}
}
