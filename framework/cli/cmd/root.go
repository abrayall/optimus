package cmd

import (
	"fmt"
	"os"

	"optimus/core/lib/ui"

	"github.com/spf13/cobra"
)

// Version is set via ldflags during build
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "optimus [url]",
	Short: "Optimus — AI-powered SEO optimizer for any website",
	Long: ui.Banner() + "\n" +
		ui.VersionLine(Version) + "\n\n" +
		ui.Divider() + "\n\n" +
		"  Optimus scans a website, analyzes it for SEO issues using AI,\n" +
		"  and generates a detailed report with prioritized recommendations.\n\n" +
		"  Usage: optimus <url>",
	Args: cobra.MaximumNArgs(1),
	Run:  runOptimus,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}

func init() {
	rootCmd.Flags().StringVarP(&optimusName, "name", "n", "", "Site name (defaults to hostname)")
	rootCmd.Flags().IntVarP(&optimusCount, "count", "c", 1, "Number of pages to crawl")
	rootCmd.Flags().IntVar(&optimusDepth, "depth", 3, "Max link-follow depth for crawling")
	rootCmd.Flags().IntVarP(&optimusTimeout, "timeout", "t", 120, "Scraping timeout in seconds")
	rootCmd.Flags().BoolVar(&optimusSkipScrape, "skip-scrape", false, "Skip scraping (reuse existing)")
	rootCmd.Flags().BoolVar(&optimusSkipAnalyze, "skip-analyze", false, "Skip the analysis phase")
	rootCmd.Flags().StringVar(&optimusURL, "url", "", "URL to analyze (alternative to positional arg)")
	rootCmd.Flags().StringVarP(&optimusInstructions, "instructions", "i", "", "Custom instructions for the analysis")

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		ui.PrintError("%s", err)
		os.Exit(1)
	}
}
