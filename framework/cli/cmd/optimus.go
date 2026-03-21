package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"optimus/core/lib/engine"
	"optimus/core/lib/publisher"
	"optimus/core/lib/render"
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
	optimusSkill        string

	// Publishing
	optimusPublish    string
	optimusS3Bucket   string
	optimusS3Region   string
	optimusS3Endpoint string

	// External API keys
	optimusSerpAPIKey          string
	optimusGoogleAPIKey        string
	optimusGoogleCSEID         string
	optimusGSCCreds            string
	optimusPerplexityKey       string
	optimusMozAPIKey           string
	optimusAhrefsAPIKey        string
	optimusBingAPIKey          string
	optimusRedditClientID      string
	optimusRedditClientSecret  string
	optimusTwitterBearerToken  string
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
		if strings.HasPrefix(targetURL, "localhost") || strings.HasPrefix(targetURL, "127.0.0.1") {
			targetURL = "http://" + targetURL
		} else {
			targetURL = "https://" + targetURL
		}
	}

	// Validate skill exists before doing any work (skip for "all" mode)
	if optimusSkill != "all" {
		if _, err := engine.LoadSkill(optimusSkill); err != nil {
			available := engine.ListSkills()
			ui.PrintError("Skill %q not found. Available skills: all, %s", optimusSkill, strings.Join(available, ", "))
			os.Exit(1)
		}
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
	ui.PrintKeyValue("Skill", optimusSkill)
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

	// Phase 2: Run skill with AI
	if optimusSkipAnalyze {
		fmt.Print("  ")
		ui.PrintWarning("Skipping analysis")
		fmt.Println()
		return
	}

	cfg := engine.Config{
		SiteURL:            targetURL,
		ScrapedDir:         scrapedDir,
		Pages:              pages,
		OutputDir:          baseDir,
		Instructions:       optimusInstructions,
		Skill:              optimusSkill,
		SerpAPIKey:         optimusSerpAPIKey,
		GoogleAPIKey:       optimusGoogleAPIKey,
		GoogleCSEID:        optimusGoogleCSEID,
		GSCCredentials:     optimusGSCCreds,
		PerplexityKey:      optimusPerplexityKey,
		MozAPIKey:          optimusMozAPIKey,
		AhrefsAPIKey:       optimusAhrefsAPIKey,
		BingAPIKey:         optimusBingAPIKey,
		RedditClientID:     optimusRedditClientID,
		RedditClientSecret: optimusRedditClientSecret,
		TwitterBearerToken: optimusTwitterBearerToken,
	}

	pub := createPublisher(name)

	if optimusSkill == "all" {
		handleCombinedOutput(cfg, baseDir, targetURL, pub)
		return
	}

	fmt.Println(ui.Header("Phase 2: Running skill with AI"))
	fmt.Println()

	result, err := engine.Run(cfg)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Analysis failed: %s", err)
		os.Exit(1)
	}

	// Handle output based on skill type
	switch result.Skill.Output {
	case "scorecard":
		handleScorecardOutput(result, baseDir, pub)
	case "files":
		handleFilesOutput(result, baseDir, targetURL, pub)
	case "backlinks":
		handleBacklinksOutput(result, baseDir, pub)
	default:
		handleReportOutput(result, baseDir, pub)
	}

	// Print session ID for resuming
	if result.SessionID != "" {
		fmt.Println()
		ui.PrintKeyValue("Session", result.SessionID)
		ui.PrintInfo("Resume with: claude --resume %s", result.SessionID)
	}
}

// handleReportOutput parses a report from the raw output and generates HTML/JSON reports
func handleReportOutput(result *engine.Result, baseDir string, pub publisher.Publisher) {
	report, err := engine.ParseReport(result.RawOutput)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Failed to parse report: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Found %d recommendations", len(report.Recommendations))
	if report.Summary.CriticalCount > 0 {
		fmt.Print("    ")
		ui.PrintWarning("%d critical issues found!", report.Summary.CriticalCount)
	}
	if report.Summary.HighCount > 0 {
		fmt.Print("    ")
		ui.PrintInfo("%d high priority issues", report.Summary.HighCount)
	}
	fmt.Println()

	// Phase 3: Generate Reports
	fmt.Println(ui.Header("Phase 3: Generating reports"))
	fmt.Println()

	css, logoSVG := loadReportAssets()
	reportResult, err := render.Generate(render.Config{
		Report:    report,
		OutputDir: baseDir,
		CSS:       css,
		LogoSVG:   logoSVG,
		Version:   Version,
	})
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Report generation failed: %s", err)
		os.Exit(1)
	}

	pubResult, err := pub.Publish(reportResult.HTMLPath, reportResult.JSONPath)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Publishing failed: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Reports generated")
	fmt.Print("    ")
	ui.PrintKeyValue("JSON", pubResult.JSONURL)
	fmt.Print("    ")
	ui.PrintKeyValue("HTML", pubResult.HTMLURL)
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
	exec.Command("open", pubResult.HTMLURL).Start()
}

// handleFilesOutput parses file entries from the raw output, writes them to disk, and opens an HTML viewer
func handleFilesOutput(result *engine.Result, baseDir string, siteURL string, pub publisher.Publisher) {
	files, err := engine.ParseFiles(result.RawOutput)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Failed to parse files output: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Generated %d files", len(files))
	fmt.Println()

	// Phase 3: Write files and generate HTML viewer
	fmt.Println(ui.Header("Phase 3: Writing files"))
	fmt.Println()

	css, logoSVG := loadReportAssets()
	outputDir := filepath.Join(baseDir, "output")
	renderResult, err := render.GenerateFiles(render.FilesConfig{
		Files:     files,
		SiteURL:   siteURL,
		SkillName: result.Skill.Name,
		OutputDir: outputDir,
		CSS:       css,
		LogoSVG:   logoSVG,
		Version:   Version,
	})
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Failed to write files: %s", err)
		os.Exit(1)
	}

	pubResult, err := pub.Publish(renderResult.HTMLPath, "")
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Publishing failed: %s", err)
		os.Exit(1)
	}

	for _, f := range files {
		fmt.Print("    ")
		ui.PrintKeyValue("Wrote", filepath.Join(outputDir, f.Filename))
	}
	fmt.Print("    ")
	ui.PrintKeyValue("HTML", pubResult.HTMLURL)
	fmt.Println()

	// Print summary
	fmt.Println(ui.Divider())
	fmt.Println()
	ui.PrintSuccess("Optimus complete!")
	fmt.Println()
	fmt.Print("  ")
	ui.PrintKeyValue("Files", fmt.Sprintf("%d", len(files)))
	fmt.Print("  ")
	ui.PrintKeyValue("Output", outputDir)
	fmt.Println()

	// Open HTML viewer in browser
	fmt.Println()
	ui.PrintInfo("Opening in browser...")
	exec.Command("open", pubResult.HTMLURL).Start()
}

// handleBacklinksOutput parses a backlink strategy from raw output and generates HTML/JSON
func handleBacklinksOutput(result *engine.Result, baseDir string, pub publisher.Publisher) {
	strategy, err := engine.ParseBacklinks(result.RawOutput)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Failed to parse backlink strategy: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Found %d backlink opportunities", len(strategy.Opportunities))
	if strategy.Summary.QuickWins > 0 {
		fmt.Print("    ")
		ui.PrintInfo("%d quick wins identified", strategy.Summary.QuickWins)
	}
	fmt.Println()

	// Phase 3: Generate Reports
	fmt.Println(ui.Header("Phase 3: Generating backlink strategy"))
	fmt.Println()

	css, logoSVG := loadReportAssets()
	renderResult, err := render.GenerateBacklinks(render.BacklinksConfig{
		Strategy:  strategy,
		OutputDir: baseDir,
		CSS:       css,
		LogoSVG:   logoSVG,
		Version:   Version,
	})
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Report generation failed: %s", err)
		os.Exit(1)
	}

	pubResult, err := pub.Publish(renderResult.HTMLPath, renderResult.JSONPath)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Publishing failed: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Backlink strategy generated")
	fmt.Print("    ")
	ui.PrintKeyValue("JSON", pubResult.JSONURL)
	fmt.Print("    ")
	ui.PrintKeyValue("HTML", pubResult.HTMLURL)
	fmt.Println()

	// Print summary
	fmt.Println(ui.Divider())
	fmt.Println()
	ui.PrintSuccess("Optimus complete!")
	fmt.Println()
	printBacklinksSummary(strategy)

	// Open HTML report in browser
	fmt.Println()
	ui.PrintInfo("Opening report in browser...")
	exec.Command("open", pubResult.HTMLURL).Start()
}

// printBacklinksSummary displays a quick overview of backlink strategy findings
func printBacklinksSummary(bs *engine.BacklinkStrategy) {
	fmt.Println(ui.Header("Backlink Summary"))
	fmt.Println()
	if bs.Summary.CurrentDA > 0 {
		fmt.Print("  ")
		ui.PrintKeyValue("Domain Authority", fmt.Sprintf("%.0f", bs.Summary.CurrentDA))
	}
	if bs.Summary.CurrentDR > 0 {
		fmt.Print("  ")
		ui.PrintKeyValue("Domain Rating", fmt.Sprintf("%.0f", bs.Summary.CurrentDR))
	}
	if bs.Summary.ReferringDomains > 0 {
		fmt.Print("  ")
		ui.PrintKeyValue("Referring Domains", fmt.Sprintf("%d", bs.Summary.ReferringDomains))
	}
	fmt.Print("  ")
	ui.PrintKeyValue("Opportunities", fmt.Sprintf("%d", bs.Summary.TotalOpps))
	fmt.Print("  ")
	ui.PrintKeyValue("Quick Wins", fmt.Sprintf("%d", bs.Summary.QuickWins))
	fmt.Print("  ")
	ui.PrintKeyValue("High ROI", fmt.Sprintf("%d", bs.Summary.HighROI))
	fmt.Println()

	// Show top opportunities by strategy
	shown := 0
	for _, opp := range bs.Opportunities {
		if shown >= 5 {
			break
		}
		icon := "🔵"
		if opp.Difficulty == "easy" && opp.Impact == "high" {
			icon = "🟢"
		} else if opp.Impact == "high" {
			icon = "🟠"
		}
		fmt.Printf("    %s %s\n", icon, opp.Title)
		fmt.Printf("       %s · %s difficulty · %s impact\n", opp.Strategy, opp.Difficulty, opp.Impact)
		shown++
	}
	if shown > 0 {
		fmt.Println()
	}
	fmt.Print("  ")
	ui.PrintInfo("Open the HTML report for full details")
}

// handleScorecardOutput parses a scorecard from raw output and generates HTML/JSON
func handleScorecardOutput(result *engine.Result, baseDir string, pub publisher.Publisher) {
	scorecard, err := engine.ParseScorecard(result.RawOutput)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Failed to parse scorecard: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Scorecard complete — overall score: %d/100", scorecard.OverallScore)
	fmt.Println()

	// Phase 3: Generate Scorecard
	fmt.Println(ui.Header("Phase 3: Generating scorecard"))
	fmt.Println()

	css, logoSVG := loadReportAssets()
	renderResult, err := render.GenerateScorecard(render.ScorecardConfig{
		Scorecard: scorecard,
		OutputDir: baseDir,
		CSS:       css,
		LogoSVG:   logoSVG,
		Version:   Version,
	})
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Scorecard generation failed: %s", err)
		os.Exit(1)
	}

	pubResult, err := pub.Publish(renderResult.HTMLPath, renderResult.JSONPath)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Publishing failed: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Scorecard generated")
	fmt.Print("    ")
	ui.PrintKeyValue("JSON", pubResult.JSONURL)
	fmt.Print("    ")
	ui.PrintKeyValue("HTML", pubResult.HTMLURL)
	fmt.Println()

	// Print summary
	fmt.Println(ui.Divider())
	fmt.Println()
	ui.PrintSuccess("Optimus complete!")
	fmt.Println()
	printScorecardSummary(scorecard)

	// Open HTML in browser
	fmt.Println()
	ui.PrintInfo("Opening scorecard in browser...")
	exec.Command("open", pubResult.HTMLURL).Start()
}

// printScorecardSummary displays a quick overview of scorecard results
func printScorecardSummary(sc *engine.Scorecard) {
	fmt.Println(ui.Header("Score Summary"))
	fmt.Println()
	fmt.Print("  ")
	ui.PrintKeyValue("Overall", fmt.Sprintf("%d/100", sc.OverallScore))
	fmt.Print("  ")
	ui.PrintKeyValue("Search Rank", fmt.Sprintf("%d/100", sc.CategoryScores.SearchRank))
	fmt.Print("  ")
	ui.PrintKeyValue("Answer Rank", fmt.Sprintf("%d/100", sc.CategoryScores.AnswerRank))
	fmt.Print("  ")
	ui.PrintKeyValue("Technical", fmt.Sprintf("%d/100", sc.CategoryScores.Technical))
	fmt.Print("  ")
	ui.PrintKeyValue("Content", fmt.Sprintf("%d/100", sc.CategoryScores.Content))
	fmt.Print("  ")
	ui.PrintKeyValue("Structure", fmt.Sprintf("%d/100", sc.CategoryScores.Structure))
	fmt.Println()

	if sc.DomainAuth != nil {
		fmt.Println(ui.Header("Domain Authority"))
		fmt.Println()
		if sc.DomainAuth.MozDA > 0 {
			fmt.Print("  ")
			ui.PrintKeyValue("Moz DA", fmt.Sprintf("%.0f/100", sc.DomainAuth.MozDA))
			fmt.Print("  ")
			ui.PrintKeyValue("Moz PA", fmt.Sprintf("%.0f/100", sc.DomainAuth.MozPA))
			fmt.Print("  ")
			ui.PrintKeyValue("Spam Score", fmt.Sprintf("%.0f%%", sc.DomainAuth.MozSpamScore))
			fmt.Print("  ")
			ui.PrintKeyValue("Linking Domains", fmt.Sprintf("%d", sc.DomainAuth.LinkingRootDomains))
		}
		if sc.DomainAuth.AhrefsDR > 0 {
			fmt.Print("  ")
			ui.PrintKeyValue("Ahrefs DR", fmt.Sprintf("%.0f/100", sc.DomainAuth.AhrefsDR))
			fmt.Print("  ")
			ui.PrintKeyValue("Ahrefs Rank", fmt.Sprintf("%d", sc.DomainAuth.AhrefsRank))
		}
		fmt.Println()
	}

	if sc.BacklinkProfile != nil {
		fmt.Println(ui.Header("Backlink Profile"))
		fmt.Println()
		fmt.Print("  ")
		ui.PrintKeyValue("Live Backlinks", fmt.Sprintf("%d", sc.BacklinkProfile.LiveBacklinks))
		fmt.Print("  ")
		ui.PrintKeyValue("Referring Domains", fmt.Sprintf("%d", sc.BacklinkProfile.ReferringDomains))
		fmt.Print("  ")
		ui.PrintKeyValue("Referring Pages", fmt.Sprintf("%d", sc.BacklinkProfile.ReferringPages))
		fmt.Println()
	}

	if len(sc.SerpPositions) > 0 {
		fmt.Println(ui.Header("Search Positions"))
		fmt.Println()
		for _, sp := range sc.SerpPositions {
			if sp.DomainFound {
				fmt.Printf("    %s: #%d\n", sp.Keyword, sp.Position)
			} else {
				fmt.Printf("    %s: %s\n", sp.Keyword, ui.Muted("not found"))
			}
		}
		fmt.Println()
	}

	if len(sc.Findings) > 0 {
		fmt.Println(ui.Header("Key Findings"))
		fmt.Println()
		for _, f := range sc.Findings {
			fmt.Printf("    %s %s\n", ui.Highlight("•"), f)
		}
		fmt.Println()
	}

	fmt.Print("  ")
	ui.PrintInfo("Open the HTML scorecard for full details")
}

// printSummary displays a quick overview of findings
func printSummary(report *engine.Report) {
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

// loadReportAssets reads the marketing CSS and logo SVG for embedding in reports.
// Returns empty strings if the files are not found (templates use CSS variable fallbacks).
func loadReportAssets() (css, logoSVG string) {
	paths := []string{
		"framework/server/www/static",
		"../server/www/static",
	}
	for _, base := range paths {
		cssData, err1 := os.ReadFile(filepath.Join(base, "css", "style.css"))
		svgData, err2 := os.ReadFile(filepath.Join(base, "images", "logo.svg"))
		if err1 == nil && err2 == nil {
			return string(cssData), string(svgData)
		}
	}
	return "", ""
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

// createPublisher returns a Publisher based on the --publish flag
func createPublisher(siteName string) publisher.Publisher {
	switch optimusPublish {
	case "s3":
		if optimusS3Bucket == "" {
			ui.PrintError("--s3-bucket is required when using --publish s3")
			os.Exit(1)
		}
		pub, err := publisher.NewS3(optimusS3Bucket, optimusS3Region, optimusS3Endpoint, siteName)
		if err != nil {
			ui.PrintError("Failed to create S3 publisher: %s", err)
			os.Exit(1)
		}
		return pub
	default:
		return publisher.NewLocal()
	}
}

// handleCombinedOutput runs all skills and generates a combined tabbed report
func handleCombinedOutput(cfg engine.Config, baseDir string, siteURL string, pub publisher.Publisher) {
	skills := engine.AnalysisSkills()
	fmt.Println(ui.Header(fmt.Sprintf("Phase 2: Running %d skills with AI", len(skills))))
	fmt.Println()

	fullResult, err := engine.RunAll(cfg)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Analysis failed: %s", err)
		os.Exit(1)
	}

	// Report per-skill status
	succeeded := 0
	failed := 0
	for _, sr := range fullResult.Skills {
		name := ""
		if sr.Skill != nil {
			name = sr.Skill.Name
		}
		if sr.Error != "" {
			failed++
			fmt.Print("  ")
			ui.PrintWarning("%s: failed (%s)", name, sr.Error)
		} else {
			succeeded++
			fmt.Print("  ")
			ui.PrintSuccess("%s: complete", name)
		}
	}
	fmt.Println()
	fmt.Print("  ")
	ui.PrintInfo("%d succeeded, %d failed", succeeded, failed)
	fmt.Println()

	// Phase 3: Parse results and generate combined report
	fmt.Println(ui.Header("Phase 3: Generating combined report"))
	fmt.Println()

	css, logoSVG := loadReportAssets()
	combinedCfg := render.CombinedConfig{
		SiteURL:   siteURL,
		Reports:   make(map[string]*engine.Report),
		Files:     make(map[string][]engine.FileEntry),
		Errors:    make(map[string]string),
		OutputDir: baseDir,
		CSS:       css,
		LogoSVG:   logoSVG,
		Version:   Version,
	}

	for _, sr := range fullResult.Skills {
		if sr.Skill == nil {
			continue
		}
		skillKey := cfg.Skill // not useful here, derive from skill
		// Derive skill key from the skill output type and name
		for _, name := range engine.AnalysisSkills() {
			s, _ := engine.LoadSkill(name)
			if s != nil && s.Name == sr.Skill.Name {
				skillKey = name
				break
			}
		}

		if sr.Error != "" {
			combinedCfg.Errors[skillKey] = sr.Error
			continue
		}

		switch sr.Skill.Output {
		case "scorecard":
			scorecard, err := engine.ParseScorecard(sr.RawOutput)
			if err != nil {
				combinedCfg.Errors[skillKey] = fmt.Sprintf("parse error: %s", err)
			} else {
				combinedCfg.Scorecard = scorecard
			}
		case "report":
			report, err := engine.ParseReport(sr.RawOutput)
			if err != nil {
				combinedCfg.Errors[skillKey] = fmt.Sprintf("parse error: %s", err)
			} else {
				combinedCfg.Reports[skillKey] = report
			}
		case "backlinks":
			strategy, err := engine.ParseBacklinks(sr.RawOutput)
			if err != nil {
				combinedCfg.Errors[skillKey] = fmt.Sprintf("parse error: %s", err)
			} else {
				combinedCfg.Backlinks = strategy
			}
		case "files":
			files, err := engine.ParseFiles(sr.RawOutput)
			if err != nil {
				combinedCfg.Errors[skillKey] = fmt.Sprintf("parse error: %s", err)
			} else {
				combinedCfg.Files[skillKey] = files
			}
		}
	}

	renderResult, err := render.GenerateCombined(combinedCfg)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Combined report generation failed: %s", err)
		os.Exit(1)
	}

	pubResult, err := pub.Publish(renderResult.HTMLPath, renderResult.JSONPath)
	if err != nil {
		fmt.Print("  ")
		ui.PrintError("Publishing failed: %s", err)
		os.Exit(1)
	}

	fmt.Print("  ")
	ui.PrintSuccess("Combined report generated")
	fmt.Print("    ")
	ui.PrintKeyValue("JSON", pubResult.JSONURL)
	fmt.Print("    ")
	ui.PrintKeyValue("HTML", pubResult.HTMLURL)
	fmt.Println()

	// Print summary
	fmt.Println(ui.Divider())
	fmt.Println()
	ui.PrintSuccess("Optimus complete!")
	fmt.Println()

	if combinedCfg.Scorecard != nil {
		printScorecardSummary(combinedCfg.Scorecard)
	}

	// Open HTML report in browser
	fmt.Println()
	ui.PrintInfo("Opening report in browser...")
	exec.Command("open", pubResult.HTMLURL).Start()
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
