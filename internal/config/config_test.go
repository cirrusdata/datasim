package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadIgnoresBinaryNamedDatasim verifies that a local binary does not get parsed as config.
func TestLoadIgnoresBinaryNamedDatasim(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "datasim")
	if err := os.WriteFile(binaryPath, []byte{0xff, 0xfe, 0xfd}, 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.ConfigFile != "" {
		t.Fatalf("expected no config file to be used, got %q", cfg.ConfigFile)
	}
}
