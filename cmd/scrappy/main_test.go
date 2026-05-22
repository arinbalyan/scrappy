package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadAppConfigParsesDefaultsAndSites(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	data := []byte("defaults:\n  search: backend\n  location: Remote\n  results_wanted: 7\n  out: /tmp/jobs.csv\n  format: csv\nsites:\n  remoteok:\n    search: golang\n    location: Remote\n")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := loadAppConfig(p)
	if cfg.Defaults.Search != "backend" || cfg.Defaults.Location != "Remote" || cfg.Defaults.ResultsWanted != 7 || cfg.Defaults.Out != "/tmp/jobs.csv" || cfg.Defaults.Format != "csv" {
		t.Fatalf("unexpected defaults: %+v", cfg.Defaults)
	}
	remoteok, ok := cfg.Sites["remoteok"]
	if !ok {
		t.Fatalf("missing remoteok site config")
	}
	if len(remoteok.Search) != 1 || remoteok.Search[0] != "golang" || remoteok.Location != "Remote" {
		t.Fatalf("unexpected remoteok target: %+v", remoteok)
	}
}

func TestLoadAppConfigParsesSiteSearchList(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	data := []byte("sites:\n  remoteok:\n    search:\n      - golang\n      - backend\n    location: Remote\n")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := loadAppConfig(p)
	remoteok, ok := cfg.Sites["remoteok"]
	if !ok {
		t.Fatalf("missing remoteok site config")
	}
	if len(remoteok.Search) != 2 || remoteok.Search[0] != "golang" || remoteok.Search[1] != "backend" {
		t.Fatalf("unexpected remoteok search terms: %#v", remoteok.Search)
	}
}

func TestRootCommandParsesEmailFlag(t *testing.T) {
	cfg := &cliConfig{}
	root := newRootCommand(cfg)
	root.SetArgs([]string{"--email", "--non-interactive", "--search", "x", "--sites", "indeed"})
	root.RunE = func(cmd *cobra.Command, args []string) error { return nil }
	if err := root.Execute(); err != nil {
		t.Fatalf("execute root: %v", err)
	}
	if !cfg.EmailOnly {
		t.Fatalf("expected --email to set EmailOnly")
	}
}

func TestLoadAppConfigMissingFile(t *testing.T) {
	cfg := loadAppConfig("/path/does/not/exist.yaml")
	if cfg.Sites != nil && len(cfg.Sites) != 0 {
		t.Fatalf("expected empty sites on missing file")
	}
}

func TestRootCommandFormatDefaultIsEmpty(t *testing.T) {
	cfg := &cliConfig{}
	root := newRootCommand(cfg)
	root.SetArgs([]string{"--non-interactive", "--search", "x", "--sites", "indeed"})
	root.RunE = func(cmd *cobra.Command, args []string) error { return nil }
	if err := root.Execute(); err != nil {
		t.Fatalf("execute root: %v", err)
	}
	if cfg.Format != "" {
		t.Fatalf("expected default format to be empty, got %q", cfg.Format)
	}
}
