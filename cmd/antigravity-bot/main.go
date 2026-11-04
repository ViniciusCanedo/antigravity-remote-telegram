package main

import (
	"os"

	"antigravity-telegram-remote/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
