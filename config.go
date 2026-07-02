package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config holds the plugin configuration loaded from a TOML file.
type Config struct {
	Token   string `toml:"token"`
	OwnerID int64  `toml:"owner_id"`
}

// loadConfig reads and parses a TOML configuration file at the given path.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	return &cfg, nil
}
