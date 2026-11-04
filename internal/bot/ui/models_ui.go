package ui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	ModelBtnPrefix       = "model_btn_"
	ModelExhaustedPrefix = "model_exhausted_"
	ModelRefreshBtn      = "model_refresh_btn"
	MaxModelButtons      = 24
)

// QuotaInfo represents the quota percentage and reset time for a model.
type QuotaInfo struct {
	RemainingFraction *float64
	ResetTime         string
}

// ModelQuotaEntry represents a model quota item returned from QuotaService.
type ModelQuotaEntry struct {
	Label     string
	Model     string
	QuotaInfo *QuotaInfo
}

// ModelsCdpProvider abstracts CDP model methods.
type ModelsCdpProvider interface {
	GetUiModels(ctx context.Context) ([]string, error)
	GetCurrentModel(ctx context.Context) (string, error)
}

// ModelsUIDeps encapsulates dependencies for SendModelsUI.
type ModelsUIDeps struct {
	GetCurrentCdp func() ModelsCdpProvider
	FetchQuota    func(ctx context.Context) ([]ModelQuotaEntry, error)
	NowFunc       func() time.Time
}

func normalizeModelName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '-' || r == '_' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func findQuotaEntry(quotaData []ModelQuotaEntry, mName string) *ModelQuotaEntry {
	if mName == "" {
		return nil
	}
	nName := normalizeModelName(mName)
	for i := range quotaData {
		q := &quotaData[i]
		nLabel := normalizeModelName(q.Label)
		nModel := normalizeModelName(q.Model)

		if nLabel == nName || nModel == nName {
			return q
		}
		if nLabel != "" && (strings.Contains(nName, nLabel) || strings.Contains(nLabel, nName)) {
			return q
		}
		if nModel != "" && (strings.Contains(nName, nModel) || strings.Contains(nModel, nName)) {
			return q
		}
	}
	return nil
}

func parseResetTime(resetTimeStr string) (time.Time, bool) {
	if resetTimeStr == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, resetTimeStr); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// BuildModelsUI queries models, current model, and quotas, returning nil if models list is empty.
func BuildModelsUI(
	ctx context.Context,
	cdp ModelsCdpProvider,
	fetchQuota func(ctx context.Context) ([]ModelQuotaEntry, error),
	nowFunc ...func() time.Time,
) (*UIPayload, error) {
	if cdp == nil {
		return nil, nil
	}

	models, err := cdp.GetUiModels(ctx)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, nil
	}

	currentModel, _ := cdp.GetCurrentModel(ctx)

	var quotaData []ModelQuotaEntry
	if fetchQuota != nil {
		qd, err := fetchQuota(ctx)
		if err == nil {
			quotaData = qd
		}
	}

	now := time.Now()
	if len(nowFunc) > 0 && nowFunc[0] != nil {
		now = nowFunc[0]()
	}

	isExhausted := func(mName string) bool {
		q := findQuotaEntry(quotaData, mName)
		if q == nil || q.QuotaInfo == nil || q.QuotaInfo.RemainingFraction == nil {
			return false
		}
		rem := *q.QuotaInfo.RemainingFraction
		return !math.IsNaN(rem) && rem <= 0
	}

	formatQuota := func(mName string, current bool) string {
		prefix := "⬜"
		if current {
			prefix = "✅"
		}
		if mName == "" {
			return prefix + " Unknown"
		}

		q := findQuotaEntry(quotaData, mName)
		if q == nil || q.QuotaInfo == nil {
			return prefix + " " + mName
		}

		timeStr := "Ready"
		if resetTime, ok := parseResetTime(q.QuotaInfo.ResetTime); ok {
			diffMs := resetTime.Sub(now).Milliseconds()
			if diffMs > 0 {
				mins := int(math.Ceil(float64(diffMs) / 60000.0))
				if mins < 60 {
					timeStr = fmt.Sprintf("%dm", mins)
				} else {
					timeStr = fmt.Sprintf("%dh %dm", mins/60, mins%60)
				}
			}
		}

		rem := q.QuotaInfo.RemainingFraction
		if rem != nil && !math.IsNaN(*rem) {
			percent := int(math.Round(*rem * 100))
			icon := "🟢"
			if percent <= 0 {
				icon = "⛔"
			} else if percent <= 20 {
				icon = "🔴"
			} else if percent <= 50 {
				icon = "🟡"
			}
			quotaStr := fmt.Sprintf("%d%%", percent)
			if percent <= 0 {
				quotaStr = "Exhausted"
			}
			return fmt.Sprintf("%s %s %s %s (⏱️ %s)", prefix, mName, icon, quotaStr, timeStr)
		}

		return fmt.Sprintf("%s %s ❓ N/A (⏱️ %s)", prefix, mName, timeStr)
	}

	currentModelFormatted := "Unknown"
	if currentModel != "" {
		currentModelFormatted = formatQuota(currentModel, true)
	}

	lines := make([]string, len(models))
	for i, m := range models {
		lines[i] = EscapeHTML(formatQuota(m, m == currentModel))
	}

	text := fmt.Sprintf(
		"<b>Model Management</b>\n\n<b>Current Model:</b>\n%s\n\n<b>Available Models (%d)</b>\n%s",
		EscapeHTML(currentModelFormatted),
		len(models),
		strings.Join(lines, "\n"),
	)

	keyboard := NewInlineKeyboard()
	limit := len(models)
	if limit > MaxModelButtons {
		limit = MaxModelButtons
	}

	for _, mName := range models[:limit] {
		exhausted := isExhausted(mName)
		safeName := mName
		if len(safeName) > 40 {
			safeName = safeName[:37] + "..."
		}
		label := safeName
		if exhausted {
			label = "⛔ " + safeName
		}
		maxNameLen := 64 - len(ModelExhaustedPrefix) // 48
		cbName := mName
		if len(cbName) > maxNameLen {
			cbName = cbName[:maxNameLen]
		}
		cbData := ModelBtnPrefix + cbName
		if exhausted {
			cbData = ModelExhaustedPrefix + cbName
		}
		keyboard.Text(label, cbData).Row()
	}
	keyboard.Text("🔄 Refresh", ModelRefreshBtn).Row()
	keyboard.Clean()

	return &UIPayload{
		Text:     text,
		Keyboard: keyboard,
	}, nil
}

// SendModelsUI validates CDP connection, invokes BuildModelsUI and dispatches through sendFn.
func SendModelsUI(
	ctx context.Context,
	sendFn SendUIFunc,
	deps ModelsUIDeps,
) error {
	if deps.GetCurrentCdp == nil || deps.GetCurrentCdp() == nil {
		if sendFn != nil {
			return sendFn(ctx, "Not connected to CDP.", NewInlineKeyboard())
		}
		return nil
	}

	cdp := deps.GetCurrentCdp()
	var payload *UIPayload
	var err error
	if deps.NowFunc != nil {
		payload, err = BuildModelsUI(ctx, cdp, deps.FetchQuota, deps.NowFunc)
	} else {
		payload, err = BuildModelsUI(ctx, cdp, deps.FetchQuota)
	}
	if err != nil {
		return err
	}

	if payload == nil {
		if sendFn != nil {
			return sendFn(ctx, "Failed to retrieve model list from Antigravity.", NewInlineKeyboard())
		}
		return nil
	}

	if sendFn != nil {
		return sendFn(ctx, payload.Text, payload.Keyboard)
	}
	return nil
}
