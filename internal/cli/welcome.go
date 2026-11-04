package cli

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ANSI color codes matching the original TypeScript implementation.
const (
	ColorReset   = "\x1b[0m"
	ColorBold    = "\x1b[1m"
	ColorDim     = "\x1b[2m"
	ColorRed     = "\x1b[31m"
	ColorGreen   = "\x1b[32m"
	ColorYellow  = "\x1b[33m"
	ColorMagenta = "\x1b[35m"
	ColorCyan    = "\x1b[36m"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9]+m`)

// VisibleLength returns the visual length of string s, ignoring ANSI escape codes.
// It counts Unicode characters (runes), not raw bytes.
func VisibleLength(s string) int {
	clean := ansiRegex.ReplaceAllString(s, "")
	return utf8.RuneCountInString(clean)
}

// FormatWelcomeBox builds the 50-character wide bordered welcome banner string for the given version.
func FormatWelcomeBox(version string) string {
	logoLines := []string{
		ColorCyan + "            ,_," + ColorReset,
		ColorCyan + "           (O,O)" + ColorReset,
		ColorCyan + "           (   )" + ColorReset,
		ColorCyan + "           -\"-\"-" + ColorReset,
	}

	width := 50
	borderTop := "╭" + strings.Repeat("─", width) + "╮"
	borderBottom := "╰" + strings.Repeat("─", width) + "╯"
	empty := "│" + strings.Repeat(" ", width) + "│"

	pad := func(s string) string {
		vLen := VisibleLength(s)
		padLen := width - vLen - 4
		if padLen < 0 {
			padLen = 0
		}
		return "│  " + s + strings.Repeat(" ", padLen) + "  │"
	}

	lines := []string{
		"",
		borderTop,
		empty,
		pad(logoLines[0]),
		pad(logoLines[1]),
		pad(logoLines[2]),
		pad(logoLines[3]),
		empty,
		pad(ColorBold + "Antigravity Telegram Remote" + ColorReset + " v" + version),
		pad(ColorDim + "Control Antigravity from your phone" + ColorReset),
		empty,
		pad(ColorYellow + "Quick start:" + ColorReset),
		pad("  " + ColorBold + "1." + ColorReset + " " + ColorGreen + "antigravity-telegram-remote setup" + ColorReset + "   " + ColorDim + "— configure Telegram bot" + ColorReset),
		pad("  " + ColorBold + "2." + ColorReset + " " + ColorGreen + "antigravity-telegram-remote open" + ColorReset + "    " + ColorDim + "— launch Antigravity + CDP" + ColorReset),
		pad("  " + ColorBold + "3." + ColorReset + " " + ColorGreen + "antigravity-telegram-remote start" + ColorReset + "   " + ColorDim + "— start the bot" + ColorReset),
		empty,
		pad(ColorDim + "Troubleshoot: " + ColorReset + ColorGreen + "antigravity-telegram-remote doctor" + ColorReset),
		pad(ColorDim + "GitHub: " + ColorReset + ColorCyan + "github.com/optimistengineer/remoat" + ColorReset),
		empty,
		borderBottom,
		"",
	}

	return strings.Join(lines, "\n")
}

// PrintWelcome prints FormatWelcomeBox(Version) to stdout or the provided writer.
func PrintWelcome(w ...io.Writer) {
	out := io.Writer(os.Stdout)
	if len(w) > 0 && w[0] != nil {
		out = w[0]
	}
	fmt.Fprintln(out, FormatWelcomeBox(Version))
}
