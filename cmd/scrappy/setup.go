package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newSetupCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "setup",
		Aliases: []string{"init", "onboard", "wizard"},
		Short:   "Run the interactive setup wizard to create your config",
		Long: `Walk through an interactive setup to create ~/.scrappy/config.yaml
and ~/.scrappy/.env with your API keys, proxies, and preferences.

This is the same wizard that runs when you start scrappy without arguments,
exposed as a standalone command for easy re-configuration.

EXAMPLES:
  scrappy setup
  scrappy setup --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(ascii)
			fmt.Printf("\033[38;5;117mscrappy v%s \033[0m— setup wizard\n", version)
			fmt.Println(strings.Repeat("─", 50))
			fmt.Println()
			fmt.Println("  This wizard will help you create your scrappy configuration.")
			fmt.Println("  You can stop at any time with Ctrl+C — nothing is saved until")
			fmt.Println("  you confirm at the end.")
			fmt.Println()

			reader := bufio.NewReader(os.Stdin)

			// 1. Collect settings
			fmt.Println(" \033[38;5;117m╭─ Search Settings ───────────────────────────────╮\033[0m")
			search := ask(reader, "  Default search term (e.g. \"AI Engineer\")", "")
			location := ask(reader, "  Default location (e.g. \"Remote\" or \"San Francisco\")", "")
			sites := ask(reader, "  Sites to scrape (comma-separated, empty=all)", "")
			resultsWanted := askInt(reader, "  Results wanted (0 = unlimited)", 0)
			fmt.Println(" \033[38;5;117m╰──────────────────────────────────────────────────╯\033[0m")
			fmt.Println()

			fmt.Println(" \033[38;5;117m╭─ Output Settings ─────────────────────────────────╮\033[0m")
			format := ask(reader, "  Output format (jsonl/csv/xlsx/parquet)", "jsonl")
			outPath := ask(reader, "  Output path (empty = stdout)", "")
			fmt.Println(" \033[38;5;117m╰──────────────────────────────────────────────────╯\033[0m")
			fmt.Println()

			fmt.Println(" \033[38;5;117m╭─ Filters ────────────────────────────────────────╮\033[0m")
			isRemote := askBool(reader, "  Only remote jobs? (y/n)", "n")
			jobType := ask(reader, "  Job type (fulltime/parttime/contract/any)", "any")
			if jobType == "any" {
				jobType = ""
			}
			memCap := ask(reader, "  Memory cap (e.g. 512MB, 1GB, 0=unlimited)", "0")
			fmt.Println(" \033[38;5;117m╰──────────────────────────────────────────────────╯\033[0m")
			fmt.Println()

			fmt.Println(" \033[38;5;117m╭─ Network ────────────────────────────────────────╮\033[0m")
			proxy := ask(reader, "  Proxy (socks5://user:pass@host:port)", "")
			fmt.Println(" \033[38;5;117m╰──────────────────────────────────────────────────╯\033[0m")
			fmt.Println()

			fmt.Println(" \033[38;5;117m╭─ API Keys ───────────────────────────────────────╮\033[0m")
			fmt.Println("  \033[90mThese are optional — only needed for Adzuna, Careerjet,\033[0m")
			fmt.Println("  \033[90mInfojobs, Findwork, and Arbeitsagentur.\033[0m")
			fmt.Println("  \033[90mSkip any by pressing Enter.\033[0m")
			fmt.Println()

			adzunaID := ask(reader, "  ADZUNA_APP_ID", "")
			adzunaKey := ask(reader, "  ADZUNA_APP_KEY", "")
			careerjetID := ask(reader, "  CAREERJET_AFFID", "")
			findworkKey := ask(reader, "  FINDWORK_API_KEY", "")
			fmt.Println(" \033[38;5;117m╰──────────────────────────────────────────────────╯\033[0m")
			fmt.Println()

			// 2. Confirm
			fmt.Println(" \033[38;5;117m╭─ Summary ─────────────────────────────────────────╮\033[0m")
			fmt.Printf("  \033[38;5;117m│\033[0m Search:     %s\n", ifEmpty(search, "(none)"))
			fmt.Printf("  \033[38;5;117m│\033[0m Location:   %s\n", ifEmpty(location, "(none)"))
			fmt.Printf("  \033[38;5;117m│\033[0m Sites:      %s\n", ifEmpty(sites, "(all 55+)"))
			fmt.Printf("  \033[38;5;117m│\033[0m Format:     %s\n", format)
			fmt.Printf("  \033[38;5;117m│\033[0m Proxy:      %s\n", ifEmpty(proxy, "(none)"))
			fmt.Printf("  \033[38;5;117m│\033[0m API keys:   %s\n", apiKeySummary(adzunaID, adzunaKey, careerjetID, findworkKey))
			fmt.Println(" \033[38;5;117m╰──────────────────────────────────────────────────╯\033[0m")
			fmt.Println()

			save := askBool(reader, "  Save this configuration? (y/N)", "n")
			if !save {
				fmt.Println("  \033[33m✗\033[0m Setup cancelled. Nothing was saved.")
				return nil
			}

			// 3. Create dirs and write files
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("cannot determine home dir: %w", err)
			}
			scrappyDir := filepath.Join(home, ".scrappy")
			if err := os.MkdirAll(scrappyDir, 0700); err != nil {
				return fmt.Errorf("create %s: %w", scrappyDir, err)
			}

			// Write config.yaml
			type siteCfg struct {
				Search   []string `yaml:"search,omitempty"`
				Location []string `yaml:"location,omitempty"`
			}
			type configDefaults struct {
				Search        []string `yaml:"search,omitempty"`
				Location      []string `yaml:"location,omitempty"`
				ResultsWanted int      `yaml:"results_wanted,omitempty"`
				Out           string   `yaml:"out,omitempty"`
				Format        string   `yaml:"format,omitempty"`
				MemoryCap     string   `yaml:"memory_cap,omitempty"`
				IsRemote      bool     `yaml:"is_remote,omitempty"`
				JobType       string   `yaml:"job_type,omitempty"`
			}
			cfgYAML := struct {
				Defaults configDefaults       `yaml:"defaults"`
				Proxy    string               `yaml:"proxy,omitempty"`
				Sites    map[string]siteCfg   `yaml:"sites,omitempty"`
			}{
				Defaults: configDefaults{
					ResultsWanted: resultsWanted,
					Out:           outPath,
					Format:        format,
					MemoryCap:     memCap,
					IsRemote:      isRemote,
					JobType:       jobType,
				},
				Proxy: proxy,
			}
			if search != "" {
				cfgYAML.Defaults.Search = []string{search}
			}
			if location != "" {
				cfgYAML.Defaults.Location = []string{location}
			}

			cfgBytes, err := yaml.Marshal(&cfgYAML)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			cfgPath := filepath.Join(scrappyDir, "config.yaml")
			if err := os.WriteFile(cfgPath, cfgBytes, 0600); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			fmt.Printf("  \033[32m✓\033[0m Created %s\n", cfgPath)

			// Write .env if any API keys provided
			var envLines []string
			if adzunaID != "" {
				envLines = append(envLines, "ADZUNA_APP_ID="+adzunaID)
			}
			if adzunaKey != "" {
				envLines = append(envLines, "ADZUNA_APP_KEY="+adzunaKey)
			}
			if careerjetID != "" {
				envLines = append(envLines, "CAREERJET_AFFID="+careerjetID)
			}
			if findworkKey != "" {
				envLines = append(envLines, "FINDWORK_API_KEY="+findworkKey)
			}
			if len(envLines) > 0 {
				envPath := filepath.Join(scrappyDir, ".env")
				if err := os.WriteFile(envPath, []byte(strings.Join(envLines, "\n")+"\n"), 0600); err != nil {
					return fmt.Errorf("write env: %w", err)
				}
				fmt.Printf("  \033[32m✓\033[0m Created %s (%d keys)\n", envPath, len(envLines))
			}

			fmt.Println()
			fmt.Println("  \033[32m╭──────────────────────────────────────────────────╮\033[0m")
			fmt.Println("  \033[32m│\033[0m  Setup complete!                              \033[32m│\033[0m")
			fmt.Println("  \033[32m│\033[0m  Run 'scrappy doctor' to verify everything.   \033[32m│\033[0m")
			fmt.Println("  \033[32m│\033[0m  Run 'scrappy --help' to see all options.     \033[32m│\033[0m")
			fmt.Println("  \033[32m╰──────────────────────────────────────────────────╯\033[0m")
			fmt.Println()
			return nil
		},
	}
}

func ifEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func apiKeySummary(adzunaID, adzunaKey, careerjetID, findworkKey string) string {
	count := 0
	if adzunaID != "" && adzunaKey != "" {
		count++
	}
	if careerjetID != "" {
		count++
	}
	if findworkKey != "" {
		count++
	}
	if count > 0 {
		return fmt.Sprintf("%d site(s) configured", count)
	}
	return "(none)"
}
