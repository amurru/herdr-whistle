package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	// Determine config path from environment or use default.
	configPath := "./config.toml"
	if envDir := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); envDir != "" {
		configPath = filepath.Join(envDir, "config.toml")
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("ERROR loading config from %s: %v", configPath, err)
	}

	if cfg.Token == "" {
		log.Fatalf("ERROR: bot token is empty in configuration")
	}
	if cfg.OwnerID == 0 {
		log.Fatalf("ERROR: owner_id is not set in configuration")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "Starting herdr-whistle bot...\n")
	// startBot blocks on b.Start(ctx), which runs until ctx is cancelled by
	// SIGINT/SIGTERM. If it ever returns early we exit rather than hang.
	startBot(ctx, cfg)
	fmt.Fprintf(os.Stderr, "Shutting down...\n")
}
