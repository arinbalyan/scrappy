package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppConfigParsesDefaultsAndSites(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	data := []byte("defaults:\n  search: backend\n  location: Remote\n  results_wanted: 7\nsites:\n  remoteok:\n    search: golang\n    location: Remote\n")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := loadAppConfig(p)
	if cfg.Defaults.Search != "backend" || cfg.Defaults.Location != "Remote" || cfg.Defaults.ResultsWanted != 7 {
		t.Fatalf("unexpected defaults: %+v", cfg.Defaults)
	}
	remoteok, ok := cfg.Sites["remoteok"]
	if !ok {
		t.Fatalf("missing remoteok site config")
	}
	if remoteok.Search != "golang" || remoteok.Location != "Remote" {
		t.Fatalf("unexpected remoteok target: %+v", remoteok)
	}
}

func TestLoadAppConfigMissingFile(t *testing.T) {
	cfg := loadAppConfig("/path/does/not/exist.yaml")
	if cfg.Sites != nil && len(cfg.Sites) != 0 {
		t.Fatalf("expected empty sites on missing file")
	}
}
