package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
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

const version = "0.1.0"
const ascii = "\033[38;5;117m" + `
  ███████╗ ██████╗██████╗  █████╗ ██████╗ ██████╗ ██╗   ██╗
  ██╔════╝██╔════╝██╔══██╗██╔══██╗██╔══██╗██╔══██╗╚██╗ ██╔╝
  ███████╗██║     ██████╔╝███████║██████╔╝██████╔╝ ╚████╔╝
  ╚════██║██║     ██╔══██╗██╔══██║██╔═══╝ ██╔═══╝   ╚██╔╝
  ███████║╚██████╗██║  ██║██║  ██║██║     ██║        ██║
  ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝        ╚═╝
` + "\033[0m"

const longHelp = `Bulk job-board scraper for 65+ sites — Go-native, high concurrency,
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

FLAGS
  --search             Comma-separated search terms (e.g. "AI Engineer,Software Dev")
  --location           Comma-separated locations (e.g. "Remote,New York,Hyderabad")
  --sites              Comma-separated site names (empty = all 65+)
  --results-wanted     Max results total
  --format             Output: jsonl (default), csv, xlsx, parquet
  --out                Output file path (empty = stdout)
  --timeout            Scrape timeout in seconds (default 600)
  --email              Only include jobs with >= 1 email
  --is-remote          Only jobs flagged as remote (location-independent filter)
  --remote-only        Only truly remote jobs (no location filter applied)
  --job-type           Filter: fulltime|parttime|contract|internship
  --log-level          Log verbosity: DEBUG|INFO|WARN|ERROR
  --config             Path to per-site config yaml
  --memory-cap         Memory budget: "512MB", "1GB", "256"=MB (0=unlimited)
  --non-interactive    Disable interactive wizard (for scripts)
  --interactive        Force interactive mode (default: auto)

SITES (65 total)
  linkedin, indeed, zip_recruiter, bayt, bdjobs, naukri,
  internshala, builtin, startupjobs, greenhouse, gunio,
  himalayas, hiringcafe, huggingfacejobs, jobindex, remoteok,
  remotive, remotefirstjobs, jobspresso, hasjob,
  vuejobs, larajobs, arbeitnow, hackernews,
  cryptocurrencyjobs, androidjobs, jobicy, devopsjobs,
  crunchboard, iosdevjobs, swissdevjobs, cryptojobslist,
  devitjobs, dribbble, aijobs, workingnomads, wuzzuf,
  ycjobs, ukvisajobs, google, glassdoor, adzuna,
  simplyhired, careerbuilder, careerjet, jooble, dice,
  monster, stepstone, infojobs, reed, themuse, jobsdb,
  snagajob, djinni, headhunter, mycareersfuture, jobstreet,
  upwork, 4dayweek, academiccareers, eurojobs, findwork,
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

ENVIRONMENT
  SCRAPPY_LOG_LEVEL    Default log level
  SCRAPPY_PROXIES      Comma-separated SOCKS5 proxy URLs`

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

func main() {
	cfg := &cliConfig{}
	root := newRootCommand(cfg)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(cfg *cliConfig) *cobra.Command {
	root := &cobra.Command{
		Use:     "scrappy",
		Short:   "Bulk job-board scraper for 65+ sites",
		Long:    longHelp,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.NonInteractive {
				cfg.Interactive = false
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
	root.Flags().StringVar(&cfg.Sites, "sites", "", "comma-separated site names (empty = all 65+)")
	root.Flags().IntVar(&cfg.ResultsWanted, "results-wanted", 0, "max results")
	root.Flags().StringVar(&cfg.Format, "format", "", "output format: jsonl|csv|xlsx|parquet")
	root.Flags().StringVar(&cfg.Out, "out", "", "output path (empty = stdout)")
	root.Flags().BoolVar(&cfg.Interactive, "interactive", true, "interactive wizard mode")
	root.Flags().BoolVar(&cfg.NonInteractive, "non-interactive", false, "disable interactive wizard")
	root.Flags().StringVar(&cfg.LogLevel, "log-level", "", "log level: DEBUG|INFO|WARN|ERROR")
	root.Flags().StringVar(&cfg.ConfigPath, "config", defaultConfigPath(), "path to config yaml")
	root.Flags().BoolVar(&cfg.EmailOnly, "email", false, "only include jobs with at least one email")
	root.Flags().IntVar(&cfg.Timeout, "timeout", 600, "scrape timeout in seconds")
	root.Flags().StringVar(&cfg.MemoryCap, "memory-cap", "", "memory budget (e.g. 512MB, 1GB)")
	root.Flags().BoolVar(&cfg.IsRemote, "is-remote", false, "only jobs flagged as remote")
	root.Flags().BoolVar(&cfg.RemoteOnly, "remote-only", false, "only truly remote jobs (no location)")
	root.Flags().StringVar(&cfg.JobType, "job-type", "", "filter: fulltime|parttime|contract|internship")
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
		if cfg.JobType == "" && ac.Defaults.JobType != "" {
			cfg.JobType = ac.Defaults.JobType
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
	cfg.Search = ask(reader, "Search term (e.g. \"software engineer\")", cfg.Search)
	cfg.Location = ask(reader, "Location (e.g. \"San Francisco, CA\" or \"Remote\")", cfg.Location)
	cfg.Sites = ask(reader, "Sites (comma-separated, empty=all)", cfg.Sites)
	cfg.ResultsWanted = askInt(reader, "Results wanted", cfg.ResultsWanted)
	cfg.Format = ask(reader, "Format (jsonl/csv/xlsx/parquet)", cfg.Format)
	cfg.Out = ask(reader, "Output path (empty = stdout)", cfg.Out)
	irDef := "n"
	if cfg.IsRemote {
		irDef = "y"
	}
	cfg.IsRemote = askBool(reader, "Only show remote jobs? (y/n)", irDef)
	jtDef := cfg.JobType
	if jtDef == "" {
		jtDef = "any"
	}
	cfg.JobType = ask(reader, "Job type (fulltime/parttime/contract/internship/any)", jtDef)
	if cfg.JobType == "any" {
		cfg.JobType = ""
	}
	memDefault := cfg.MemoryCap
	if memDefault == "" {
		memDefault = "0"
	}
	cfg.MemoryCap = ask(reader, "Memory cap (e.g. 512MB, 1GB, 0=unlimited)", memDefault)
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
	// Collect global search terms (CLI comma-separated → slice).
	var searchTerms []string
	if v := strings.TrimSpace(cfg.Search); v != "" {
		searchTerms = splitCommas(v)
	}
	if len(searchTerms) == 0 && len(ac.Defaults.Search) > 0 {
		searchTerms = ac.Defaults.Search
	}

	// Collect global locations (CLI comma-separated → slice).
	var locations []string
	if v := strings.TrimSpace(cfg.Location); v != "" {
		locations = splitCommas(v)
	}
	if len(locations) == 0 && len(ac.Defaults.Location) > 0 {
		locations = ac.Defaults.Location
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

	siteSearch := map[model.Site][]string{}
	siteLocations := map[model.Site][]string{}
	siteLocation := map[model.Site]string{}
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
					loc = strings.TrimSpace(loc)
					if loc != "" {
						locs = append(locs, loc)
					}
				}
				if len(locs) > 0 {
					siteLocations[s] = locs
				}
				// Keep single-string SiteLocation for backward compat.
				siteLocation[s] = strings.Join(locs, ", ")
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

	input := model.ScraperInput{
		Sites:          sites,
		SearchTerm:     firstSearch,
		Location:       firstLoc,
		SearchTerms:    searchTerms,
		Locations:      locations,
		ResultsWanted:  resultsWanted,
		Dedup:          true,
		DedupByCompany: false,
		MinScore:       0,
		EmailsOnly:     cfg.EmailOnly,
		LogLevel:       level,
		SiteSearch:     siteSearch,
		SiteLocation:   siteLocation,
		SiteLocations:  siteLocations,
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
		enc.SetIndent("", "  ")
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
	ac.Defaults.IsRemote = cfg.IsRemote
	ac.Defaults.RemoteOnly = cfg.RemoteOnly
	ac.Defaults.JobType = cfg.JobType

	b, err := yaml.Marshal(&ac)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError\033[0m marshalling config: %v\n", err)
		return
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, b, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError\033[0m writing %s: %v\n", cfgPath, err)
		return
	}
	fmt.Printf("  \033[38;5;117m✓\033[0m Saved to %s\n", cfgPath)

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
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError\033[0m writing %s: %v\n", envPath, err)
		return
	}
	fmt.Printf("\n  \033[38;5;117m✓\033[0m Saved to %s\n", envPath)
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
