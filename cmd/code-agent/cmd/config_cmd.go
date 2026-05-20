package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/lazrossi/code-agent/internal/config"
	"github.com/spf13/cobra"
)

var discoverFlag bool
var discoverDir string

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or discover configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if discoverFlag {
			return runDiscover()
		}
		return showConfig()
	},
}

func showConfig() error {
	fmt.Printf("Config file: %s\n\n", cfgFile)
	fmt.Printf("[github]\n")
	fmt.Printf("  org           = %s\n", cfg.GitHub.Org)
	fmt.Printf("  poll_interval = %s\n", cfg.GitHub.PollInterval.Duration)
	fmt.Printf("  require_label = %s\n", cfg.GitHub.RequireLabel)
	fmt.Printf("  skip_labels   = %v\n", cfg.GitHub.SkipLabels)

	fmt.Printf("\n[agent]\n")
	fmt.Printf("  model       = %s\n", cfg.Agent.Model)
	fmt.Printf("  max_turns   = %d\n", cfg.Agent.MaxTurns)
	fmt.Printf("  timeout     = %s\n", cfg.Agent.Timeout.Duration)
	fmt.Printf("  concurrency = %d\n", cfg.Agent.Concurrency)

	fmt.Printf("\n[database]\n")
	fmt.Printf("  path = %s\n", cfg.DBPath())

	fmt.Printf("\n[http]\n")
	fmt.Printf("  addr = %s\n", cfg.HTTP.Addr)

	if len(cfg.Repos) > 0 {
		fmt.Printf("\nRepos:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tPATH\tENABLED")
		for name, repo := range cfg.Repos {
			fmt.Fprintf(w, "  %s\t%s\t%v\n", name, repo.Path, repo.Enabled)
		}
		w.Flush()
	} else {
		fmt.Println("\nNo repos configured. Use --discover to scan for repos.")
	}
	return nil
}

func runDiscover() error {
	dir := discoverDir
	if dir == "" {
		wd, _ := os.Getwd()
		dir = wd
	}

	fmt.Printf("Scanning %s for %s repos...\n\n", dir, cfg.GitHub.Org)
	repos := config.DiscoverRepos(dir, cfg.GitHub.Org)

	if len(repos) == 0 {
		fmt.Println("No repos found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH")
	for name, repo := range repos {
		fmt.Fprintf(w, "%s\t%s\n", name, repo.Path)
	}
	w.Flush()

	fmt.Println("\nAdd these to your config.toml under [repos.*] sections.")
	return nil
}

func init() {
	configCmd.Flags().BoolVar(&discoverFlag, "discover", false, "Scan for repos matching the configured org")
	configCmd.Flags().StringVar(&discoverDir, "dir", "", "Directory to scan (default: current directory)")
	rootCmd.AddCommand(configCmd)
}
