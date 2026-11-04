package bot

import (
	"regexp"
	"strings"
)

// ParsedMessage represents the result of parsing a text message or voice transcription.
type ParsedMessage struct {
	IsCommand   bool
	CommandName string
	Args        []string
	Text        string
}

var commandTokenRegex = regexp.MustCompile(`(?:[^\s"]+|"[^"]*")+`)

// ParseMessageContent inspects content to determine whether it is a slash command and extracts
// the command name and tokenized arguments (stripping surrounding double quotes from arguments).
func ParseMessageContent(content string) ParsedMessage {
	trimmed := strings.TrimSpace(content)

	if !strings.HasPrefix(trimmed, "/") || trimmed == "/" {
		return ParsedMessage{
			IsCommand: false,
			Text:      content,
		}
	}

	parts := commandTokenRegex.FindAllString(trimmed[1:], -1)
	if len(parts) == 0 {
		return ParsedMessage{
			IsCommand: false,
			Text:      content,
		}
	}

	commandName := parts[0]
	rawArgs := parts[1:]
	args := make([]string, 0, len(rawArgs))

	for _, arg := range rawArgs {
		if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") && len(arg) >= 2 {
			arg = arg[1 : len(arg)-1]
		}
		args = append(args, arg)
	}

	return ParsedMessage{
		IsCommand:   true,
		CommandName: commandName,
		Args:        args,
	}
}
