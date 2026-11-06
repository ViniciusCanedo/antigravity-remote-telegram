package cli

import (
	"os"
	"path/filepath"

	"antigravity-telegram-remote/internal/config"
	"antigravity-telegram-remote/internal/logger"
	"github.com/spf13/cobra"
)

// Version stores the semantic package version (default "0.2.12", injectable via ldflags).
var Version = "0.2.12"

// RootOptions represents global CLI flags (--verbose and --quiet).
type RootOptions struct {
	Verbose bool
	Quiet   bool
}

// ResolveCliLogLevel resolves log level precedence: --verbose ("debug") > --quiet ("error") > nil.
func ResolveCliLogLevel(opts RootOptions) *logger.LogLevel {
	if opts.Verbose {
		lvl := logger.LevelDebug
		return &lvl
	}
	if opts.Quiet {
		lvl := logger.LevelError
		return &lvl
	}
	return nil
}

// NewRootCmd constructs the root "antigravity-telegram-remote" command and registers subcommands.
func NewRootCmd() *cobra.Command {
	var rootOpts RootOptions

	rootCmd := &cobra.Command{
		Use:     "antigravity-telegram-remote",
		Short:   "Control your AI coding assistant from Telegram",
		Version: Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				cwd = "."
			}
			hasConfig := config.ConfigExists()
			_, envErr := os.Stat(filepath.Join(cwd, ".env"))
			hasEnv := envErr == nil

			if !hasConfig && !hasEnv {
				PrintWelcome()
				return SetupAction(cmd.Context())
			}
			return StartAction(cmd.Context(), rootOpts, nil)
		},
	}

	rootCmd.PersistentFlags().BoolVar(&rootOpts.Verbose, "verbose", false, "Show debug-level logs")
	rootCmd.PersistentFlags().BoolVar(&rootOpts.Quiet, "quiet", false, "Show errors only")

	// Subcommand: start
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Telegram bot",
		RunE: func(cmd *cobra.Command, args []string) error {
			return StartAction(cmd.Context(), rootOpts, nil)
		},
	}

	// Subcommand: setup
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return SetupAction(cmd.Context())
		},
	}

	// Subcommand: open
	openCmd := &cobra.Command{
		Use:   "open",
		Short: "Open Antigravity with CDP enabled (automatically selects an available port)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return OpenAction(cmd.Context())
		},
	}

	// Subcommand: doctor
	var doctorOpts DoctorOptions
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check environment and dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return DoctorAction(cmd.Context(), doctorOpts)
		},
	}
	doctorCmd.Flags().BoolVar(&doctorOpts.Selectors, "selectors", false, "Check DOM selectors status (ADR-003)")

	rootCmd.AddCommand(startCmd, setupCmd, openCmd, doctorCmd)

	return rootCmd
}

// Execute runs the root command.
func Execute() error {
	return NewRootCmd().Execute()
}
