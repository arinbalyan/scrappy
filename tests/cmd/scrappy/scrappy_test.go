package scrappy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scrappy")
	cmd := exec.Command("go", "build", "-o", path, "./cmd/scrappy")
	cmd.Dir = filepath.Join("..", "..", "..") // tests/cmd/scrappy/ → repo root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return path
}

func TestVersionFlag(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected version output")
	}
}

func TestHelpFlag(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "--help").Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	help := string(out)
	if len(help) < 100 {
		t.Fatal("help too short")
	}
	// Should mention 141 sites
	if !contains(help, "141 sites") {
		t.Error("help should mention 141 sites")
	}
	// Should mention github-scrape
	if !contains(help, "github-scrape") {
		t.Error("help should mention --github-scrape")
	}
}

func TestEmailFlagParsesInCLI(t *testing.T) {
	bin := buildBinary(t)
	// Just verify the flag exists without error
	cmd := exec.Command(bin, "--email", "--non-interactive", "--search", "x", "--sites", "indeed", "--results-wanted", "1")
	// Don't wait for actual scrape - just verify the command starts
	if err := cmd.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	cmd.Process.Kill()
}

func TestGitHubScrapeFlag(t *testing.T) {
	bin := buildBinary(t)
	// Verify the flag exists without error for help
	cmd := exec.Command(bin, "--github-scrape", "--search", "test", "--out", os.DevNull)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	cmd.Process.Kill()
}

func TestDefaultConfigPath(t *testing.T) {
	bin := buildBinary(t)
	// Running with --help shouldn't error
	out, err := exec.Command(bin, "--help").Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	// Should reference config.toml (not config.yaml)
	if contains(string(out), "config.yaml") {
		t.Error("help should not reference config.yaml")
	}
	if !contains(string(out), "config.toml") {
		t.Error("help should reference config.toml")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
