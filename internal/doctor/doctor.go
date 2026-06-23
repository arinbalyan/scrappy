package doctor

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

// Severity levels for diagnostic results.
type Severity int

const (
	SeverityPass  Severity = 0
	SeverityWarn  Severity = 1
	SeverityFail  Severity = 2
	SeverityFix   Severity = 3
	SeverityInfo  Severity = 4
)

func (s Severity) String() string {
	switch s {
	case SeverityPass:
		return "\033[32m✓ PASS\033[0m"
	case SeverityWarn:
		return "\033[33m⚠ WARN\033[0m"
	case SeverityFail:
		return "\033[31m✗ FAIL\033[0m"
	case SeverityFix:
		return "\033[36m⚡ FIX\033[0m"
	case SeverityInfo:
		return "\033[34mℹ INFO\033[0m"
	default:
		return "?"
	}
}

type CheckResult struct {
	Title    string
	Severity Severity
	Message  string
	Details  string
	Fix      func() error
}

type Report struct {
	Results []CheckResult
	Passed  int
	Failed  int
	Fixed   int
}

func (r *Report) Add(c CheckResult) {
	r.Results = append(r.Results, c)
	switch c.Severity {
	case SeverityPass:
		r.Passed++
	case SeverityFail:
		r.Failed++
	}
}

func (r *Report) Print() {
	fmt.Println()
	fmt.Println("  \033[38;5;117m╭─ scrappy doctor ───────────────────────────╮\033[0m")
	fmt.Println("  \033[38;5;117m│\033[0m  System health check for scrappy setup       \033[38;5;117m│\033[0m")
	fmt.Println("  \033[38;5;117m╰──────────────────────────────────────────────╯\033[0m")
	fmt.Println()

	for _, c := range r.Results {
		icon := c.Severity.String()
		if c.Severity == SeverityFix {
			fmt.Printf("  %s %s\n", icon, c.Title)
		} else {
			fmt.Printf("  %s  %s\n", icon, c.Title)
		}
		if c.Message != "" {
			fmt.Printf("         \033[90m%s\033[0m\n", c.Message)
		}
		if c.Details != "" {
			fmt.Printf("         \033[90m%s\033[0m\n", c.Details)
		}
	}

	fmt.Println()
	fmt.Printf("  \033[90m─── \033[0m%d passed, %d failed, %d fixed\n\n", r.Passed, r.Failed, r.Fixed)
}

func (r *Report) PrintBrief() {
	fmt.Println()
	status := "\033[32m✓ All checks passed\033[0m"
	if r.Failed > 0 {
		status = fmt.Sprintf("\033[31m✗ %d check(s) failed\033[0m", r.Failed)
	}
	fmt.Printf("  scrappy doctor: %d passed, %d failed, %d fixed  |  %s\n", r.Passed, r.Failed, r.Fixed, status)
	fmt.Println()
}

type DoctorConfig struct {
	ConfigPath string
	FixMode    bool
	Verbose    bool
}

func Run(ctx context.Context, cfg DoctorConfig) *Report {
	report := &Report{}

	checkGoVersion(report)
	checkBinaryVersion(report)
	checkConfigFile(report, cfg.ConfigPath)
	checkConfigTOML(report, cfg.ConfigPath, cfg.FixMode)
	checkEnvVars(report)
	checkHomeDir(report)
	checkNetwork(report)
	checkPlaywright(report)
	checkDataDir(report, cfg.FixMode)

	return report
}

func checkGoVersion(r *Report) {
	run := func(name string, args ...string) string {
		cmd := exec.Command(name, args...)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}

	v := run("go", "version")
	if v == "" {
		r.Add(CheckResult{
			Title:    "Go runtime",
			Severity: SeverityInfo,
			Message:  "Go not found (only needed for development)",
		})
		return
	}
	r.Add(CheckResult{
		Title:    "Go runtime",
		Severity: SeverityPass,
		Message:  v,
	})
}

func checkBinaryVersion(r *Report) {
	// Detect if running as installed binary vs go run
	exe, err := os.Executable()
	if err != nil {
		return
	}
	info, err := os.Stat(exe)
	if err != nil {
		return
	}
	r.Add(CheckResult{
		Title:    "Binary",
		Severity: SeverityPass,
		Message:  fmt.Sprintf("%s (%d MB, built %s)", filepath.Base(exe), info.Size()/1024/1024, info.ModTime().Format("Jan 02")),
	})
}

func checkConfigFile(r *Report, configPath string) {
	paths := []string{
		configPath,
		"config.toml",
		filepath.Join(os.Getenv("HOME"), ".scrappy", "config.toml"),
	}

	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if _, err := os.Stat(p); err == nil {
			r.Add(CheckResult{
				Title:    "Config file found",
				Severity: SeverityPass,
				Message:  p,
			})

			// Check permissions — should not be world-readable (might have proxy creds)
			info, _ := os.Stat(p)
			if info != nil && info.Mode().Perm()&0044 != 0 {
				r.Add(CheckResult{
					Title:    "Config permissions",
					Severity: SeverityWarn,
					Message:  fmt.Sprintf("%s is world-readable (%o)", p, info.Mode().Perm()),
					Details:  "Run: chmod 600 " + p,
					Fix: func() error {
						return os.Chmod(p, 0600)
					},
				})
			}
			return
		}
	}
	r.Add(CheckResult{
		Title:    "Config file",
		Severity: SeverityInfo,
		Message:  "No config.toml found — using defaults",
	})
}

func checkConfigTOML(r *Report, configPath string, fixMode bool) {
	paths := []string{configPath, "config.toml", filepath.Join(os.Getenv("HOME"), ".scrappy", "config.toml")}
	seen := map[string]bool{}

	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		// Try to parse as TOML
		var raw interface{}
		if err := toml.Unmarshal(b, &raw); err != nil {
			r.Add(CheckResult{
				Title:    "Config TOML syntax",
				Severity: SeverityFail,
				Message:  fmt.Sprintf("%s: %v", p, err),
			})
			continue
		}

		// Try to parse into appConfig
		var ac struct {
			Defaults struct {
				Search        interface{} `toml:"search"`
				ResultsWanted int        `toml:"results_wanted"`
				Format        string     `toml:"format"`
				MemoryCap     string     `toml:"memory_cap"`
			} `toml:"defaults"`
			Proxy string                 `toml:"proxy"`
			Sites map[string]interface{} `toml:"sites"`
		}
		if err := toml.Unmarshal(b, &ac); err != nil {
			r.Add(CheckResult{
				Title:    "Config structure",
				Severity: SeverityFail,
				Message:  fmt.Sprintf("%s: %v", p, err),
			})
			continue
		}

		// Validate known fields
		r.Add(CheckResult{
			Title:    "Config TOML",
			Severity: SeverityPass,
			Message:  fmt.Sprintf("%s parsed successfully", p),
		})

		// Check for unknown/misspelled site names
		if ac.Sites != nil {
			validSites := model.AllSites()
			validMap := make(map[string]bool, len(validSites))
			for _, s := range validSites {
				validMap[strings.ToLower(string(s))] = true
			}
			for siteName := range ac.Sites {
				if !validMap[strings.ToLower(siteName)] {
					r.Add(CheckResult{
						Title:    fmt.Sprintf("Unknown site: %s", siteName),
						Severity: SeverityWarn,
						Message:  fmt.Sprintf("'%s' is not a recognized site — check spelling", siteName),
					})
				}
			}
		}
		return
	}
}

func checkEnvVars(r *Report) {
	type siteEnv struct {
		Name     string
		Vars     []string
		SetupURL string
	}

	sites := []siteEnv{
		{Name: "Adzuna", Vars: []string{"ADZUNA_APP_ID", "ADZUNA_APP_KEY"}, SetupURL: "https://developer.adzuna.com/"},
		{Name: "Careerjet", Vars: []string{"CAREERJET_AFFID"}, SetupURL: "https://www.careerjet.com/partners/"},
		{Name: "Infojobs", Vars: []string{"INFOJOBS_CLIENT_ID", "INFOJOBS_CLIENT_SECRET"}, SetupURL: "https://developer.infojobs.net/"},
		{Name: "Findwork", Vars: []string{"FINDWORK_API_KEY"}, SetupURL: "https://findwork.dev/developers/"},
		{Name: "Arbeitsagentur", Vars: []string{"ARBEITSAGENTUR_API_KEY"}, SetupURL: "https://rest.arbeitsagentur.de/"},
	}

	missingAny := false
	for _, s := range sites {
		missing := make([]string, 0)
		for _, v := range s.Vars {
			if os.Getenv(v) == "" {
				missing = append(missing, v)
			}
		}
		if len(missing) > 0 {
			r.Add(CheckResult{
				Title:    fmt.Sprintf("API key: %s", s.Name),
				Severity: SeverityWarn,
				Message:  fmt.Sprintf("Missing: %s", strings.Join(missing, ", ")),
				Details:  fmt.Sprintf("Get at %s", s.SetupURL),
			})
			missingAny = true
		} else {
			r.Add(CheckResult{
				Title:    fmt.Sprintf("API key: %s", s.Name),
				Severity: SeverityPass,
				Message:  "All env vars set",
			})
		}
	}

	if !missingAny {
		// Also check for proxy env
		if v := os.Getenv("SCRAPPY_PROXIES"); v != "" {
			r.Add(CheckResult{
				Title:    "Proxy env",
				Severity: SeverityPass,
				Message:  "SCRAPPY_PROXIES is set",
			})
		}
	}
}

func checkHomeDir(r *Report) {
	home, err := os.UserHomeDir()
	if err != nil {
		r.Add(CheckResult{
			Title:    "Home directory",
			Severity: SeverityWarn,
			Message:  fmt.Sprintf("Cannot determine home dir: %v", err),
		})
		return
	}

	scrappyDir := filepath.Join(home, ".scrappy")
	info, err := os.Stat(scrappyDir)
	if err != nil {
		r.Add(CheckResult{
			Title:    "User config dir",
			Severity: SeverityInfo,
			Message:  fmt.Sprintf("~/.scrappy/ does not exist yet — created on first interactive run"),
		})
		return
	}

	r.Add(CheckResult{
		Title:    "User config dir",
		Severity: SeverityPass,
		Message:  fmt.Sprintf("~/.scrappy/ exists (%s)", info.ModTime().Format("Jan 02")),
	})

	// Check .env file
	envPath := filepath.Join(scrappyDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		envInfo, _ := os.Stat(envPath)
		r.Add(CheckResult{
			Title:    "User .env",
			Severity: SeverityPass,
			Message:  fmt.Sprintf("~/.scrappy/.env (%d bytes)", envInfo.Size()),
		})
	}
}

func checkNetwork(r *Report) {
	// Check DNS resolution
	_, err := net.LookupHost("google.com")
	if err != nil {
		r.Add(CheckResult{
			Title:    "DNS resolution",
			Severity: SeverityFail,
			Message:  fmt.Sprintf("Cannot resolve google.com: %v", err),
		})
		return
	}

	r.Add(CheckResult{
		Title:    "DNS resolution",
		Severity: SeverityPass,
		Message:  "google.com resolves OK",
	})

	// Check HTTP connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", "google.com:80")
	if err != nil {
		r.Add(CheckResult{
			Title:    "Internet connectivity",
			Severity: SeverityFail,
			Message:  fmt.Sprintf("Cannot reach google.com:80 — %v", err),
		})
		return
	}
	conn.Close()

	r.Add(CheckResult{
		Title:    "Internet connectivity",
		Severity: SeverityPass,
		Message:  "google.com:80 reachable",
	})

	// Check proxy connectivity if set
	proxyURL := os.Getenv("SCRAPPY_PROXIES")
	if proxyURL != "" {
		parts := strings.Split(proxyURL, ",")
		if u, err := url.Parse(strings.TrimSpace(parts[0])); err == nil {
			host := u.Hostname()
			port := u.Port()
			if port == "" {
				port = "1080"
			}
			ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel2()
			conn2, err := (&net.Dialer{}).DialContext(ctx2, "tcp", net.JoinHostPort(host, port))
			if err != nil {
				r.Add(CheckResult{
					Title:    "Proxy reachability",
					Severity: SeverityFail,
					Message:  fmt.Sprintf("Cannot reach proxy %s: %v", host, err),
				})
			} else {
				conn2.Close()
				r.Add(CheckResult{
					Title:    "Proxy reachability",
					Severity: SeverityPass,
					Message:  fmt.Sprintf("%s is reachable", u.Redacted()),
				})
			}
		}
	}
}

func checkPlaywright(r *Report) {
	// Check if node is available for Playwright fallback
	cmd := exec.Command("node", "--version")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	nodeVer := strings.TrimSpace(string(out))

	// Check if the playwright script exists
	scriptPath := filepath.Join("scripts", "browser-fallback.mjs")
	if _, err := os.Stat(scriptPath); err == nil {
		r.Add(CheckResult{
			Title:    "Playwright fallback",
			Severity: SeverityPass,
			Message:  fmt.Sprintf("Node %s + script found", nodeVer),
		})
	} else {
		r.Add(CheckResult{
			Title:    "Playwright fallback",
			Severity: SeverityInfo,
			Message:  fmt.Sprintf("Node %s available, but scripts/browser-fallback.mjs not found", nodeVer),
		})
	}
}

func checkDataDir(r *Report, fixMode bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	scrappyDir := filepath.Join(home, ".scrappy")
	if _, err := os.Stat(scrappyDir); os.IsNotExist(err) {
		r.Add(CheckResult{
			Title:    "~/.scrappy/ data dir",
			Severity: SeverityWarn,
			Message:  "Does not exist yet",
			Fix: func() error {
				return os.MkdirAll(scrappyDir, 0700)
			},
		})
	}
}

// ExecuteFixes applies all fixable check results.
func (r *Report) ExecuteFixes(ctx context.Context) {
	for i, c := range r.Results {
		if c.Fix != nil {
			// Ensure we don't try to run fixes if context is cancelled
			if ctx.Err() != nil {
				return
			}
			if err := c.Fix(); err != nil {
				r.Results[i].Severity = SeverityFail
				r.Results[i].Message = fmt.Sprintf("Fix failed: %v", err)
				continue
			}
			r.Results[i].Severity = SeverityFix
			r.Results[i].Message = "Auto-fixed"
			r.Fixed++
			r.Passed++

			util.Info("doctor_fix", map[string]any{"check": c.Title, "status": "fixed"})
		}
	}
}
