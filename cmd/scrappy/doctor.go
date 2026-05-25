package main

import (
	"context"
	"fmt"
	"os"

	"github.com/arinbalyan/scrappy/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var (
		fixMode   bool
		verbose   bool
		configPath string
	)

	cmd := &cobra.Command{
		Use:     "doctor",
		Aliases: []string{"diagnose", "check", "health", "status"},
		Short:   "Diagnose and fix scrappy setup issues",
		Long: `Run diagnostics on your scrappy installation, configuration, and environment.

Checks performed:
  - Go runtime version
  - Binary integrity
  - Config file existence and YAML syntax
  - Config permissions (warns if world-readable)
  - Environment variables for API keys
  - DNS resolution and internet connectivity
  - Proxy reachability (if configured)
  - Docker availability
  - Playwright/Node.js fallback scripts
  - ~/.scrappy/ data directory

Use --fix to automatically repair fixable issues (permissions, missing dirs).

EXAMPLES:
  scrappy doctor
  scrappy doctor --fix
  scrappy doctor --verbose
  scrappy doctor --config /path/to/config.yaml
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load .env first so env vars are visible to checks
			if configPath == "" {
				configPath = defaultConfigPath()
			}
			envPath := defaultEnvPath(configPath)
			if _, err := os.Stat(envPath); err == nil {
				loadDotEnvOnce.Do(func() { loadDotEnv(envPath) })
			}

			ctx := context.Background()
			dc := doctor.DoctorConfig{
				ConfigPath: configPath,
				FixMode:    fixMode,
				Verbose:    verbose,
			}

			fmt.Print(ascii)
			fmt.Printf("\033[38;5;117mscrappy v%s \033[0m— doctor\n", version)

			report := doctor.Run(ctx, dc)

			if fixMode {
				fmt.Println()
				fmt.Println("  \033[36m⚡ Attempting auto-fixes...\033[0m")
				report.ExecuteFixes(ctx)
			}

			report.Print()

			if report.Failed > 0 {
				return fmt.Errorf("some checks failed — run 'scrappy doctor --fix' to attempt repairs")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config yaml (default: auto-detect)")
	cmd.Flags().BoolVarP(&fixMode, "fix", "f", false, "Automatically fix fixable issues (permissions, missing dirs)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output for all checks")

	return cmd
}
