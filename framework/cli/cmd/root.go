package cmd

import (
	"fmt"
	"os"

	"optimus/core/lib/mcp"
	"optimus/core/lib/ui"

	"github.com/spf13/cobra"
)

var mcpServe bool

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
	Run: func(cmd *cobra.Command, args []string) {
		if mcpServe {
			mcp.SetAPIKeys(optimusSerpAPIKey, optimusGoogleAPIKey, optimusGoogleCSEID, optimusGSCCreds, optimusPerplexityKey, optimusMozAPIKey, optimusAhrefsAPIKey, optimusBingAPIKey, optimusRedditClientID, optimusRedditClientSecret, optimusTwitterBearerToken)
			if err := mcp.RunServer(); err != nil {
				fmt.Fprintf(os.Stderr, "MCP server error: %s\n", err)
				os.Exit(1)
			}
			return
		}
		runOptimus(cmd, args)
	},
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
	rootCmd.Flags().StringVarP(&optimusSkill, "skill", "s", "seo", "Analysis skill to use (e.g. seo, aeo)")
	rootCmd.Flags().StringVar(&optimusPublish, "publish", "local", "Publish destination (local, s3)")
	rootCmd.Flags().StringVar(&optimusS3Bucket, "s3-bucket", "", "S3 bucket name for publishing")
	rootCmd.Flags().StringVar(&optimusS3Region, "s3-region", "us-east-1", "AWS region for S3 publishing")
	rootCmd.Flags().StringVar(&optimusS3Endpoint, "s3-endpoint", "", "Custom S3 endpoint URL (for Wasabi, MinIO, etc.)")

	rootCmd.Flags().BoolVar(&mcpServe, "mcp", false, "")
	rootCmd.Flags().MarkHidden("mcp")

	// External API key flags
	rootCmd.Flags().StringVar(&optimusSerpAPIKey, "serpapi-key", "", "SerpAPI API key for SERP position lookups")
	rootCmd.Flags().StringVar(&optimusGoogleAPIKey, "google-api-key", "", "Google API key for Custom Search")
	rootCmd.Flags().StringVar(&optimusGoogleCSEID, "google-cse-id", "", "Google Custom Search Engine ID")
	rootCmd.Flags().StringVar(&optimusGSCCreds, "gsc-credentials", "", "Path to Google Search Console service account JSON")
	rootCmd.Flags().StringVar(&optimusPerplexityKey, "perplexity-key", "", "Perplexity API key for AI citation checks")
	rootCmd.Flags().StringVar(&optimusMozAPIKey, "moz-api-key", "", "Moz API key for domain authority metrics")
	rootCmd.Flags().StringVar(&optimusAhrefsAPIKey, "ahrefs-api-key", "", "Ahrefs API key for domain rating and backlink data")
	rootCmd.Flags().StringVar(&optimusBingAPIKey, "bing-api-key", "", "Bing Webmaster Tools API key")
	rootCmd.Flags().StringVar(&optimusRedditClientID, "reddit-client-id", "", "Reddit OAuth client ID for brand mention search")
	rootCmd.Flags().StringVar(&optimusRedditClientSecret, "reddit-client-secret", "", "Reddit OAuth client secret for brand mention search")
	rootCmd.Flags().StringVar(&optimusTwitterBearerToken, "twitter-bearer-token", "", "Twitter/X API bearer token for brand mention search")

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
