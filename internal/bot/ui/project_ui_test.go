package ui

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseProjectPageID(t *testing.T) {
	cases := []struct {
		input    string
		expected int
		ok       bool
	}{
		{"project_page:0", 0, true},
		{"project_page:3", 3, true},
		{"project_page:99", 99, true},
		{"other_id", 0, false},
		{"project_page", 0, false},
		{"", 0, false},
		{"project_page:abc", 0, false},
	}

	for _, c := range cases {
		page, ok := ParseProjectPageID(c.input)
		if ok != c.ok || page != c.expected {
			t.Errorf("ParseProjectPageID(%q) = (%d, %v), expected (%d, %v)",
				c.input, page, ok, c.expected, c.ok)
		}
	}
}

func TestIsProjectSelectID(t *testing.T) {
	if !IsProjectSelectID(ProjectSelectID) {
		t.Errorf("expected true for ProjectSelectID")
	}
	if !IsProjectSelectID(WorkspaceSelectID) {
		t.Errorf("expected true for WorkspaceSelectID")
	}
	if !IsProjectSelectID("project_select:my-project") {
		t.Errorf("expected true for project_select:my-project")
	}

	if IsProjectSelectID("mode_select") {
		t.Errorf("expected false for mode_select")
	}
	if IsProjectSelectID("project_page:0") {
		t.Errorf("expected false for project_page:0")
	}
	if IsProjectSelectID("") {
		t.Errorf("expected false for empty string")
	}
}

func makeTestWorkspaces(count int) []string {
	res := make([]string, count)
	for i := 0; i < count; i++ {
		res[i] = fmt.Sprintf("project-%03d", i+1)
	}
	return res
}

func TestBuildProjectListUI_ZeroWorkspaces(t *testing.T) {
	res := BuildProjectListUI([]string{}, 0)

	if !strings.Contains(res.Text, "No projects found.") {
		t.Errorf("expected 'No projects found.', got: %s", res.Text)
	}
	if res.Keyboard == nil || len(res.Keyboard.InlineKeyboard) != 0 {
		t.Errorf("expected empty keyboard, got %+v", res.Keyboard)
	}
}

func TestBuildProjectListUI_SinglePage(t *testing.T) {
	workspaces := makeTestWorkspaces(5)
	res := BuildProjectListUI(workspaces, 0)

	if !strings.Contains(res.Text, "Projects") {
		t.Errorf("expected 'Projects', got: %s", res.Text)
	}
	for _, ws := range workspaces {
		if !strings.Contains(res.Text, ws) {
			t.Errorf("expected %s in text, got: %s", ws, res.Text)
		}
	}
	if strings.Contains(res.Text, "Page") {
		t.Errorf("did not expect 'Page' for single page, got: %s", res.Text)
	}
	if len(res.Keyboard.InlineKeyboard) != 5 {
		t.Errorf("expected 5 keyboard rows, got %d", len(res.Keyboard.InlineKeyboard))
	}
}

func TestBuildProjectListUI_MultiPage(t *testing.T) {
	count := ProjectItemsPerPage + 5
	workspaces := makeTestWorkspaces(count)

	// Page 0
	res0 := BuildProjectListUI(workspaces, 0)
	if !strings.Contains(res0.Text, "Page 1 / 2") {
		t.Errorf("expected 'Page 1 / 2', got: %s", res0.Text)
	}
	if !strings.Contains(res0.Text, fmt.Sprintf("%d total projects", count)) {
		t.Errorf("expected total projects count, got: %s", res0.Text)
	}

	// Page 1
	res1 := BuildProjectListUI(workspaces, 1)
	if !strings.Contains(res1.Text, "Page 2 / 2") {
		t.Errorf("expected 'Page 2 / 2', got: %s", res1.Text)
	}
	if !strings.Contains(res1.Text, workspaces[ProjectItemsPerPage]) {
		t.Errorf("expected item %s in page 2, got: %s", workspaces[ProjectItemsPerPage], res1.Text)
	}
}

func TestBuildProjectListUI_Clamping(t *testing.T) {
	count := ProjectItemsPerPage + 5
	workspaces := makeTestWorkspaces(count)

	// Clamp page 100 to last page (Page 2 / 2)
	resOverflow := BuildProjectListUI(workspaces, 100)
	if !strings.Contains(resOverflow.Text, "Page 2 / 2") {
		t.Errorf("expected 'Page 2 / 2', got: %s", resOverflow.Text)
	}

	// Clamp negative page -5 to page 0 (Page 1 / 2)
	resNegative := BuildProjectListUI(workspaces, -5)
	if !strings.Contains(resNegative.Text, "Page 1 / 2") {
		t.Errorf("expected 'Page 1 / 2', got: %s", resNegative.Text)
	}
}

func TestBuildProjectListUI_ExactLimit(t *testing.T) {
	workspaces := makeTestWorkspaces(ProjectItemsPerPage)
	res := BuildProjectListUI(workspaces, 0)

	if strings.Contains(res.Text, "Page") {
		t.Errorf("did not expect 'Page' for exactly 100 items, got: %s", res.Text)
	}
	if len(res.Keyboard.InlineKeyboard) != ProjectItemsPerPage {
		t.Errorf("expected %d rows, got %d", ProjectItemsPerPage, len(res.Keyboard.InlineKeyboard))
	}
}
