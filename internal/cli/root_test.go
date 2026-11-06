package cli

import (
	"testing"

	"antigravity-telegram-remote/internal/logger"
)

func TestResolveCliLogLevel(t *testing.T) {
	// 1. Verbose flag takes highest priority -> LevelDebug
	lvl := ResolveCliLogLevel(RootOptions{Verbose: true, Quiet: false})
	if lvl == nil || *lvl != logger.LevelDebug {
		t.Errorf("expected LevelDebug, got %v", lvl)
	}

	// 2. Both set -> Verbose wins -> LevelDebug
	lvl = ResolveCliLogLevel(RootOptions{Verbose: true, Quiet: true})
	if lvl == nil || *lvl != logger.LevelDebug {
		t.Errorf("expected LevelDebug when both set, got %v", lvl)
	}

	// 3. Quiet flag -> LevelError
	lvl = ResolveCliLogLevel(RootOptions{Verbose: false, Quiet: true})
	if lvl == nil || *lvl != logger.LevelError {
		t.Errorf("expected LevelError, got %v", lvl)
	}

	// 4. Neither set -> nil
	lvl = ResolveCliLogLevel(RootOptions{Verbose: false, Quiet: false})
	if lvl != nil {
		t.Errorf("expected nil when neither flag set, got %v", lvl)
	}
}

func TestNewRootCmd_Structure(t *testing.T) {
	cmd := NewRootCmd()

	if cmd.Use != "antigravity-telegram-remote" {
		t.Errorf("root command Use = %q; want 'antigravity-telegram-remote'", cmd.Use)
	}
	if cmd.Version != Version {
		t.Errorf("root command Version = %q; want %q", cmd.Version, Version)
	}

	// Persistent flags
	if cmd.PersistentFlags().Lookup("verbose") == nil {
		t.Error("expected --verbose persistent flag")
	}
	if cmd.PersistentFlags().Lookup("quiet") == nil {
		t.Error("expected --quiet persistent flag")
	}

	// Subcommands
	expectedSubcmds := map[string]string{
		"start":  "Start the Telegram bot",
		"setup":  "Interactive setup wizard",
		"open":   "Open Antigravity with CDP enabled (automatically selects an available port)",
		"doctor": "Check environment and dependencies",
	}

	subcmds := cmd.Commands()
	found := make(map[string]bool)
	for _, sc := range subcmds {
		found[sc.Name()] = true
		if desc, ok := expectedSubcmds[sc.Name()]; ok {
			if sc.Short != desc {
				t.Errorf("subcommand %q Short = %q; want %q", sc.Name(), sc.Short, desc)
			}
		}
	}

	for name := range expectedSubcmds {
		if !found[name] {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}

	// Verify doctor --selectors flag
	doctorCmd, _, err := cmd.Find([]string{"doctor"})
	if err != nil || doctorCmd == nil {
		t.Fatal("expected doctor subcommand to be found")
	}
	if doctorCmd.Flags().Lookup("selectors") == nil {
		t.Error("expected --selectors flag on doctor command")
	}
}
