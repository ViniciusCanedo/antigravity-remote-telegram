package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// GENERATE_TITLE_SCRIPT is evaluated via CDP Runtime.evaluate to generate a session title.
const GENERATE_TITLE_SCRIPT = `(async (userPrompt) => {
    try {
        // Generate title using Antigravity's internal fetch API
        // Look for configuration from __NEXT_DATA__ or window.__remixContext
        const configs = [
            window.__NEXT_DATA__,
            window.__remixContext,
            window.__APP_CONFIG__,
        ].filter(Boolean);

        // Fallback: extract leading text from prompt
        const fallbackTitle = userPrompt
            .replace(/^\[.*?\]\n?/, '')
            .substring(0, 40)
            .trim();

        return { ok: true, title: fallbackTitle, method: 'text-extract' };
    } catch (e) {
        return { ok: false, error: e.message };
    }
})`

var (
	reWorkspacePrefix    = regexp.MustCompile(`^\[ワークスペース:.*?\]\n?`)
	reWhitespace         = regexp.MustCompile(`\s+`)
	reAllowedTitleRunes  = regexp.MustCompile(`[^a-z0-9\-_\x{3000}-\x{303f}\x{3040}-\x{309f}\x{30a0}-\x{30ff}\x{ff00}-\x{ff9f}\x{4e00}-\x{9faf}]`)
	reConsecutiveHyphens = regexp.MustCompile(`-{2,}`)
	reTrimHyphens        = regexp.MustCompile(`^-+|-+$`)
)

// CdpEvaluator defines the minimal CDP interface needed by TitleGeneratorService and PromptDispatcher.
type CdpEvaluator interface {
	GetPrimaryContextId() *int
	Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)
	GetCurrentWorkspaceName() string
}

// TitleGeneratorService generates short session/topic titles.
type TitleGeneratorService struct{}

// NewTitleGeneratorService creates a new TitleGeneratorService.
func NewTitleGeneratorService() *TitleGeneratorService {
	return &TitleGeneratorService{}
}

// GenerateTitle generates a title from the user's prompt using CDP if available,
// falling back to sanitized text extraction.
func (s *TitleGeneratorService) GenerateTitle(ctx context.Context, prompt string, cdpService ...CdpEvaluator) string {
	if len(cdpService) > 0 && cdpService[0] != nil {
		title := s.generateViaCdp(ctx, prompt, cdpService[0])
		if title != "" {
			return title
		}
	}

	return s.extractTitleFromText(prompt)
}

func (s *TitleGeneratorService) generateViaCdp(ctx context.Context, prompt string, cdp CdpEvaluator) string {
	cleanPrompt := s.stripWorkspacePrefix(prompt)
	jsonPrompt, err := json.Marshal(cleanPrompt)
	if err != nil {
		return ""
	}

	callParams := map[string]any{
		"expression":    fmt.Sprintf("%s(%s)", GENERATE_TITLE_SCRIPT, string(jsonPrompt)),
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if contextId := cdp.GetPrimaryContextId(); contextId != nil {
		callParams["contextId"] = *contextId
	}

	result, err := cdp.Call(ctx, "Runtime.evaluate", callParams)
	if err != nil {
		return ""
	}

	resMap, _ := result["result"].(map[string]any)
	valMap, _ := resMap["value"].(map[string]any)

	okVal, _ := valMap["ok"].(bool)
	titleVal, _ := valMap["title"].(string)

	if okVal && titleVal != "" {
		return s.SanitizeForChannelName(titleVal)
	}

	return ""
}

func (s *TitleGeneratorService) extractTitleFromText(prompt string) string {
	cleanPrompt := s.stripWorkspacePrefix(prompt)
	runes := []rune(cleanPrompt)
	if len(runes) > 40 {
		runes = runes[:40]
	}
	truncated := strings.TrimSpace(string(runes))
	return s.SanitizeForChannelName(truncated)
}

func (s *TitleGeneratorService) stripWorkspacePrefix(prompt string) string {
	return reWorkspacePrefix.ReplaceAllString(prompt, "")
}

// SanitizeForChannelName normalizes text for Telegram channel/topic naming:
// lowercase, replaces spaces/symbols with hyphens, preserves CJK runes, collapses hyphens,
// trims leading/trailing hyphens, and truncates to 80 runes (defaulting to "untitled").
func (s *TitleGeneratorService) SanitizeForChannelName(text string) string {
	sanitized := strings.ToLower(text)
	sanitized = reWhitespace.ReplaceAllString(sanitized, "-")
	sanitized = reAllowedTitleRunes.ReplaceAllString(sanitized, "-")
	sanitized = reConsecutiveHyphens.ReplaceAllString(sanitized, "-")
	sanitized = reTrimHyphens.ReplaceAllString(sanitized, "")

	runes := []rune(sanitized)
	if len(runes) > 80 {
		sanitized = string(runes[:80])
	}

	if sanitized == "" {
		return "untitled"
	}
	return sanitized
}
