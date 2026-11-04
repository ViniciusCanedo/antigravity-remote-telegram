package formatter

import "regexp"

var (
	reTaskID = regexp.MustCompile(`(?i)TaskID:\s*([^\s|]+)`)
	reDir    = regexp.MustCompile(`(?i)Dir:\s*([^\s|]+)`)
)

// TaskMetadata represents metadata extracted from a message footer.
// Empty fields ("") correspond to undefined in TypeScript.
type TaskMetadata struct {
	TaskID    string `json:"taskId,omitempty"`
	Directory string `json:"directory,omitempty"`
}

// ExtractMetadataFromFooter extracts TaskID and Dir from a footer text.
func ExtractMetadataFromFooter(footerText string) TaskMetadata {
	var result TaskMetadata

	if taskMatch := reTaskID.FindStringSubmatch(footerText); len(taskMatch) > 1 {
		result.TaskID = taskMatch[1]
	}

	if dirMatch := reDir.FindStringSubmatch(footerText); len(dirMatch) > 1 {
		result.Directory = dirMatch[1]
	}

	return result
}
