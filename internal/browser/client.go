// Package browser provides a browser-based fallback for scraping sites
// that block plain HTTP requests with anti-bot challenges (DataDome,
// Cloudflare, reCAPTCHA, etc.).
//
// It calls the Playwright script scripts/fetch-page.mjs via os/exec to
// render pages in headless Chromium. Browser automation is optional — if
// the script is not found or execution fails, FetchPage returns a clear
// error so callers can fall through gracefully.
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PageResult holds the browser-fetched page content.
type PageResult struct {
	HTML    string   `json:"html"`
	Cookies []Cookie `json:"cookies"`
	Status  int      `json:"status"`
}

// Cookie represents a browser cookie.
type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// detectScriptPath finds the Playwright fetch script relative to the
// Go binary or the working directory.
func detectScriptPath() string {
	// Try next to the running binary first.
	execPath, err := exec.LookPath("fetch-page.mjs")
	if err == nil {
		return execPath
	}

	// Check from CWD for project root layouts.
	for _, root := range []string{".", "..", "../..", "../../.."} {
		p := filepath.Join(root, "scripts", "fetch-page.mjs")
		if fileExists(p) {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// FetchPage renders the given URL in headless Chromium via Playwright
// and returns the rendered HTML, cookies, and HTTP status.
//
// If the Playwright script is not found, it returns a descriptive error.
// ctx controls the overall timeout.
func FetchPage(ctx context.Context, targetURL string, waitSelector string) (*PageResult, error) {
	scriptPath := detectScriptPath()
	if scriptPath == "" {
		return nil, fmt.Errorf("browser: fetch-page.mjs not found — install Playwright and run npm install in scripts/")
	}
	if strings.TrimSpace(targetURL) == "" {
		return nil, fmt.Errorf("browser: empty target URL")
	}
	u, err := url.Parse(targetURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("browser: invalid target URL: %q", targetURL)
	}

	args := []string{scriptPath, targetURL}
	if waitSelector != "" {
		args = append(args, "--wait", waitSelector)
	}

	// Create a command context with default 45s timeout.
	timeout := 45 * time.Second
	if d, ok := ctx.Deadline(); ok {
		timeout = time.Until(d)
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
	}

	// One retry for transient browser boot/timeout errors.
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(execCtx, "node", args...)
		// Set working directory to scripts dir so playwright can find its modules.
		scriptsDir := filepath.Dir(scriptPath)
		cmd.Dir = scriptsDir
		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			msg := string(output)
			if len(msg) > 1200 {
				msg = msg[:1200] + "...(truncated)"
			}
			lastErr = fmt.Errorf("browser fetch attempt %d: %w (output: %s)", attempt, err, msg)
			if attempt == 1 {
				continue
			}
			return nil, lastErr
		}
		if len(output) == 0 {
			lastErr = fmt.Errorf("browser fetch attempt %d: empty output", attempt)
			if attempt == 1 {
				continue
			}
			return nil, lastErr
		}
		var result PageResult
		if err := json.Unmarshal(output, &result); err != nil {
			raw := string(output)
			if len(raw) > 1200 {
				raw = raw[:1200] + "...(truncated)"
			}
			lastErr = fmt.Errorf("browser fetch attempt %d: decode: %w (raw: %s)", attempt, err, raw)
			if attempt == 1 {
				continue
			}
			return nil, lastErr
		}
		if result.Status == 0 {
			// Normalized fallback; script may omit status in edge cases.
			result.Status = 200
		}
		return &result, nil
	}
	return nil, lastErr
}

// CookieMap returns the fetched cookies as a map[string]string for
// easy injection into HTTP headers.
func (r *PageResult) CookieMap() map[string]string {
	m := make(map[string]string, len(r.Cookies))
	for _, c := range r.Cookies {
		m[c.Name] = c.Value
	}
	return m
}

// IsAvailable returns true if the Playwright browser script is
// installed and ready to use.
func IsAvailable() bool {
	_, err := exec.LookPath("node")
	if err != nil {
		return false
	}
	return detectScriptPath() != ""
}

// CheckDependencies checks if the browser fetch setup is complete and
// returns a human-readable status.
func CheckDependencies() error {
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("node not found in PATH")
	}
	if detectScriptPath() == "" {
		return fmt.Errorf("scripts/fetch-page.mjs not found")
	}
	// Try to resolve playwright module.
	scriptsDir := filepath.Dir(detectScriptPath())
	cmd := exec.Command("node", "-e", "require('playwright')")
	cmd.Dir = scriptsDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("playwright module not found in scripts/: %s", string(out))
	}
	return nil
}
