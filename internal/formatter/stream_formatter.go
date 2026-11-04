package formatter

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reGenericStatus = regexp.MustCompile(`^(?:fast|planning|plan|generating\.*|thinking\.*|processing\.*|working\.*)$`)
	reSingleWordLog = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,24}[.!…]?$`)
	reAnalyzedTrace = regexp.MustCompile(`^analyzed`)
)

// BuildModeModelLines generates mode/model lines for initial status display.
// Consolidates into a single line if Fast and Plan models are the same (case-insensitive after trim).
func BuildModeModelLines(modeName string, fastModel string, planModel string) []string {
	lines := []string{fmt.Sprintf("Chế độ hiện tại: %s", modeName)}
	if strings.EqualFold(strings.TrimSpace(fastModel), strings.TrimSpace(planModel)) {
		lines = append(lines, fmt.Sprintf("Mô hình: %s", fastModel))
		return lines
	}

	lines = append(lines, fmt.Sprintf("Mô hình Nhanh: %s", fastModel))
	lines = append(lines, fmt.Sprintf("Mô hình Kế hoạch: %s", planModel))
	return lines
}

// ShouldSkipActivityLog filters out activity logs that tend to be noise in message display.
func ShouldSkipActivityLog(activity string, modeName string, modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(activity))
	if normalized == "" {
		return true
	}

	modeLower := strings.ToLower(strings.TrimSpace(modeName))
	modelLower := strings.ToLower(strings.TrimSpace(modelName))
	if normalized == modeLower || normalized == modelLower {
		return true
	}

	if reGenericStatus.MatchString(normalized) {
		return true
	}

	// Single-word logs that tend to be noise (create / ready / pull. etc.)
	if reSingleWordLog.MatchString(normalized) {
		return true
	}

	// Detailed trace for file reading operations (Analyzed....)
	if reAnalyzedTrace.MatchString(normalized) {
		return true
	}

	return false
}

// SplitForEmbedDescription splits long text into multiple chunks for Embed description (default maxLength = 3500).
func SplitForEmbedDescription(text string, maxLength int) []string {
	if maxLength <= 0 {
		maxLength = 3500
	}

	textRunes := []rune(text)
	if len(textRunes) <= maxLength {
		return []string{text}
	}

	lines := strings.Split(text, "\n")
	var chunks []string
	current := ""

	flush := func() {
		if current == "" {
			return
		}
		chunks = append(chunks, current)
		current = ""
	}

	for _, line := range lines {
		candidate := line
		if current != "" {
			candidate = current + "\n" + line
		}

		if len([]rune(candidate)) <= maxLength {
			current = candidate
			continue
		}

		flush()
		lineRunes := []rune(line)
		if len(lineRunes) <= maxLength {
			current = line
			continue
		}

		cursor := 0
		for cursor < len(lineRunes) {
			end := cursor + maxLength
			if end > len(lineRunes) {
				end = len(lineRunes)
			}
			chunks = append(chunks, string(lineRunes[cursor:end]))
			cursor += maxLength
		}
	}

	flush()
	if len(chunks) > 0 {
		return chunks
	}
	return []string{""}
}

// FitForSingleEmbedDescription fits text within the limit for a single Embed description (default maxLength = 3500).
// When exceeding the limit, truncates the beginning and prioritizes displaying the most recent tail with prefix "...(phần đầu được lược bớt)\n".
func FitForSingleEmbedDescription(text string, maxLength int) string {
	if maxLength <= 0 {
		maxLength = 3500
	}

	textRunes := []rune(text)
	if len(textRunes) <= maxLength {
		return text
	}

	prefix := "...(phần đầu được lược bớt)\n"
	prefixRunes := []rune(prefix)

	tailLength := maxLength - len(prefixRunes)
	if tailLength < 0 {
		tailLength = 0
	}

	var tail string
	if tailLength > 0 {
		if tailLength >= len(textRunes) {
			tail = text
		} else {
			tail = string(textRunes[len(textRunes)-tailLength:])
		}
	}

	return prefix + tail
}
