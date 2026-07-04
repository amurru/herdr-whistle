package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := "token = \"bot123\"\nowner_id = 42\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Token != "bot123" {
		t.Errorf("Token = %q, want \"bot123\"", cfg.Token)
	}
	if cfg.OwnerID != 42 {
		t.Errorf("OwnerID = %d, want 42", cfg.OwnerID)
	}

	if _, err := loadConfig(filepath.Join(dir, "does-not-exist.toml")); err == nil {
		t.Error("expected error for missing config file")
	}
}
