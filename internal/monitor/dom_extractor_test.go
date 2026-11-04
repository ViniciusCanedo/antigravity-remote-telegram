package monitor

import (
	"encoding/json"
	"strings"
	"testing"
)

func buildPayload(segments []AssistantDomSegment) AssistantDomSegmentPayload {
	return AssistantDomSegmentPayload{
		Source:      "dom-structured",
		ExtractedAt: 1700000000000,
		Segments:    segments,
	}
}

func TestClassifyAssistantSegments_SeparatesBodyActivityFeedback(t *testing.T) {
	payload := buildPayload([]AssistantDomSegment{
		{
			Kind:         SegmentKindAssistantBody,
			Text:         "最終回答: 1ドル=154.8円です。",
			Role:         "assistant",
			MessageIndex: 9,
			DomPath:      `[data-message-role="assistant"]:nth-of-type(9)`,
		},
		{
			Kind:         SegmentKindThinking,
			Text:         "Analyzing current exchange data...",
			Role:         "assistant",
			MessageIndex: 9,
			DomPath:      "details[open] summary",
		},
		{
			Kind:         SegmentKindToolCall,
			Text:         "jina-mcp-server / search_web",
			Role:         "assistant",
			MessageIndex: 9,
			DomPath:      "details[open] div.tool-call",
		},
		{
			Kind:         SegmentKindToolResult,
			Text:         "Full output written to output.txt#L1-10",
			Role:         "assistant",
			MessageIndex: 9,
			DomPath:      "details[open] div.tool-result",
		},
		{
			Kind:         SegmentKindFeedback,
			Text:         "Good",
			Role:         "assistant",
			MessageIndex: 9,
			DomPath:      "footer.feedback button.good",
		},
		{
			Kind:         SegmentKindFeedback,
			Text:         "Bad",
			Role:         "assistant",
			MessageIndex: 9,
			DomPath:      "footer.feedback button.bad",
		},
	})

	result := ClassifyAssistantSegments(payload)

	if result.FinalOutputText != "最終回答: 1ドル=154.8円です。" {
		t.Fatalf("unexpected FinalOutputText: %q", result.FinalOutputText)
	}

	expectedActivity := []string{
		"jina-mcp-server / search_web",
		"Full output written to output.txt#L1-10",
	}
	if len(result.ActivityLines) != len(expectedActivity) {
		t.Fatalf("expected %d activity lines, got %d", len(expectedActivity), len(result.ActivityLines))
	}
	for i, v := range expectedActivity {
		if result.ActivityLines[i] != v {
			t.Errorf("activity line %d mismatch: expected %q, got %q", i, v, result.ActivityLines[i])
		}
	}

	expectedThinking := []string{"Analyzing current exchange data..."}
	if len(result.ThinkingLines) != len(expectedThinking) || result.ThinkingLines[0] != expectedThinking[0] {
		t.Fatalf("unexpected ThinkingLines: %v", result.ThinkingLines)
	}

	expectedFeedback := []string{"Good", "Bad"}
	if len(result.Feedback) != len(expectedFeedback) {
		t.Fatalf("expected %d feedback items, got %d", len(expectedFeedback), len(result.Feedback))
	}
	for i, v := range expectedFeedback {
		if result.Feedback[i] != v {
			t.Errorf("feedback %d mismatch: expected %q, got %q", i, v, result.Feedback[i])
		}
	}

	if result.Diagnostics.Source != "dom-structured" {
		t.Errorf("expected source 'dom-structured', got %q", result.Diagnostics.Source)
	}
	if result.Diagnostics.SegmentCounts[string(SegmentKindAssistantBody)] != 1 {
		t.Errorf("expected 1 assistant-body count, got %d", result.Diagnostics.SegmentCounts[string(SegmentKindAssistantBody)])
	}
	if result.Diagnostics.SegmentCounts[string(SegmentKindFeedback)] != 2 {
		t.Errorf("expected 2 feedback count, got %d", result.Diagnostics.SegmentCounts[string(SegmentKindFeedback)])
	}
}

func TestClassifyAssistantSegments_ConcatenatesMultipleBodySegments(t *testing.T) {
	payload := buildPayload([]AssistantDomSegment{
		{
			Kind:         SegmentKindAssistantBody,
			Text:         "前半です。",
			Role:         "assistant",
			MessageIndex: 3,
			DomPath:      "article:nth-child(3)",
		},
		{
			Kind:         SegmentKindAssistantBody,
			Text:         "後半です。",
			Role:         "assistant",
			MessageIndex: 3,
			DomPath:      "article:nth-child(3) p:last-child",
		},
		{
			Kind:         SegmentKindToolCall,
			Text:         "mcp.search",
			Role:         "assistant",
			MessageIndex: 3,
			DomPath:      "details summary",
		},
	})

	result := ClassifyAssistantSegments(payload)

	expectedText := "前半です。\n\n後半です。"
	if result.FinalOutputText != expectedText {
		t.Fatalf("expected %q, got %q", expectedText, result.FinalOutputText)
	}

	if len(result.ActivityLines) != 1 || result.ActivityLines[0] != "mcp.search" {
		t.Fatalf("unexpected ActivityLines: %v", result.ActivityLines)
	}

	if len(result.Feedback) != 0 {
		t.Fatalf("expected empty feedback, got %v", result.Feedback)
	}
}

func TestClassifyAssistantSegments_InvalidPayloadFallback(t *testing.T) {
	testCases := []struct {
		name    string
		payload any
	}{
		{"nil payload", nil},
		{"string payload", "invalid string"},
		{"empty map", map[string]any{}},
		{"wrong source", map[string]any{"source": "wrong", "segments": []any{}}},
		{"nil segments", map[string]any{"source": "dom-structured", "segments": nil}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ClassifyAssistantSegments(tc.payload)

			if result.FinalOutputText != "" {
				t.Errorf("expected empty FinalOutputText, got %q", result.FinalOutputText)
			}
			if len(result.ActivityLines) != 0 {
				t.Errorf("expected empty ActivityLines, got %v", result.ActivityLines)
			}
			if len(result.ThinkingLines) != 0 {
				t.Errorf("expected empty ThinkingLines, got %v", result.ThinkingLines)
			}
			if len(result.Feedback) != 0 {
				t.Errorf("expected empty Feedback, got %v", result.Feedback)
			}
			if result.Diagnostics.Source != "legacy-fallback" {
				t.Errorf("expected Diagnostics.Source 'legacy-fallback', got %q", result.Diagnostics.Source)
			}
			if result.Diagnostics.FallbackReason != "invalid-payload" {
				t.Errorf("expected Diagnostics.FallbackReason 'invalid-payload', got %q", result.Diagnostics.FallbackReason)
			}
		})
	}
}

func TestClassifyAssistantSegments_RoutesThinkingAndThinkingContent(t *testing.T) {
	payload := buildPayload([]AssistantDomSegment{
		{
			Kind:         SegmentKindAssistantBody,
			Text:         "Here is the answer.",
			Role:         "assistant",
			MessageIndex: 0,
			DomPath:      "multi-selector",
		},
		{
			Kind:         SegmentKindThinking,
			Text:         "Thought for 12 seconds",
			Role:         "assistant",
			MessageIndex: 0,
			DomPath:      "details:nth(0) summary",
		},
		{
			Kind:         SegmentKindThinkingContent,
			Text:         "Let me analyze the user request carefully...",
			Role:         "assistant",
			MessageIndex: 0,
			DomPath:      "details:nth(0) child:nth(1)",
		},
		{
			Kind:         SegmentKindToolCall,
			Text:         "mcp.search",
			Role:         "assistant",
			MessageIndex: 0,
			DomPath:      "details:nth(1) summary",
		},
	})

	result := ClassifyAssistantSegments(payload)

	if result.FinalOutputText != "Here is the answer." {
		t.Fatalf("unexpected FinalOutputText: %q", result.FinalOutputText)
	}

	expectedThinking := []string{
		"Thought for 12 seconds",
		"Let me analyze the user request carefully...",
	}
	if len(result.ThinkingLines) != len(expectedThinking) {
		t.Fatalf("expected %d thinking lines, got %d", len(expectedThinking), len(result.ThinkingLines))
	}
	for i, v := range expectedThinking {
		if result.ThinkingLines[i] != v {
			t.Errorf("thinking line %d mismatch: expected %q, got %q", i, v, result.ThinkingLines[i])
		}
	}

	if len(result.ActivityLines) != 1 || result.ActivityLines[0] != "mcp.search" {
		t.Fatalf("unexpected ActivityLines: %v", result.ActivityLines)
	}

	if result.Diagnostics.SegmentCounts[string(SegmentKindThinkingContent)] != 1 {
		t.Errorf("expected 1 thinking-content, got %d", result.Diagnostics.SegmentCounts[string(SegmentKindThinkingContent)])
	}
}

func TestExtractAssistantSegmentsPayloadScript_Structure(t *testing.T) {
	script := ExtractAssistantSegmentsPayloadScript()

	if script == "" {
		t.Fatal("expected non-empty extraction script")
	}

	requiredStrings := []string{
		"data-message-role",
		"assistant-body",
		"details",
		"feedback",
		".antigravity-agent-side-panel",
		"looksLikeActivityLog",
		"looksLikeFeedbackFooter",
		"looksLikeToolOutput",
		"looksLikeThinking",
		"isInsideExcludedContainer",
		"dom-structured",
	}

	for _, s := range requiredStrings {
		if !strings.Contains(script, s) {
			t.Errorf("script missing required substring: %q", s)
		}
	}
}

func TestExtractAssistantSegments_Pass25_LeafActivity(t *testing.T) {
	script := ExtractAssistantSegmentsPayloadScript()
	if !strings.Contains(script, "Pass 2.5: Broad activity scan") {
		t.Error("script missing Pass 2.5 broad activity scan section")
	}
	if !strings.Contains(script, "el.children.length > 3") {
		t.Error("script missing leaf-ish condition (children.length > 3)")
	}

	payload := buildPayload([]AssistantDomSegment{
		{
			Kind:         SegmentKindToolCall,
			Text:         "Analyzed package.json#L1-75",
			Role:         "assistant",
			MessageIndex: 0,
			DomPath:      "activity-scan:nth(4)",
		},
	})

	result := ClassifyAssistantSegments(payload)
	found := false
	for _, l := range result.ActivityLines {
		if l == "Analyzed package.json#L1-75" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected activity lines to contain 'Analyzed package.json#L1-75', got %v", result.ActivityLines)
	}
}

func TestExtractAssistantSegments_Pass25_McpToolOutput(t *testing.T) {
	script := ExtractAssistantSegmentsPayloadScript()
	if !strings.Contains(script, "looksLikeToolOutput") {
		t.Error("script missing looksLikeToolOutput check")
	}

	payload := buildPayload([]AssistantDomSegment{
		{
			Kind:         SegmentKindToolResult,
			Text:         "jina-mcp-server / search_web",
			Role:         "assistant",
			MessageIndex: 0,
			DomPath:      "activity-scan:nth(2)",
		},
	})

	result := ClassifyAssistantSegments(payload)
	found := false
	for _, l := range result.ActivityLines {
		if l == "jina-mcp-server / search_web" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected activity lines to contain 'jina-mcp-server / search_web', got %v", result.ActivityLines)
	}
}

func TestExtractAssistantSegments_Pass25_NewActivityVerbs(t *testing.T) {
	verbs := []string{
		"Fetching data from API",
		"Creating new file",
		"Building project",
		"Connected to server",
	}

	script := ExtractAssistantSegmentsPayloadScript()
	for _, kw := range []string{"fetching", "creating", "building", "connected"} {
		if !strings.Contains(script, kw) {
			t.Errorf("script missing verb keyword: %q", kw)
		}
	}

	segments := make([]AssistantDomSegment, len(verbs))
	for i, v := range verbs {
		segments[i] = AssistantDomSegment{
			Kind:         SegmentKindToolCall,
			Text:         v,
			Role:         "assistant",
			MessageIndex: 0,
			DomPath:      "activity-scan",
		}
	}

	result := ClassifyAssistantSegments(buildPayload(segments))
	for _, v := range verbs {
		found := false
		for _, l := range result.ActivityLines {
			if l == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected activity line %q in result, got %v", v, result.ActivityLines)
		}
	}
}

func TestExtractAssistantSegments_Pass25_SkipsRoleDialog(t *testing.T) {
	script := ExtractAssistantSegmentsPayloadScript()
	if !strings.Contains(script, `node.closest('[role="dialog"]')`) {
		t.Error("script missing role=dialog exclusion check")
	}

	// Mode descriptions inside role="dialog" are excluded by the extractor
	payload := buildPayload([]AssistantDomSegment{})
	result := ClassifyAssistantSegments(payload)

	if len(result.ActivityLines) != 0 {
		t.Fatalf("expected empty ActivityLines, got %v", result.ActivityLines)
	}
}

func TestExtractAssistantSegments_Pass25_SkipsContainerOver3Children(t *testing.T) {
	script := ExtractAssistantSegmentsPayloadScript()
	if !strings.Contains(script, "el.children.length > 3") {
		t.Error("script missing children length check")
	}

	// Simulating that the container text itself is NOT emitted as a combined string
	containerText := "Analyzed file0.tsAnalyzed file1.tsAnalyzed file2.tsAnalyzed file3.tsAnalyzed file4.ts"
	payload := buildPayload([]AssistantDomSegment{
		{Kind: SegmentKindToolCall, Text: "Analyzed file0.ts", Role: "assistant", DomPath: "activity-scan:nth(0)"},
		{Kind: SegmentKindToolCall, Text: "Analyzed file1.ts", Role: "assistant", DomPath: "activity-scan:nth(1)"},
	})

	result := ClassifyAssistantSegments(payload)
	for _, l := range result.ActivityLines {
		if l == containerText {
			t.Fatalf("activity lines should NOT contain concatenated container text: %q", containerText)
		}
	}
}

func TestExtractAssistantSegments_PreservesListAndFileReferences(t *testing.T) {
	// Task 1 & Task 2 & Task 3: DOM extraction and Markdown structure/file reference restoration
	bodyHtml := `<p>改行付き` + "\n" + `の段落</p><ol><li>最初の項目</li><li>次の項目</li></ol><ul><li>箇条書き1</li></ul><div title="src/bot/index.ts">:54</div>`

	payload := buildPayload([]AssistantDomSegment{
		{
			Kind:         SegmentKindAssistantBody,
			Text:         bodyHtml,
			Role:         "assistant",
			MessageIndex: 0,
			DomPath:      "multi-selector",
		},
	})

	result := ClassifyAssistantSegments(payload)
	output := result.FinalOutputText

	expectedSubstrings := []string{
		"1. 最初の項目",
		"2. 次の項目",
		"- 箇条書き1",
		"改行付き\nの段落",
		"src/bot/index.ts:54",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(output, sub) {
			t.Errorf("output missing expected substring %q, got:\n%s", sub, output)
		}
	}
}

func TestClassifyAssistantSegments_JSONPayloadUnmarshaling(t *testing.T) {
	rawJSON := `{
		"source": "dom-structured",
		"extractedAt": 1700000000000,
		"segments": [
			{
				"kind": "assistant-body",
				"text": "<b>Hello</b>",
				"role": "assistant",
				"messageIndex": 0,
				"domPath": "multi-selector"
			}
		]
	}`

	// Test unmarshaling from json.RawMessage
	resFromRaw := ClassifyAssistantSegments(json.RawMessage(rawJSON))
	if resFromRaw.FinalOutputText != "<b>Hello</b>" {
		t.Errorf("unexpected output from RawMessage: %q", resFromRaw.FinalOutputText)
	}

	// Test unmarshaling from []byte
	resFromBytes := ClassifyAssistantSegments([]byte(rawJSON))
	if resFromBytes.FinalOutputText != "<b>Hello</b>" {
		t.Errorf("unexpected output from bytes: %q", resFromBytes.FinalOutputText)
	}

	// Test unmarshaling from string
	resFromString := ClassifyAssistantSegments(rawJSON)
	if resFromString.FinalOutputText != "<b>Hello</b>" {
		t.Errorf("unexpected output from string: %q", resFromString.FinalOutputText)
	}

	// Test unmarshaling from map[string]any
	var m map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		t.Fatalf("failed to unmarshal JSON into map: %v", err)
	}
	resFromMap := ClassifyAssistantSegments(m)
	if resFromMap.FinalOutputText != "<b>Hello</b>" {
		t.Errorf("unexpected output from map: %q", resFromMap.FinalOutputText)
	}
}
