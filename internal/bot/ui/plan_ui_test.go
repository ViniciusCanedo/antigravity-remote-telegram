package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestBuildPlanNotificationUI_AllFourButtons(t *testing.T) {
	info := PlanningInfo{
		OpenText:    "Open",
		ProceedText: "Proceed",
		PlanTitle:   "refactor-auth.md",
		PlanSummary: "Refactor the authentication module",
		Description: "This plan refactors the auth module to use JWT tokens.",
	}

	res := BuildPlanNotificationUI(info, "my-project", "12345")

	if !strings.Contains(res.Text, "Planning Mode") {
		t.Errorf("expected 'Planning Mode', got %s", res.Text)
	}
	if !strings.Contains(res.Text, "refactor-auth.md") {
		t.Errorf("expected 'refactor-auth.md', got %s", res.Text)
	}
	if !strings.Contains(res.Text, "my-project") {
		t.Errorf("expected 'my-project', got %s", res.Text)
	}
	if !strings.Contains(res.Text, "JWT tokens") {
		t.Errorf("expected 'JWT tokens', got %s", res.Text)
	}

	rows := res.Keyboard.InlineKeyboard
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if len(rows[0]) != 2 {
		t.Errorf("expected row 0 to have 2 buttons, got %d", len(rows[0]))
	}
	if len(rows[1]) != 2 {
		t.Errorf("expected row 1 to have 2 buttons, got %d", len(rows[1]))
	}

	if !strings.Contains(rows[0][0].CallbackData, PlanViewBtn) {
		t.Errorf("expected PlanViewBtn in %s", rows[0][0].CallbackData)
	}
	if !strings.Contains(rows[0][1].CallbackData, PlanProceedBtn) {
		t.Errorf("expected PlanProceedBtn in %s", rows[0][1].CallbackData)
	}
	if !strings.Contains(rows[1][0].CallbackData, PlanEditBtn) {
		t.Errorf("expected PlanEditBtn in %s", rows[1][0].CallbackData)
	}
	if !strings.Contains(rows[1][1].CallbackData, PlanRefreshBtn) {
		t.Errorf("expected PlanRefreshBtn in %s", rows[1][1].CallbackData)
	}
}

func TestBuildPlanNotificationUI_Fallbacks(t *testing.T) {
	infoNoDesc := PlanningInfo{
		PlanSummary: "Refactor the authentication module",
		PlanTitle:   "Plan 1",
	}
	res := BuildPlanNotificationUI(infoNoDesc, "proj", "999")
	if !strings.Contains(res.Text, "Refactor the authentication module") {
		t.Errorf("expected planSummary fallback in text, got: %s", res.Text)
	}

	infoEmpty := PlanningInfo{
		PlanTitle: "Plan 1",
	}
	resEmpty := BuildPlanNotificationUI(infoEmpty, "proj", "999")
	if !strings.Contains(resEmpty.Text, "awaiting your review") {
		t.Errorf("expected default text in text, got: %s", resEmpty.Text)
	}
}

func TestPaginatePlanContent_Short(t *testing.T) {
	pages := PaginatePlanContent("Short plan content", 3500)
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if pages[0] != "Short plan content" {
		t.Errorf("expected 'Short plan content', got %q", pages[0])
	}
}

func TestPaginatePlanContent_Empty(t *testing.T) {
	pages := PaginatePlanContent("", 3500)
	if len(pages) != 1 || pages[0] != "(Empty plan)" {
		t.Errorf("expected ['(Empty plan)'], got %+v", pages)
	}

	pagesWhitespace := PaginatePlanContent("   \n\t  ", 3500)
	if len(pagesWhitespace) != 1 || pagesWhitespace[0] != "(Empty plan)" {
		t.Errorf("expected ['(Empty plan)'], got %+v", pagesWhitespace)
	}
}

func TestPaginatePlanContent_NewlineBoundaries(t *testing.T) {
	line := strings.Repeat("A", 100) + "\n"
	content := strings.Repeat(line, 50) // ~5050 chars
	pages := PaginatePlanContent(content, 2000)

	if len(pages) <= 1 {
		t.Fatalf("expected more than 1 page, got %d", len(pages))
	}
	for i, page := range pages {
		if len(page) > 2000 {
			t.Errorf("page %d exceeds pageSize (len=%d)", i, len(page))
		}
	}

	rejoined := strings.Join(pages, "\n")
	origClean := strings.ReplaceAll(content, "\n", "")
	rejoinedClean := strings.ReplaceAll(rejoined, "\n", "")
	if rejoinedClean != origClean {
		t.Errorf("content was altered during pagination")
	}
}

func TestPaginatePlanContent_HardSplit(t *testing.T) {
	content := strings.Repeat("X", 5000)
	pages := PaginatePlanContent(content, 2000)

	if len(pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(pages))
	}
	if len(pages[0]) != 2000 {
		t.Errorf("expected page 0 to have length 2000, got %d", len(pages[0]))
	}
}

func TestBuildPlanContentUI_SinglePage(t *testing.T) {
	pages := []string{"Plan content here"}
	res := BuildPlanContentUI(pages, 0, "proj", "123")

	if !strings.Contains(res.Text, "Plan Content") {
		t.Errorf("expected 'Plan Content', got %s", res.Text)
	}
	if !strings.Contains(res.Text, "Plan content here") {
		t.Errorf("expected 'Plan content here', got %s", res.Text)
	}

	pagePattern := regexp.MustCompile(`\(\d+/\d+\)`)
	if pagePattern.MatchString(res.Text) {
		t.Errorf("did not expect page numbers in single page, got %s", res.Text)
	}

	rows := res.Keyboard.InlineKeyboard
	var allButtons []InlineKeyboardButton
	for _, row := range rows {
		allButtons = append(allButtons, row...)
	}

	if len(allButtons) != 4 {
		t.Fatalf("expected 4 action buttons, got %d", len(allButtons))
	}

	foundProceed, foundEdit, foundRefresh, foundView := false, false, false, false
	for _, b := range allButtons {
		if strings.Contains(b.CallbackData, PlanProceedBtn) {
			foundProceed = true
		}
		if strings.Contains(b.CallbackData, PlanEditBtn) {
			foundEdit = true
		}
		if strings.Contains(b.CallbackData, PlanRefreshBtn) {
			foundRefresh = true
		}
		if strings.Contains(b.CallbackData, PlanViewBtn) {
			foundView = true
		}
	}

	if !foundProceed || !foundEdit || !foundRefresh || !foundView {
		t.Errorf("missing action buttons: proceed=%v edit=%v refresh=%v view=%v",
			foundProceed, foundEdit, foundRefresh, foundView)
	}
}

func TestBuildPlanContentUI_MultiPage(t *testing.T) {
	pages := []string{"Page 1", "Page 2", "Page 3"}

	// Page 0: only Next
	res1 := BuildPlanContentUI(pages, 0, "proj", "123")
	if !strings.Contains(res1.Text, "(1/3)") {
		t.Errorf("expected '(1/3)', got %s", res1.Text)
	}
	rows1 := res1.Keyboard.InlineKeyboard
	if len(rows1[0]) != 1 {
		t.Fatalf("expected 1 navigation button on page 0, got %d", len(rows1[0]))
	}
	if !strings.Contains(rows1[0][0].CallbackData, PlanPagePrefix) {
		t.Errorf("expected PlanPagePrefix, got %s", rows1[0][0].CallbackData)
	}
	if !strings.Contains(rows1[0][0].Text, "Next") {
		t.Errorf("expected 'Next', got %s", rows1[0][0].Text)
	}

	// Page 1: Prev and Next
	res2 := BuildPlanContentUI(pages, 1, "proj", "123")
	if !strings.Contains(res2.Text, "(2/3)") {
		t.Errorf("expected '(2/3)', got %s", res2.Text)
	}
	rows2 := res2.Keyboard.InlineKeyboard
	if len(rows2[0]) != 2 {
		t.Fatalf("expected 2 navigation buttons on page 1, got %d", len(rows2[0]))
	}
	if !strings.Contains(rows2[0][0].Text, "Prev") {
		t.Errorf("expected 'Prev', got %s", rows2[0][0].Text)
	}
	if !strings.Contains(rows2[0][1].Text, "Next") {
		t.Errorf("expected 'Next', got %s", rows2[0][1].Text)
	}

	// Page 2: only Prev
	res3 := BuildPlanContentUI(pages, 2, "proj", "123")
	if !strings.Contains(res3.Text, "(3/3)") {
		t.Errorf("expected '(3/3)', got %s", res3.Text)
	}
	rows3 := res3.Keyboard.InlineKeyboard
	if len(rows3[0]) != 1 {
		t.Fatalf("expected 1 navigation button on page 2, got %d", len(rows3[0]))
	}
	if !strings.Contains(rows3[0][0].Text, "Prev") {
		t.Errorf("expected 'Prev', got %s", rows3[0][0].Text)
	}
}

func TestMarkdownToTelegramHTML(t *testing.T) {
	input := `# Section 1
Here is **bold** and *italic* and ` + "`code`" + ` text.
- Item 1
* Item 2
` + "```go\nfmt.Println(\"<hello>\")\n```"

	res := MarkdownToTelegramHTML(input)

	if !strings.Contains(res, "<b>Section 1</b>") {
		t.Errorf("expected heading converted, got: %s", res)
	}
	if !strings.Contains(res, "<b>bold</b>") {
		t.Errorf("expected bold converted, got: %s", res)
	}
	if !strings.Contains(res, "<i>italic</i>") {
		t.Errorf("expected italic converted, got: %s", res)
	}
	if !strings.Contains(res, "<code>code</code>") {
		t.Errorf("expected inline code converted, got: %s", res)
	}
	if !strings.Contains(res, "• Item 1") || !strings.Contains(res, "• Item 2") {
		t.Errorf("expected bullet items converted, got: %s", res)
	}
	if !strings.Contains(res, "<pre>\nfmt.Println(\"&lt;hello&gt;\")\n</pre>") {
		t.Errorf("expected code block converted, got: %s", res)
	}
}
