// Package foundation anchors foundational external dependencies for the Antigravity Telegram Remote port.
package foundation

import (
	_ "github.com/coder/websocket"
	_ "github.com/go-telegram/bot"
	_ "github.com/joho/godotenv"
	_ "github.com/robfig/cron/v3"
	_ "github.com/spf13/cobra"
	_ "golang.org/x/term"
	_ "modernc.org/sqlite"
)

// PinnedDependencies enumerates the required external dependencies defined in Spec 00.
var PinnedDependencies = []string{
	"github.com/coder/websocket",
	"github.com/go-telegram/bot",
	"github.com/joho/godotenv",
	"github.com/robfig/cron/v3",
	"github.com/spf13/cobra",
	"golang.org/x/term",
	"modernc.org/sqlite",
}
