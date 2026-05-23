package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
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

const ascii = `
  ███████╗ ██████╗██████╗  █████╗ ██████╗ ██████╗ ██╗   ██╗
  ██╔════╝██╔════╝██╔══██╗██╔══██╗██╔══██╗██╔══██╗╚██╗ ██╔╝
  ███████╗██║     ██████╔╝███████║██████╔╝██████╔╝ ╚████╔╝
  ╚════██║██║     ██╔══██╗██╔══██║██╔═══╝ ██╔═══╝   ╚██╔╝
  ███████║╚██████╗██║  ██║██║  ██║██║     ██║        ██║
  ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝        ╚═╝
`

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
}

type siteSearchTerms []string

func (s *siteSearchTerms) UnmarshalYAML(value *yaml.Node) error {
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
		return fmt.Errorf("site search must be string or list of strings")
	}
}

type siteTarget struct {
	Search   siteSearchTerms `yaml:"search"`
	Location string          `yaml:"location"`
}

type appConfig struct {
	Defaults struct {
		Search        string `yaml:"search"`
		Location      string `yaml:"location"`
		ResultsWanted int    `yaml:"results_wanted"`
		Out           string `yaml:"out"`
		Format        string `yaml:"format"`
	} `yaml:"defaults"`
	Sites map[string]siteTarget `yaml:"sites"`
}

var loadDotEnvOnce sync.Once

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
		Use:   "scrappy",
		Short: "Bulk job scraper",
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

	root.Flags().StringVar(&cfg.Search, "search", "", "search term")
	root.Flags().StringVar(&cfg.Location, "location", "", "search location")
	root.Flags().StringVar(&cfg.Sites, "sites", "", "comma-separated sites (empty = all supported)")
	root.Flags().IntVar(&cfg.ResultsWanted, "results-wanted", 0, "max results")
	root.Flags().StringVar(&cfg.Format, "format", "", "output format: jsonl|csv|xlsx|parquet")
	root.Flags().StringVar(&cfg.Out, "out", "", "output path")
	root.Flags().BoolVar(&cfg.Interactive, "interactive", true, "interactive wizard mode")
	root.Flags().BoolVar(&cfg.NonInteractive, "non-interactive", false, "disable interactive wizard")
	root.Flags().StringVar(&cfg.LogLevel, "log-level", "", "log level: DEBUG|INFO|WARN|ERROR|SYSTEM_ERROR|API_MISS")
	root.Flags().StringVar(&cfg.ConfigPath, "config", "config.yaml", "path to config yaml with per-site search/location")
	root.Flags().BoolVar(&cfg.EmailOnly, "email", false, "only include jobs with at least one email")
	root.Flags().IntVar(&cfg.Timeout, "timeout", 600, "scrape timeout in seconds")
	return root
}

func runInteractive(cfg *cliConfig) {
	fmt.Print(ascii)
	fmt.Println("Welcome to Scrappy interactive mode")
	reader := bufio.NewReader(os.Stdin)
	cfg.Search = ask(reader, "Search term", cfg.Search)
	cfg.Location = ask(reader, "Location", cfg.Location)
	cfg.Sites = ask(reader, "Sites (comma-separated)", cfg.Sites)
	cfg.ResultsWanted = askInt(reader, "Results wanted", cfg.ResultsWanted)
	cfg.Format = ask(reader, "Format (jsonl/csv/xlsx/parquet)", cfg.Format)
	cfg.Out = ask(reader, "Output path (empty for stdout)", cfg.Out)
}

func ask(r *bufio.Reader, label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
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

func runOnce(cfg *cliConfig) error {
	envPath := filepath.Join(filepath.Dir(cfg.ConfigPath), ".env")
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
	globalSearch := strings.TrimSpace(cfg.Search)
	if globalSearch == "" {
		globalSearch = strings.TrimSpace(ac.Defaults.Search)
	}
	globalLocation := strings.TrimSpace(cfg.Location)
	if globalLocation == "" {
		globalLocation = strings.TrimSpace(ac.Defaults.Location)
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
	siteSearch := map[model.Site][]string{}
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
			if v := strings.TrimSpace(t.Location); v != "" {
				siteLocation[s] = v
			}
		}
	}

	input := model.ScraperInput{
		Sites:          sites,
		SearchTerm:     globalSearch,
		Location:       globalLocation,
		ResultsWanted:  resultsWanted,
		Dedup:          true,
		DedupByCompany: false,
		MinScore:       0,
		EmailsOnly:     cfg.EmailOnly,
		LogLevel:       level,
		SiteSearch:     siteSearch,
		SiteLocation:   siteLocation,
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

	jobs, err := engine.Scrape(ctx, input)
	if err != nil {
		return err
	}

	if outPath == "" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jobs)
	}

	switch strings.ToLower(format) {
	case "csv":
		return export.WriteCSV(outPath, jobs)
	case "xlsx":
		return export.WriteXLSX(outPath, jobs)
	case "parquet":
		return export.WriteParquet(outPath, jobs)
	default:
		return export.WriteJSONL(outPath, jobs)
	}
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

func parseCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		x := strings.TrimSpace(p)
		if x != "" {
			out = append(out, x)
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
