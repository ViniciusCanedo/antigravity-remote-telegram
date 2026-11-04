package formatter

import (
	"testing"
)

func TestExtractMetadataFromFooter(t *testing.T) {
	t.Run("correctly extracts TaskID and target directory from a Footer", func(t *testing.T) {
		footerText := "TaskID: abc-123 | Dir: /workspace/projectA"
		result := ExtractMetadataFromFooter(footerText)
		if result.TaskID != "abc-123" {
			t.Errorf("expected TaskID 'abc-123', got %q", result.TaskID)
		}
		if result.Directory != "/workspace/projectA" {
			t.Errorf("expected Directory '/workspace/projectA', got %q", result.Directory)
		}
	})

	t.Run("extracts correctly when only TaskID is present", func(t *testing.T) {
		footerText := "TaskID: xyz-999"
		result := ExtractMetadataFromFooter(footerText)
		if result.TaskID != "xyz-999" {
			t.Errorf("expected TaskID 'xyz-999', got %q", result.TaskID)
		}
		if result.Directory != "" {
			t.Errorf("expected empty Directory, got %q", result.Directory)
		}
	})

	t.Run("extracts correctly when only the target directory is present", func(t *testing.T) {
		footerText := "Dir: /tmp/testDir"
		result := ExtractMetadataFromFooter(footerText)
		if result.TaskID != "" {
			t.Errorf("expected empty TaskID, got %q", result.TaskID)
		}
		if result.Directory != "/tmp/testDir" {
			t.Errorf("expected Directory '/tmp/testDir', got %q", result.Directory)
		}
	})

	t.Run("returns an empty object when no metadata is present", func(t *testing.T) {
		footerText := "antigravity-telegram-remote System"
		result := ExtractMetadataFromFooter(footerText)
		if result.TaskID != "" || result.Directory != "" {
			t.Errorf("expected empty TaskMetadata, got %+v", result)
		}
	})

	t.Run("is resilient to whitespace variations and format differences", func(t *testing.T) {
		footerText := "  TaskID:   foo-bar   |   Dir:   /project/a/b/c  "
		result := ExtractMetadataFromFooter(footerText)
		if result.TaskID != "foo-bar" {
			t.Errorf("expected TaskID 'foo-bar', got %q", result.TaskID)
		}
		if result.Directory != "/project/a/b/c" {
			t.Errorf("expected Directory '/project/a/b/c', got %q", result.Directory)
		}
	})
}
