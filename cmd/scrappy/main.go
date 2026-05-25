package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/export"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
	"github.com/arinbalyan/scrappy/pkg/scrappy"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var version = "0.1.0" // overridden at build via -ldflags="-X main.version=x.y.z"
const ascii = "\033[38;5;117m" + `
  ███████╗ ██████╗██████╗  █████╗ ██████╗ ██████╗ ██╗   ██╗
  ██╔════╝██╔════╝██╔══██╗██╔══██╗██╔══██╗██╔══██╗╚██╗ ██╔╝
  ███████╗██║     ██████╔╝███████║██████╔╝██████╔╝ ╚████╔╝
  ╚════██║██║     ██╔══██╗██╔══██║██╔═══╝ ██╔═══╝   ╚██╔╝
  ███████║╚██████╗██║  ██║██║  ██║██║     ██║        ██║
  ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝        ╚═╝
` + "\033[0m"

const longHelp = `Bulk job-board scraper for 55+ sites — Go-native, high concurrency,
low memory, and bulk-first design.

SETUP
  API keys (required for 5 sites):
    ADZUNA_APP_ID / ADZUNA_APP_KEY   https://developer.adzuna.com/
    CAREERJET_AFFID                   https://www.careerjet.com/partners/
    INFOJOBS_CLIENT_ID/SECRET         https://developer.infojobs.net/
    FINDWORK_API_KEY                  https://findwork.dev/developers/
    ARBEITSAGENTUR_API_KEY            https://rest.arbeitsagentur.de/
  Optional env vars:
    SCRAPPY_PROXIES                   socks5://user:pass@host:port,...
    SCRAPPY_LOG_LEVEL                 DEBUG|INFO|WARN|ERROR
    SCRAPPY_GREENHOUSE_SEEDS          comma-separated company names
    SCRAPPY_INDEED_API_KEY            Indeed API key (paid)
    SCRAPPY_INDEED_CO                 Indeed company override
    SCRAPPY_PROXY_ROTATE_EVERY_N      rotate every N requests
    SCRAPPY_PROXY_STICKY_WINDOW_N     stickiness window

CONFIGURATION FILES (auto-detected)
  1. config.yaml in current directory
  2. ~/.scrappy/config.yaml (user-wide defaults)

  .env files are loaded from the same locations (beside config).

  Run 'scrappy' without flags to enter interactive mode, which
  can save your preferences to ~/.scrappy/config.yaml.

COMMANDS
  scrape    Run a scraping job (default command)
  doctor    Run diagnostics on your scrappy setup (config, env, network)
  setup     Interactive setup wizard to create your configuration

FLAGS
  --search             Comma-separated search terms (e.g. "AI Engineer,Software Dev")
  --location           Comma-separated locations (e.g. "Remote,New York,Hyderabad")
  --sites              Comma-separated site names (empty = all 55+)
  --results-wanted     Max results total
  --format             Output: jsonl (default), csv, xlsx, parquet
  --out                Output file path (empty = stdout)
  --timeout            Scrape timeout in seconds (default 600)
  --proxy              Comma-separated proxy URLs (socks5://, http://)
  --email              Only include jobs with >= 1 email
  --is-remote          Only jobs flagged as remote (location-independent filter)
  --remote-only        Only truly remote jobs (no location filter applied)
  --job-type           Filter: fulltime|parttime|contract|internship
  --hours-old          Only jobs posted within this many hours (0 = no filter)
  --log-level          Log verbosity: DEBUG|INFO|WARN|ERROR
  --config             Path to per-site config yaml
  --memory-cap         Memory budget: "512MB", "1GB", "256"=MB (0=unlimited).
                       Enables memory-pressure monitor; auto-scales concurrency.
  --json-pretty        Pretty-print JSON output on stdout (default: auto-detect)
  --json-minify        Force minified JSON output even on TTY stdout
  --non-interactive    Disable interactive wizard (for scripts)
  --interactive        Force interactive mode (default: auto)

SITES (55 total)
  linkedin, indeed, naukri,
  internshala, builtin, startupjobs, greenhouse, gunio,
  himalayas, hiringcafe, huggingfacejobs, jobindex, remoteok,
  remotive, remotefirstjobs, jobspresso, hasjob,
  vuejobs, larajobs, arbeitnow, hackernews,
  cryptocurrencyjobs, androidjobs, jobicy, devopsjobs,
  crunchboard, cryptojobslist,
  dribbble, aijobs, workingnomads,
  ycjobs, ukvisajobs, google, glassdoor, adzuna,
  simplyhired, careerbuilder, careerjet, jooble, dice,
  monster, infojobs, reed, themuse, jobsdb,
  snagajob, djinni, headhunter, mycareersfuture, jobstreet,
  4dayweek, eurojobs, findwork,
  web3career, arbeitsagentur

EXAMPLES
  Interactive wizard:
    scrappy

  Scrape a few sites:
    scrappy --sites linkedin,indeed,glassdoor --search "software engineer" \
      --location "San Francisco" --results-wanted 500

  Export to CSV:
    scrappy --sites remoteok,remotive --search "rust" \
      --format csv --out jobs.csv --results-wanted 100

  Multi-value (cartesian product: 2 terms × 2 locations = 4 passes per site):
    scrappy --sites indeed --search "AI Engineer,Software Engineer" \
      --location "Remote,New York" --results-wanted 500

  Filter by job type and remote:
    scrappy --sites linkedin,indeed --search "rust developer" \
      --is-remote --job-type fulltime --results-wanted 100

  Memory-constrained (512 MB cap):
    scrappy --sites linkedin,indeed --search "golang" --location "Remote" \
      --memory-cap 512MB --results-wanted 200 --format jsonl

  Non-interactive for cron/CI:
    scrappy --sites indeed --search "golang" --location "Remote" \
      --results-wanted 200 --format jsonl --out /data/jobs.jsonl \
      --non-interactive

  Single SOCKS5 proxy (avoid rate limits):
    scrappy --sites linkedin,indeed,glassdoor --search "AI Engineer" \
      --location "Remote" --results-wanted 500 \
      --proxy socks5://user:pass@proxy:1080

  Multi-proxy round-robin:
    scrappy --sites indeed,google,zip_recruiter --search "developer" \
      --location "Remote" --results-wanted 300 \
      --proxy socks5://proxy1:1080,socks5://proxy2:1080

  Proxy from env / config (config.yaml overrides env, --proxy overrides both):
    SCRAPPY_PROXIES=socks5://user:pass@proxy:1080 \
      scrappy --sites linkedin --search "engineer" --results-wanted 100

ENVIRONMENT
  SCRAPPY_LOG_LEVEL    Default log level
  SCRAPPY_PROXIES      Comma-separated SOCKS5/HTTP proxy URLs (lowest priority)

  PROXY PRECEDENCE: --proxy CLI flag  >  config.yaml proxy: field  >  SCRAPPY_PROXIES env`

func homeDir() string {
	u, err := user.Current()
	if err != nil {
		return os.Getenv("HOME")
	}
	return u.HomeDir
}

func defaultConfigPath() string {
	cwd := "config.yaml"
	if _, err := os.Stat(cwd); err == nil {
		return "config.yaml"
	}
	userCfg := filepath.Join(homeDir(), ".scrappy", "config.yaml")
	if _, err := os.Stat(userCfg); err == nil {
		return userCfg
	}
	return "config.yaml"
}

func defaultEnvPath(configPath string) string {
	dir := filepath.Dir(configPath)
	if env := filepath.Join(dir, ".env"); dir != "." {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	dotEnv := filepath.Join(homeDir(), ".scrappy", ".env")
	if _, err := os.Stat(dotEnv); err == nil {
		return dotEnv
	}
	// Fall back to .env beside the config file even if it doesn't exist yet.
	return filepath.Join(dir, ".env")
}

func userConfigDir() string {
	return filepath.Join(homeDir(), ".scrappy")
}

type cliConfig struct {
	Search         string
	Location       string
	Sites          string
	ResultsWanted  int
	Format         string
	Out            string
	Timeout        int
	Interactive    bool
	NonInteractive bool
	LogLevel       string
	ConfigPath     string
	EmailOnly      bool
	MemoryCap      string
	IsRemote       bool
	RemoteOnly     bool
	JobType        string
	Proxy          string
	MinScore         int
	MaxRPS           int
	SiteRPS          string
	Dedup            bool
	DedupByCompany   bool
	HoursOld         int
	SinceDate        string
	JSONPretty       bool
	JSONMinify       bool
	Schema           bool
	VersionJSON      bool
}

type multiString []string

func (s *multiString) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		term := strings.TrimSpace(value.Value)
		if term == "" {
			*s = nil
			return nil
		}
		*s = []string{term}
		return nil
	case yaml.SequenceNode:
		terms := make([]string, 0, len(value.Content))
		for _, n := range value.Content {
			if n.Kind != yaml.ScalarNode {
				continue
			}
			term := strings.TrimSpace(n.Value)
			if term == "" {
				continue
			}
			terms = append(terms, term)
		}
		if len(terms) == 0 {
			*s = nil
			return nil
		}
		*s = terms
		return nil
	default:
		return fmt.Errorf("field must be a string or list of strings")
	}
}

type siteTarget struct {
	Search   multiString `yaml:"search"`
	Location multiString `yaml:"location"`
	Country  string      `yaml:"country,omitempty"`
	IsRemote *bool       `yaml:"is_remote,omitempty"`
}

type appConfig struct {
	Defaults struct {
		Search        multiString `yaml:"search"`
		Location      multiString `yaml:"location"`
		ResultsWanted int         `yaml:"results_wanted"`
		Out           string      `yaml:"out"`
		Format        string      `yaml:"format"`
		MemoryCap     string      `yaml:"memory_cap"`
		IsRemote      bool        `yaml:"is_remote"`
		RemoteOnly    bool        `yaml:"remote_only"`
		JobType       string      `yaml:"job_type"`
	} `yaml:"defaults"`
	Proxy string                `yaml:"proxy,omitempty"`
	Sites map[string]siteTarget `yaml:"sites"`
}

var loadDotEnvOnce sync.Once

// apiKeySites lists sites that need env vars and where to get them.
var apiKeySites = []struct {
	Site     string
	EnvVars  []string
	SetupURL string
}{
	{Site: "adzuna", EnvVars: []string{"ADZUNA_APP_ID", "ADZUNA_APP_KEY"}, SetupURL: "https://developer.adzuna.com/"},
	{Site: "careerjet", EnvVars: []string{"CAREERJET_AFFID"}, SetupURL: "https://www.careerjet.com/partners/"},
	{Site: "infojobs", EnvVars: []string{"INFOJOBS_CLIENT_ID", "INFOJOBS_CLIENT_SECRET"}, SetupURL: "https://developer.infojobs.net/"},
	{Site: "findwork", EnvVars: []string{"FINDWORK_API_KEY"}, SetupURL: "https://findwork.dev/developers/"},
	{Site: "arbeitsagentur", EnvVars: []string{"ARBEITSAGENTUR_API_KEY"}, SetupURL: "https://rest.arbeitsagentur.de/"},
}

var rootCmd *cobra.Command

func main() {
	cfg := &cliConfig{}
	rootCmd = newRootCommand(cfg)
	registerSubcommands()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func registerSubcommands() {
	rootCmd.AddCommand(newDoctorCommand())
	rootCmd.AddCommand(newSetupCommand())
}

func newRootCommand(cfg *cliConfig) *cobra.Command {
	root := &cobra.Command{
		Use:     "scrappy",
		Short:   "Bulk job-board scraper for 55+ sites",
		Long:    longHelp,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Early exits before any scraping logic.
			if cfg.Schema {
				printSchema()
				return nil
			}
			if cfg.VersionJSON {
				printVersionJSON(cfg.JSONPretty, cfg.JSONMinify)
				return nil
			}
			if cfg.NonInteractive {
				cfg.Interactive = false
			}
			// Auto-detect interactive mode: enable when no search term given, on a TTY.
			if !cfg.Interactive && !cfg.NonInteractive && cfg.Search == "" && cfg.Sites == "" {
				if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
					cfg.Interactive = true
				}
			}
			if cfg.Interactive {
				if fi, err := os.Stdin.Stat(); err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
					cfg.Interactive = false
				}
			}
			if cfg.Interactive {
				runInteractive(cfg)
			}
			return runOnce(cfg)
		},
	}

	root.Flags().StringVar(&cfg.Search, "search", "", "search term (e.g. \"software engineer\")")
	root.Flags().StringVar(&cfg.Location, "location", "", "search location (e.g. \"San Francisco, CA\")")
	root.Flags().StringVar(&cfg.Sites, "sites", "", "comma-separated site names (empty = all 55+)")
	root.Flags().IntVar(&cfg.ResultsWanted, "results-wanted", 0, "max results")
	root.Flags().StringVar(&cfg.Format, "format", "", "output format: jsonl|csv|xlsx|parquet")
	root.Flags().StringVar(&cfg.Out, "out", "", "output path (empty = stdout)")
	root.Flags().BoolVar(&cfg.Interactive, "interactive", false, "interactive wizard mode (auto-detected when no args given on TTY)")
	root.Flags().BoolVar(&cfg.NonInteractive, "non-interactive", false, "disable interactive wizard")
	root.Flags().StringVar(&cfg.LogLevel, "log-level", "", "log level: DEBUG|INFO|WARN|ERROR")
	root.Flags().StringVar(&cfg.ConfigPath, "config", defaultConfigPath(), "path to config yaml")
	root.Flags().BoolVar(&cfg.EmailOnly, "email", false, "only include jobs with at least one email")
	root.Flags().IntVar(&cfg.Timeout, "timeout", 600, "scrape timeout in seconds")
	root.Flags().StringVar(&cfg.MemoryCap, "memory-cap", "", "memory budget (e.g. 512MB, 1GB)")
	root.Flags().BoolVar(&cfg.IsRemote, "is-remote", false, "only jobs flagged as remote")
	root.Flags().BoolVar(&cfg.RemoteOnly, "remote-only", false, "only truly remote jobs (no location)")
	root.Flags().StringVar(&cfg.JobType, "job-type", "", "filter: fulltime|parttime|contract|internship")
	root.Flags().StringVar(&cfg.Proxy, "proxy", os.Getenv("SCRAPPY_PROXIES"), "comma-separated proxy URLs (socks5://, http://); TCP-dial health check at startup, unhealthy proxies excluded; takes precedence over config.yaml proxy: and SCRAPPY_PROXIES env")
	root.Flags().IntVar(&cfg.MinScore, "min-score", 0, "quality score floor (0-100)")
	root.Flags().IntVar(&cfg.MaxRPS, "max-rps", 0, "global max requests per second (overrides per-site defaults)")
	root.Flags().StringVar(&cfg.SiteRPS, "site-rps", "", "per-site RPS overrides, e.g. linkedin:1,indeed:10")
	root.Flags().IntVar(&cfg.HoursOld, "hours-old", 0, "only jobs posted within this many hours (0 = no filter)")
	root.Flags().StringVar(&cfg.SinceDate, "since", "", "only jobs posted on or after this date (RFC3339 or YYYY-MM-DD)")
	root.Flags().BoolVar(&cfg.JSONPretty, "json-pretty", false, "pretty-print JSON output (stdout only, default: auto-detect)")
	root.Flags().BoolVar(&cfg.JSONMinify, "json-minify", false, "force minified JSON output even on stdout")
	root.Flags().BoolVar(&cfg.Schema, "schema", false, "print JSON Schema for JobPost type and exit")
	root.Flags().BoolVar(&cfg.VersionJSON, "version-json", false, "print version info as JSON and exit")
	root.Flags().BoolVar(&cfg.Dedup, "dedup", true, "deduplicate jobs by URL across sites")
	root.Flags().BoolVar(&cfg.DedupByCompany, "dedup-by-company", false, "keep only one posting per company")
	root.SetVersionTemplate("scrappy v{{.Version}}\n")
	return root
}

func runInteractive(cfg *cliConfig) {
	fmt.Print(ascii)
	fmt.Printf("\033[38;5;117mscrappy v%s \033[0m— interactive mode\n", version)
	fmt.Println(strings.Repeat("─", 50))

	cfgPath := cfg.ConfigPath
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	if b, err := os.ReadFile(cfgPath); err == nil {
		var ac appConfig
		_ = yaml.Unmarshal(b, &ac)
		if len(ac.Defaults.Search) > 0 && cfg.Search == "" {
			cfg.Search = strings.Join(ac.Defaults.Search, ",")
		}
		if len(ac.Defaults.Location) > 0 && cfg.Location == "" {
			cfg.Location = strings.Join(ac.Defaults.Location, ",")
		}
		if ac.Defaults.IsRemote {
			cfg.IsRemote = true
		}
		if cfg.JobType == "" && ac.Defaults.JobType != "" {
			cfg.JobType = ac.Defaults.JobType
		}
		if ac.Defaults.RemoteOnly {
			cfg.RemoteOnly = true
		}
		if ac.Defaults.ResultsWanted > 0 && cfg.ResultsWanted <= 0 {
			cfg.ResultsWanted = ac.Defaults.ResultsWanted
		}
		if ac.Defaults.Format != "" && cfg.Format == "" {
			cfg.Format = ac.Defaults.Format
		}
		if ac.Defaults.Out != "" && cfg.Out == "" {
			cfg.Out = ac.Defaults.Out
		}
		fmt.Printf("Loaded defaults from: %s\n\n", cfgPath)
	} else if os.IsNotExist(err) {
		fmt.Println("No config found — using defaults")
	} else {
		fmt.Println()
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println(" \033[38;5;117m╭─ Main Settings ───────────────────────────────────╮\033[0m")
	cfg.Search = ask(reader, "  Search term (e.g. \"AI Engineer\" or \"software engineer\")", cfg.Search)
	cfg.Location = ask(reader, "  Location (e.g. \"Remote\" or \"San Francisco, CA\")", cfg.Location)
	cfg.Sites = ask(reader, "  Sites (comma-separated, empty=all 55+, e.g. linkedin,indeed)", cfg.Sites)
	cfg.ResultsWanted = askInt(reader, "  Results wanted (0 = unlimited)", cfg.ResultsWanted)
	fmt.Println(" \033[38;5;117m╰────────────────────────────────────────────────────╯\033[0m")

	fmt.Println(" \033[38;5;117m╭─ Output Settings ───────────────────────────────────╮\033[0m")
	cfg.Format = ask(reader, "  Format (jsonl/csv/xlsx/parquet)", cfg.Format)
	cfg.Out = ask(reader, "  Output path (empty = stdout)", cfg.Out)
	fmt.Println(" \033[38;5;117m╰────────────────────────────────────────────────────╯\033[0m")

	fmt.Println(" \033[38;5;117m╭─ Filters ──────────────────────────────────────────╮\033[0m")
	irDef := "n"
	if cfg.IsRemote {
		irDef = "y"
	}
	cfg.IsRemote = askBool(reader, "  Only remote jobs? (y/n)", irDef)
	jtDef := cfg.JobType
	if jtDef == "" {
		jtDef = "any"
	}
	cfg.JobType = ask(reader, "  Job type (fulltime/parttime/contract/internship/any)", jtDef)
	if cfg.JobType == "any" {
		cfg.JobType = ""
	}
	cfg.HoursOld = askInt(reader, "  Only jobs posted within last N hours (0 = no filter)", cfg.HoursOld)
	memDefault := cfg.MemoryCap
	if memDefault == "" {
		memDefault = "0"
	}
	cfg.MemoryCap = ask(reader, "  Memory cap (e.g. 512MB, 1GB, 0=unlimited)", memDefault)
	fmt.Println(" \033[38;5;117m╰────────────────────────────────────────────────────╯\033[0m")

	fmt.Println(" \033[38;5;117m╭─ Network ──────────────────────────────────────────╮\033[0m")
	proxyDefault := cfg.Proxy
	if proxyDefault == "" {
		if v := os.Getenv("SCRAPPY_PROXIES"); v != "" {
			proxyDefault = v
		}
	}
	cfg.Proxy = ask(reader, "  Proxy (socks5://user:pass@host:port, comma-separated)", proxyDefault)
	fmt.Println(" \033[38;5;117m╰────────────────────────────────────────────────────╯\033[0m")

	fmt.Println()
	fmt.Printf(" \033[38;5;240mTip:\033[0m For site-specific searches, add per-site config to \033[38;5;117m%s\033[0m\n", defaultConfigPath())
	fmt.Println(" \033[38;5;240m     Example:\033[0m")
	fmt.Println(" \033[38;5;240m       sites:\033[0m")
	fmt.Println(" \033[38;5;240m         indeed:\033[0m")
	fmt.Println(" \033[38;5;240m           search: '\"AI Engineer\" OR \"ML Engineer\"'\033[0m")
	fmt.Println(" \033[38;5;240m           location: Remote\033[0m")
	fmt.Println(" \033[38;5;240m           country: germany   # uses indeed-co header\033[0m")
	fmt.Println(" \033[38;5;240m         linkedin:\033[0m")
	fmt.Println(" \033[38;5;240m           search: 'AI Engineer OR ML Engineer'\033[0m")
	fmt.Println(" \033[38;5;240m         reed:\033[0m")
	fmt.Println(" \033[38;5;240m           location: United Kingdom\033[0m")
	fmt.Println()
}

func ask(r *bufio.Reader, label, def string) string {
	fmt.Printf("  \033[38;5;117m?\033[0m %s [%s]: ", label, def)
	in, _ := r.ReadString('\n')
	in = strings.TrimSpace(in)
	if in == "" {
		return def
	}
	return in
}

func askInt(r *bufio.Reader, label string, def int) int {
	v := ask(r, label, strconv.Itoa(def))
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func askBool(r *bufio.Reader, label, def string) bool {
	v := ask(r, label, def)
	return v == "y" || v == "yes" || v == "true"
}

func runOnce(cfg *cliConfig) error {
	envPath := defaultEnvPath(cfg.ConfigPath)
	loadDotEnvOnce.Do(func() { loadDotEnv(envPath) })
	level := strings.TrimSpace(cfg.LogLevel)
	if level == "" {
		level = strings.TrimSpace(os.Getenv("SCRAPPY_LOG_LEVEL"))
	}
	util.SetLogLevel(level)

	sites := parseSites(cfg.Sites)
	if len(sites) == 0 {
		sites = make([]model.Site, len(model.AllSites()))
		copy(sites, model.AllSites())
	}
	ac := loadAppConfig(cfg.ConfigPath)

	// Proxy setup: CLI flag → config → env var
	proxyRaw := strings.TrimSpace(cfg.Proxy)
	if proxyRaw == "" {
		proxyRaw = strings.TrimSpace(ac.Proxy)
	}
	if proxyRaw == "" {
		proxyRaw = strings.TrimSpace(os.Getenv("SCRAPPY_PROXIES"))
	}
	if v := strings.TrimSpace(proxyRaw); v != "" {
		parts := strings.Split(v, ",")
		healthy := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			u, err := url.Parse(p)
			if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
				util.Warn("proxy_parse_fail", map[string]any{"proxy": p, "err": err})
				continue
			}
			host := u.Hostname()
			port := u.Port()
			if port == "" {
				switch u.Scheme {
				case "http", "https":
					port = "80"
				case "socks5", "socks5h":
					port = "1080"
				default:
					port = "80"
				}
			}
			conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(host, port), 500*time.Millisecond)
			if dialErr != nil {
				util.Warn("proxy_unreachable", map[string]any{"proxy": p, "err": dialErr.Error()})
				continue
			}
			conn.Close()
			healthy = append(healthy, p)
		}
		if len(healthy) > 0 {
			_ = os.Setenv("SCRAPPY_PROXIES", strings.Join(healthy, ","))
			var redacted []string
		for _, p := range healthy {
			if u, err := url.Parse(p); err == nil {
				redacted = append(redacted, u.Redacted())
			} else {
				redacted = append(redacted, p)
			}
		}
		util.Info("proxy_setup", map[string]any{"healthy": len(healthy), "total": len(parts), "proxies": strings.Join(redacted, ",")})
		} else {
			util.Warn("proxy_no_healthy", map[string]any{"total": len(parts)})
		}
	}

	// Collect global search terms (CLI comma-separated → slice).
	var searchTerms []string
	if v := strings.TrimSpace(cfg.Search); v != "" {
		searchTerms = splitCommas(v)
	}
	if len(searchTerms) == 0 && len(ac.Defaults.Search) > 0 {
		searchTerms = make([]string, 0, len(ac.Defaults.Search))
		for _, s := range ac.Defaults.Search {
			searchTerms = append(searchTerms, splitCommas(s)...)
		}
	}

	// Collect global locations (CLI comma-separated → slice).
	var locations []string
	if v := strings.TrimSpace(cfg.Location); v != "" {
		locations = splitCommas(v)
	}
	if len(locations) == 0 && len(ac.Defaults.Location) > 0 {
		locations = make([]string, 0, len(ac.Defaults.Location))
		for _, l := range ac.Defaults.Location {
			locations = append(locations, splitCommas(l)...)
		}
	}

	resultsWanted := cfg.ResultsWanted
	if resultsWanted <= 0 && ac.Defaults.ResultsWanted > 0 {
		resultsWanted = ac.Defaults.ResultsWanted
	}
	outPath := strings.TrimSpace(cfg.Out)
	if outPath == "" {
		outPath = strings.TrimSpace(ac.Defaults.Out)
	}
	format := strings.TrimSpace(cfg.Format)
	if format == "" {
		format = strings.TrimSpace(ac.Defaults.Format)
	}

	// Memory cap: CLI flag takes precedence, falls back to config default.
	memCapRaw := strings.TrimSpace(cfg.MemoryCap)
	if memCapRaw == "" {
		memCapRaw = strings.TrimSpace(ac.Defaults.MemoryCap)
	}
	memoryCapMB := parseMemoryCap(memCapRaw)

	// IsRemote / RemoteOnly / JobType from config defaults (CLI flag takes precedence).
	if ac.Defaults.IsRemote {
		cfg.IsRemote = true
	}
	if ac.Defaults.RemoteOnly {
		cfg.RemoteOnly = true
	}
	if cfg.JobType == "" && ac.Defaults.JobType != "" {
		cfg.JobType = ac.Defaults.JobType
	}

	siteSearch := map[model.Site][]string{}
	siteLocations := map[model.Site][]string{}
	siteLocation := map[model.Site]string{}
	siteCountry := map[model.Site]model.Country{}
	for _, s := range sites {
		if t, ok := ac.Sites[string(s)]; ok {
			if len(t.Search) > 0 {
				terms := make([]string, 0, len(t.Search))
				for _, term := range t.Search {
					term = strings.TrimSpace(term)
					if term != "" {
						terms = append(terms, term)
					}
				}
				if len(terms) > 0 {
					siteSearch[s] = terms
				}
			}
			if len(t.Location) > 0 {
				locs := make([]string, 0, len(t.Location))
				for _, loc := range t.Location {
					for _, part := range splitCommas(loc) {
						part = strings.TrimSpace(part)
						if part != "" {
							locs = append(locs, part)
						}
					}
				}
				if len(locs) > 0 {
					siteLocations[s] = locs
				}
				// Keep single-string SiteLocation for backward compat.
				siteLocation[s] = strings.Join(locs, ", ")
			}
				if t.Country != "" {
				siteCountry[s] = model.Country(t.Country)
			}
			if t.IsRemote != nil {
				// Per-site is_remote override — used at engine/site level.
			}
		}
	}

	// Fall back to single-term / single-location when no multi-value input.
	firstSearch := ""
	if len(searchTerms) > 0 {
		firstSearch = searchTerms[0]
	}
	firstLoc := ""
	if len(locations) > 0 {
		firstLoc = locations[0]
	}

	siteRPS := parseSiteRPS(cfg.SiteRPS)
	input := model.ScraperInput{
		Sites:          sites,
		SearchTerm:     firstSearch,
		Location:       firstLoc,
		SearchTerms:    searchTerms,
		Locations:      locations,
		ResultsWanted:  resultsWanted,
		HoursOld:       cfg.HoursOld,
		SinceDate:      cfg.SinceDate,
		Dedup:          cfg.Dedup,
		DedupByCompany: cfg.DedupByCompany,
		MinScore:       cfg.MinScore,
		MaxRPS:         cfg.MaxRPS,
		SiteRPS:        siteRPS,
		EmailsOnly:     cfg.EmailOnly,
		LogLevel:       level,
		SiteSearch:     siteSearch,
		SiteLocation:   siteLocation,
		SiteLocations:  siteLocations,
		SiteCountry:    siteCountry,
		MemoryCapMB:    memoryCapMB,
		IsRemote:       cfg.IsRemote,
		RemoteOnly:     cfg.RemoteOnly,
		JobType:        model.JobType(cfg.JobType),
	}

	constraints := scrappy.EvaluateConstraints(input)
	for _, w := range constraints.Warnings {
		fmt.Printf("[constraint-warning] %s\n", w)
	}
	if len(constraints.Errors) > 0 {
		return fmt.Errorf("constraint errors: %v", constraints.Errors)
	}

	engine := scrappy.NewEngine()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	start := time.Now()
	jobs, err := engine.Scrape(ctx, input)
	elapsed := time.Since(start)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\n  \033[38;5;117m✓\033[0m Scraped %d jobs in %s\n\n", len(jobs), elapsed.Round(time.Second))

	if outPath == "" {
		enc := json.NewEncoder(os.Stdout)
		if cfg.JSONPretty || (!cfg.JSONMinify) {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(jobs)
	}

	switch strings.ToLower(format) {
	case "csv":
		if err := export.WriteCSV(outPath, jobs); err != nil {
			return err
		}
	case "xlsx":
		if err := export.WriteXLSX(outPath, jobs); err != nil {
			return err
		}
	case "parquet":
		if err := export.WriteParquet(outPath, jobs); err != nil {
			return err
		}
	default:
		if err := export.WriteJSONL(outPath, jobs); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "  \033[38;5;117m✓\033[0m Wrote %d jobs to %s\n\n", len(jobs), outPath)

	// Save config prompt (only if we were in interactive mode).
	if cfg.Interactive {
		promptSaveConfig(cfg)
		runAPIKeyWizard()
	}
	return nil
}

func promptSaveConfig(cfg *cliConfig) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("  \033[38;5;117m?\033[0m Save these settings to ~/.scrappy/config.yaml? (y/N): ")
	in, _ := reader.ReadString('\n')
	in = strings.TrimSpace(strings.ToLower(in))
	if in != "y" && in != "yes" {
		fmt.Println("  Skipped.")
		return
	}

	dir := userConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError\033[0m creating %s: %v\n", dir, err)
		return
	}

	ac := appConfig{}
	ac.Defaults.Search = splitCommas(cfg.Search)
	ac.Defaults.Location = splitCommas(cfg.Location)
	ac.Defaults.ResultsWanted = cfg.ResultsWanted
	ac.Defaults.Format = cfg.Format
	ac.Defaults.Out = cfg.Out
	ac.Defaults.MemoryCap = cfg.MemoryCap
	ac.Proxy = cfg.Proxy
	ac.Defaults.IsRemote = cfg.IsRemote
	ac.Defaults.RemoteOnly = cfg.RemoteOnly
	ac.Defaults.JobType = cfg.JobType

	b, err := yaml.Marshal(&ac)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError\033[0m marshalling config: %v\n", err)
		return
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, b, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError\033[0m writing %s: %v\n", cfgPath, err)
		return
	}
	fmt.Printf("  \033[38;5;117m✓\033[0m Saved to %s (restricted permissions — may contain proxy credentials)\n", cfgPath)

	// Update config path so next run picks it up.
	cfg.ConfigPath = cfgPath
}

func runAPIKeyWizard() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n  Some sites require API keys to function:")
	for _, s := range apiKeySites {
		missing := false
		for _, ev := range s.EnvVars {
			if os.Getenv(ev) == "" {
				missing = true
				break
			}
		}
		status := "\033[32m✓\033[0m"
		if missing {
			status = "\033[33m✗\033[0m"
		}
		fmt.Printf("    %s %-14s %s\n", status, s.Site, strings.Join(s.EnvVars, ", "))
	}

	fmt.Print("\n  \033[38;5;117m?\033[0m Configure API keys now? (y/N): ")
	in, _ := reader.ReadString('\n')
	in = strings.TrimSpace(strings.ToLower(in))
	if in != "y" && in != "yes" {
		fmt.Println("  Skipped. Set env vars or add to ~/.scrappy/.env later.")
		return
	}

	var envLines []string
	for _, s := range apiKeySites {
		allSet := true
		for _, ev := range s.EnvVars {
			if os.Getenv(ev) == "" {
				allSet = false
				break
			}
		}
		if allSet {
			continue
		}
		fmt.Printf("\n    \033[38;5;117m→\033[0m %s: Get credentials at %s\n", s.Site, s.SetupURL)
		for _, ev := range s.EnvVars {
			v := ask(reader, fmt.Sprintf("  %s", ev), "")
			if v != "" {
				envLines = append(envLines, fmt.Sprintf("%s=%s", ev, v))
			}
		}
	}

	if len(envLines) == 0 {
		fmt.Println("\n  No new keys to save.")
		return
	}

	dir := userConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError\033[0m creating %s: %v\n", dir, err)
		return
	}

	envPath := filepath.Join(dir, ".env")
	content := strings.Join(envLines, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError\033[0m writing %s: %v\n", envPath, err)
		return
	}
	fmt.Printf("\n  \033[38;5;117m✓\033[0m Saved to %s (restricted permissions — may contain credentials)\n", envPath)
	fmt.Println("  Keys will load automatically on next run.")
}

// splitCommas splits a comma-separated string, trims whitespace,
// and filters empty entries.
func splitCommas(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseSiteRPS(v string) map[model.Site]int {
	m := map[model.Site]int{}
	if v == "" {
		return m
	}
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		parts := strings.SplitN(p, ":", 2)
		if len(parts) != 2 {
			continue
		}
		site := model.Site(strings.TrimSpace(parts[0]))
		rps, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || rps <= 0 || site == "" {
			continue
		}
		m[site] = rps
	}
	return m
}

func parseSites(v string) []model.Site {
	parts := strings.Split(v, ",")
	out := make([]model.Site, 0, len(parts))
	for _, p := range parts {
		s := model.Site(strings.TrimSpace(strings.ToLower(p)))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func loadAppConfig(path string) appConfig {
	var c appConfig
	if strings.TrimSpace(path) == "" {
		return c
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = yaml.Unmarshal(b, &c)
	if c.Sites == nil {
		c.Sites = map[string]siteTarget{}
	}
	return c
}

// parseMemoryCap converts a user-supplied string like "512MB", "1GB",
// or a plain number (as MB) into an integer MB value.  Empty or "0" → 0 (unlimited).
func parseMemoryCap(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	raw = strings.ToUpper(raw)
	switch {
	case strings.HasSuffix(raw, "GB"):
		n, err := strconv.Atoi(strings.TrimSuffix(raw, "GB"))
		if err == nil && n > 0 {
			return n * 1024
		}
	case strings.HasSuffix(raw, "MB"):
		n, err := strconv.Atoi(strings.TrimSuffix(raw, "MB"))
		if err == nil && n > 0 {
			return n
		}
	default:
		// Plain number → treat as MB.
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		if _, exists := os.LookupEnv(k); exists {
			continue
		}
		_ = os.Setenv(k, v)
	}
}

// printVersionJSON outputs version information as JSON.  Used by --version-json flag.
func printVersionJSON(pretty bool, minify bool) {
	info := map[string]interface{}{
		"version":   version,
		"sites":     len(model.AllSites()),
		"go":        strings.TrimPrefix(runtime.Version(), "go"),
		"formats":   []string{"jsonl", "csv", "xlsx", "parquet"},
	}
	if exe, err := os.Executable(); err == nil {
		if fi, err := os.Stat(exe); err == nil {
			info["binary_size_bytes"] = fi.Size()
			info["build_time"] = fi.ModTime().UTC().Format(time.RFC3339)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	if pretty || !minify {
		enc.SetIndent("", "  ")
	}
	_ = enc.Encode(info)
}
