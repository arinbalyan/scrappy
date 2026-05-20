package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/export"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
	"github.com/arinbalyan/scrappy/pkg/scrappy"
	"github.com/spf13/cobra"
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
	Interactive    bool
	NonInteractive bool
	WorkableSeeds  string
	WorkdaySeeds   string
	LogLevel       string
}

func main() {
	cfg := &cliConfig{}
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

	root.Flags().StringVar(&cfg.Search, "search", "software engineer", "search term")
	root.Flags().StringVar(&cfg.Location, "location", "Remote", "search location")
	root.Flags().StringVar(&cfg.Sites, "sites", "linkedin,indeed", "comma-separated sites")
	root.Flags().IntVar(&cfg.ResultsWanted, "results-wanted", 15, "max results")
	root.Flags().StringVar(&cfg.Format, "format", "jsonl", "output format: jsonl|csv|xlsx|parquet")
	root.Flags().StringVar(&cfg.Out, "out", "", "output path")
	root.Flags().BoolVar(&cfg.Interactive, "interactive", true, "interactive wizard mode")
	root.Flags().BoolVar(&cfg.NonInteractive, "non-interactive", false, "disable interactive wizard")
	root.Flags().StringVar(&cfg.WorkableSeeds, "workable-seeds", "", "comma-separated Workable account/company seeds")
	root.Flags().StringVar(&cfg.WorkdaySeeds, "workday-seeds", "", "comma-separated Workday CXS endpoint seeds")
	root.Flags().StringVar(&cfg.LogLevel, "log-level", "", "log level: DEBUG|INFO|WARN|ERROR|SYSTEM_ERROR|API_MISS")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
	level := strings.TrimSpace(cfg.LogLevel)
	if level == "" {
		level = strings.TrimSpace(os.Getenv("SCRAPPY_LOG_LEVEL"))
	}
	util.SetLogLevel(level)

	sites := parseSites(cfg.Sites)
	input := model.ScraperInput{
		Sites:          sites,
		SearchTerm:     cfg.Search,
		Location:       cfg.Location,
		ResultsWanted:  cfg.ResultsWanted,
		Dedup:          true,
		DedupByCompany: false,
		MinScore:       0,
		WorkableSeeds:  parseCSV(cfg.WorkableSeeds),
		WorkdaySeeds:   parseCSV(cfg.WorkdaySeeds),
		LogLevel:       level,
	}

	constraints := scrappy.EvaluateConstraints(input)
	for _, w := range constraints.Warnings {
		fmt.Printf("[constraint-warning] %s\n", w)
	}
	if len(constraints.Errors) > 0 {
		return fmt.Errorf("constraint errors: %v", constraints.Errors)
	}

	engine := scrappy.NewEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	jobs, err := engine.Scrape(ctx, input)
	if err != nil {
		return err
	}

	if cfg.Out == "" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jobs)
	}

	switch strings.ToLower(cfg.Format) {
	case "csv":
		return export.WriteCSV(cfg.Out, jobs)
	case "xlsx":
		return export.WriteXLSX(cfg.Out, jobs)
	case "parquet":
		return export.WriteParquet(cfg.Out, jobs)
	default:
		return export.WriteJSONL(cfg.Out, jobs)
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
