package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"optimus/core/lib/analyzer"
	"optimus/core/lib/reporter"
	"optimus/core/lib/scraper"
	"optimus/core/lib/ui"

	"github.com/spf13/cobra"
)

var (
	optimusName         string
	optimusCount        int
	optimusDepth        int
	optimusSkipScrape   bool
	optimusSkipAnalyze  bool
	optimusTimeout      int
	optimusURL          string
	optimusInstructions string
)

func runOptimus(cmd *cobra.Command, args []string) {
	// Determine URL
	targetURL := optimusURL
	if len(args) > 0 {
		targetURL = args[0]
	}
	if targetURL == "" {
		cmd.Help()
		return
	}

	// Normalize URL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	// Derive site name
	name := optimusName
	if name == "" {
		name = deriveNameFromURL(targetURL)
	}

	// Set up working directory in system temp
	baseDir := filepath.Join(os.TempDir(), "optimus", "work", name)
	scrapedDir := filepath.Join(baseDir, "scraped")

	// Print header
	ui.PrintHeader(Version)
	ui.PrintKeyValue("URL", targetURL)
	ui.PrintKeyValue("Name", name)
	ui.PrintKeyValue("Pages", fmt.Sprintf("%d", optimusCount))
	ui.PrintKeyValue("Directory", baseDir)
	fmt.Println()

	// Back up previous run if not skipping anything
	if !optimusSkipScrape && !optimusSkipAnalyze {
		if _, err := os.Stat(baseDir); err == nil {
			backupDir := findBackupName(baseDir)
			ui.PrintInfo("Backing up previous run to %s", backupDir)
			if err := os.Rename(baseDir, backupDir); err != nil {
				ui.PrintWarning("Could not backup: %s", err)
			}
			fmt.Println()
		}
	}

	// Ensure base dir exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		ui.PrintError("Failed to create working directory: %s", err)
		os.Exit(1)
	}

	// Phase 1: Scrape
	var pages []scraper.PageResult
	if !optimusSkipScrape {
		fmt.Println(ui.Header("Phase 1: Scraping website"))
		fmt.Println()

		scrapeResult, err := scraper.Scrape(scraper.Config{
			URL:       targetURL,
			OutputDir: scrapedDir,
			Timeout:   time.Duration(optimusTimeout) * time.Second,
			MaxPages:  optimusCount,
			MaxDepth:  optimusDepth,
		})
		if err != nil {
			ui.PrintError("Scraping failed: %s", err)
			os.Exit(1)
		}

		pages = scrapeResult.Pages
		fmt.Print("  ")
		ui.PrintSuccess("Scraped %d pages", len(pages))
		fmt.Print("  ")
		ui.PrintKeyValue("Output", scrapeResult.OutputDir)
		fmt.Println()
	} else {
		fmt.Print("  ")
		ui.PrintWarning("Skipping scrape (reusing existing)")
		// Load existing scraped pages
		pages = loadExistingPages(scrapedDir)
		if len(pages) == 0 {
			fmt.Print("  ")
			ui.PrintError("No scraped pages found in %s", scrapedDir)
			os.Exit(1)
		}
		fmt.Print("  ")
		ui.PrintInfo("Found %d existing pages", len(pages))
		fmt.Println()
	}

	// Phase 2: Analyze with AI
	var report *analyzer.Report
	if !optimusSkipAnalyze {
		fmt.Println(ui.Header("Phase 2: Analyzing SEO with AI"))
		fmt.Println()

		analyzeResult, err := analyzer.Analyze(analyzer.Config{
			SiteURL:      targetURL,
			ScrapedDir:   scrapedDir,
			Pages:        pages,
			OutputDir:    baseDir,
			Instructions: optimusInstructions,
		})
		if err != nil {
			fmt.Print("  ")
			ui.PrintError("Analysis failed: %s", err)
			os.Exit(1)
		}

		report = analyzeResult.Report
		fmt.Print("  ")
		ui.PrintSuccess("Found %d SEO recommendations", len(report.Recommendations))
		if report.Summary.CriticalCount > 0 {
			fmt.Print("    ")
			ui.PrintWarning("%d critical issues found!", report.Summary.CriticalCount)
		}
		if report.Summary.HighCount > 0 {
			fmt.Print("    ")
			ui.PrintInfo("%d high priority issues", report.Summary.HighCount)
		}
		fmt.Println()
	} else {
		fmt.Print("  ")
		ui.PrintWarning("Skipping analysis")
		fmt.Println()
		return
	}

	// Phase 3: Generate Reports
	fmt.Println(ui.Header("Phase 3: Generating reports"))
	fmt.Println()

	reportResult, err := reporter.Generate(reporter.Config{
		Report:    report,
		OutputDir: baseDir,
	})
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Report generation failed: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Reports generated")
	fmt.Print("    ")
	ui.PrintKeyValue("JSON", reportResult.JSONPath)
	fmt.Print("    ")
	ui.PrintKeyValue("HTML", reportResult.HTMLPath)
	fmt.Println()

	// Print summary
	fmt.Println(ui.Divider())
	fmt.Println()
	ui.PrintSuccess("Optimus complete!")
	fmt.Println()
	printSummary(report)

	// Open HTML report in browser
	fmt.Println()
	ui.PrintInfo("Opening report in browser...")
	exec.Command("open", reportResult.HTMLPath).Start()
}

// printSummary displays a quick overview of findings
func printSummary(report *analyzer.Report) {
	fmt.Println(ui.Header("Summary"))
	fmt.Println()
	fmt.Print("  ")
	ui.PrintKeyValue("Total Issues", fmt.Sprintf("%d", report.Summary.TotalIssues))
	if report.Summary.CriticalCount > 0 {
		fmt.Print("  ")
		ui.PrintKeyValue("Critical", fmt.Sprintf("%d", report.Summary.CriticalCount))
	}
	if report.Summary.HighCount > 0 {
		fmt.Print("  ")
		ui.PrintKeyValue("High", fmt.Sprintf("%d", report.Summary.HighCount))
	}
	if report.Summary.MediumCount > 0 {
		fmt.Print("  ")
		ui.PrintKeyValue("Medium", fmt.Sprintf("%d", report.Summary.MediumCount))
	}
	if report.Summary.LowCount > 0 {
		fmt.Print("  ")
		ui.PrintKeyValue("Low", fmt.Sprintf("%d", report.Summary.LowCount))
	}
	fmt.Println()

	// Show top 5 critical/high issues
	shown := 0
	for _, rec := range report.Recommendations {
		if shown >= 5 {
			break
		}
		if rec.Priority == "critical" || rec.Priority == "high" {
			icon := "🔴"
			if rec.Priority == "high" {
				icon = "🟠"
			}
			fmt.Printf("    %s %s\n", icon, rec.Issue)
			fmt.Printf("       %s\n", ui.Muted(rec.URL))
			shown++
		}
	}
	if shown > 0 {
		fmt.Println()
	}
	fmt.Print("  ")
	ui.PrintInfo("Open the HTML report for full details")
}

// findBackupName returns the next available backup name
func findBackupName(baseDir string) string {
	for i := 1; ; i++ {
		backup := fmt.Sprintf("%s.bak%d", baseDir, i)
		if _, err := os.Stat(backup); os.IsNotExist(err) {
			return backup
		}
	}
}

// deriveNameFromURL extracts a site name from a URL
func deriveNameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "site"
	}

	hostname := parsed.Hostname()
	hostname = strings.TrimPrefix(hostname, "www.")
	// Join all parts with dashes: hub.responsiveworks.com -> hub-responsiveworks-com
	return strings.ReplaceAll(hostname, ".", "-")
}

// loadExistingPages loads previously scraped page files
func loadExistingPages(scrapedDir string) []scraper.PageResult {
	var pages []scraper.PageResult

	entries, err := os.ReadDir(scrapedDir)
	if err != nil {
		return pages
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		filePath := filepath.Join(scrapedDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		pages = append(pages, scraper.PageResult{
			FilePath:  filePath,
			CleanHTML: string(content),
			Title:     entry.Name(),
		})
	}

	return pages
}
