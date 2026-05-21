package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppConfigParsesDefaultsAndSites(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	data := []byte("defaults:\n  search: backend\n  location: Remote\n  results_wanted: 7\nsites:\n  seek:\n    search: golang\n    location: Australia\n")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := loadAppConfig(p)
	if cfg.Defaults.Search != "backend" || cfg.Defaults.Location != "Remote" || cfg.Defaults.ResultsWanted != 7 {
		t.Fatalf("unexpected defaults: %+v", cfg.Defaults)
	}
	seek, ok := cfg.Sites["seek"]
	if !ok {
		t.Fatalf("missing seek site config")
	}
	if seek.Search != "golang" || seek.Location != "Australia" {
		t.Fatalf("unexpected seek target: %+v", seek)
	}
}

func TestLoadAppConfigMissingFile(t *testing.T) {
	cfg := loadAppConfig("/path/does/not/exist.yaml")
	if cfg.Sites != nil && len(cfg.Sites) != 0 {
		t.Fatalf("expected empty sites on missing file")
	}
}
