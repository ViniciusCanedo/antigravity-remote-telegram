package cli

import (
	"context"
	"fmt"

	"antigravity-telegram-remote/internal/bot"
	"antigravity-telegram-remote/internal/logger"
	"antigravity-telegram-remote/internal/platform"
)

// StartDeps allows injecting mocks in unit tests for startup flow.
type StartDeps struct {
	AcquireLock     func() error
	CheckForUpdates func(ctx context.Context, currentVersion string)
	StartBot        func(ctx context.Context, cliLogLevel *logger.LogLevel) error
}

// StartAction executes log level resolution, prints LOGO, acquires .bot.lock, fires CheckForUpdates in goroutine and starts StartBot.
func StartAction(ctx context.Context, opts RootOptions, deps *StartDeps) error {
	cliLogLevel := ResolveCliLogLevel(opts)
	if cliLogLevel != nil {
		logger.Default.SetLogLevel(*cliLogLevel)
	}

	fmt.Print(Logo)

	if deps != nil && deps.AcquireLock != nil {
		if err := deps.AcquireLock(); err != nil {
			return err
		}
	} else {
		release := platform.AcquireLock()
		defer release()
	}

	if deps != nil && deps.CheckForUpdates != nil {
		deps.CheckForUpdates(ctx, Version)
	} else {
		go CheckForUpdates(ctx, Version)
	}

	if deps != nil && deps.StartBot != nil {
		if err := deps.StartBot(ctx, cliLogLevel); err != nil {
			logger.Default.Error("Failed to start bot:", err)
			return err
		}
		return nil
	}

	if err := bot.StartBot(ctx, cliLogLevel); err != nil {
		logger.Default.Error("Failed to start bot:", err)
		return err
	}

	return nil
}
