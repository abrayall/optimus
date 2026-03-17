package analyzer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"optimus/core/lib/scraper"
	"optimus/core/lib/ui"
)

// Recommendation represents a single SEO optimization recommendation
type Recommendation struct {
	Priority    string   `json:"priority"`    // critical, high, medium, low
	Category    string   `json:"category"`    // title, meta, headings, content, images, links, performance, structured-data, accessibility
	URL         string   `json:"url"`         // the page URL this applies to
	Issue       string   `json:"issue"`       // description of the problem
	CurrentText string   `json:"current_text"` // current text/value (if applicable)
	Suggestions []string `json:"suggestions"` // suggested replacement options
	Impact      string   `json:"impact"`      // expected impact description
}

// Report holds the complete SEO analysis
type Report struct {
	SiteURL         string           `json:"site_url"`
	AnalyzedAt      string           `json:"analyzed_at"`
	PagesAnalyzed   int              `json:"pages_analyzed"`
	Summary         Summary          `json:"summary"`
	Recommendations []Recommendation `json:"recommendations"`
}

// Summary provides high-level metrics
type Summary struct {
	TotalIssues    int            `json:"total_issues"`
	CriticalCount  int            `json:"critical_count"`
	HighCount      int            `json:"high_count"`
	MediumCount    int            `json:"medium_count"`
	LowCount       int            `json:"low_count"`
	TopCategories  []CategoryCount `json:"top_categories"`
}

// CategoryCount tracks issues per category
type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// Config holds analyzer configuration
type Config struct {
	SiteURL      string
	ScrapedDir   string
	Pages        []scraper.PageResult
	OutputDir    string
	Instructions string
}

// Result holds analyzer output
type Result struct {
	Report   *Report
	LogPath  string
}

// Analyze uses Claude to perform SEO analysis on scraped pages
func Analyze(cfg Config) (*Result, error) {
	// Build the prompt
	prompt := buildPrompt(cfg)

	// Run Claude
	logPath := filepath.Join(cfg.OutputDir, "optimus.log")
	report, err := runClaude(prompt, cfg.OutputDir, logPath)
	if err != nil {
		return nil, fmt.Errorf("running Claude analysis: %w", err)
	}

	// Sort recommendations by priority, then homepage first, then URL alphabetically
	sortRecommendations(report.Recommendations)

	// Compute summary stats
	report.Summary = computeSummary(report.Recommendations)

	return &Result{
		Report:  report,
		LogPath: logPath,
	}, nil
}

// buildPrompt creates the Claude prompt for SEO analysis
func buildPrompt(cfg Config) string {
	var pageList strings.Builder
	for _, page := range cfg.Pages {
		relPath, _ := filepath.Rel(cfg.OutputDir, page.FilePath)
		pageList.WriteString(fmt.Sprintf("- %s (Title: %q, File: %s)\n", page.URL, page.Title, relPath))
	}

	prompt := fmt.Sprintf(`You are an expert SEO analyst. Analyze the following website for SEO optimization opportunities.

## Website
URL: %s
Pages scraped: %d

## Page List
%s

## Instructions

Read each scraped HTML file listed above and analyze the website for SEO issues and optimization opportunities.

For each issue found, provide a structured recommendation. Focus on these categories:

1. **Title Tags** - Missing, too long, too short, not descriptive, missing keywords
2. **Meta Descriptions** - Missing, too long, too short, not compelling, missing call-to-action
3. **Heading Structure** - Missing H1, multiple H1s, skipped heading levels, non-descriptive headings
4. **Content Quality** - Thin content, keyword stuffing, missing keywords, poor readability, grammar issues
5. **Image Optimization** - Missing alt text, non-descriptive alt text, missing title attributes
6. **Internal Links** - Broken links, missing anchor text, orphan pages, poor link structure
7. **Structured Data** - Missing schema markup opportunities (LocalBusiness, Article, FAQ, etc.)
8. **Accessibility** - Missing ARIA labels, poor contrast indicators, missing form labels
9. **URL Structure** - Non-descriptive URLs, too long, missing keywords
10. **Mobile/Performance** - Missing viewport meta, render-blocking resources, large images

## Output Format

You MUST output a single JSON object (and nothing else) with this exact structure:

{
  "site_url": "%s",
  "analyzed_at": "%s",
  "pages_analyzed": %d,
  "recommendations": [
    {
      "priority": "critical|high|medium|low",
      "category": "title|meta|headings|content|images|links|structured-data|accessibility|url-structure|performance",
      "url": "https://example.com/page",
      "issue": "Description of the SEO issue",
      "current_text": "The current text or value that needs changing (leave empty if adding something new)",
      "suggestions": [
        "First suggested improvement option",
        "Second suggested improvement option",
        "Third suggested improvement option"
      ],
      "impact": "Expected improvement from making this change"
    }
  ]
}

## Priority Guidelines
- **critical**: Missing title tags, missing H1, no meta descriptions, broken core functionality
- **high**: Poor title/meta content, missing alt text on key images, heading hierarchy issues
- **medium**: Content improvements, missing structured data, internal link optimization
- **low**: Minor wording tweaks, nice-to-have additions, style improvements

## Important Rules
- Provide 2-3 suggestion options for each issue where text changes are needed
- Be specific - include the actual current text and specific replacement suggestions
- Focus on actionable changes, not vague advice
- Each recommendation should be independently implementable
- Order recommendations by priority (critical first, low last)
- Only output the JSON object, no markdown code fences, no explanatory text before or after

OUTPUT ONLY THE JSON OBJECT:`, cfg.SiteURL, len(cfg.Pages), pageList.String(), cfg.SiteURL, time.Now().Format(time.RFC3339), len(cfg.Pages))

	if cfg.Instructions != "" {
		prompt += fmt.Sprintf(`

## Additional Focus Areas (from user)
%s`, cfg.Instructions)
	}

	return prompt
}

// streamEvent represents a parsed JSON event from claude stream-json output
type streamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
	Result struct {
		Text string `json:"text,omitempty"`
	} `json:"result"`
}

// toolInput extracts file paths and commands from tool inputs
type toolInput struct {
	FilePath string `json:"file_path"`
	Command  string `json:"command"`
}

// runClaude invokes Claude CLI to perform the SEO analysis
func runClaude(prompt string, workDir string, logPath string) (*Report, error) {
	// Open log file
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}
	defer logFile.Close()

	logf := func(format string, a ...interface{}) {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintf(logFile, "[%s] %s\n", time.Now().Format("15:04:05"), msg)
	}

	logf("=== OPTIMUS SEO ANALYSIS SESSION ===")
	logf("Working directory: %s", workDir)
	logf("")
	logf("=== PROMPT ===")
	fmt.Fprintln(logFile, prompt)
	logf("")
	logf("=== CLAUDE SESSION ===")

	sp := ui.NewSpinner("Starting AI SEO analysis...")

	cmd := exec.Command("claude", "-p",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
	)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stderr = os.Stderr

	logf("Command: claude -p --output-format stream-json --verbose --dangerously-skip-permissions")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sp.Finish()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		sp.Finish()
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, fmt.Errorf("claude CLI not found — install it from https://claude.ai/code")
		}
		return nil, fmt.Errorf("starting claude: %w", err)
	}

	logf("Claude started (PID %d)", cmd.Process.Pid)

	// Collect the final text output for JSON parsing
	var finalText strings.Builder

	// Parse stream-json events
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fmt.Fprintln(logFile, line)

		var event streamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "system":
			if event.Subtype == "init" {
				logf("[STATUS] Session initialized")
				sp.Update("AI is analyzing...")
			}

		case "assistant":
			for _, content := range event.Message.Content {
				switch content.Type {
				case "text":
					text := strings.TrimSpace(content.Text)
					if text != "" {
						logf("[CLAUDE] %s", text)
						finalText.WriteString(content.Text)
						sp.Update("Generating SEO report...")
					}

				case "thinking":
					logf("[THINKING] (extended thinking block)")
					sp.Update("Thinking...")

				case "tool_use":
					var input toolInput
					json.Unmarshal(content.Input, &input)

					logf("[TOOL] %s", content.Name)
					if input.FilePath != "" {
						logf("  file: %s", input.FilePath)
					}

					switch content.Name {
					case "Read":
						file := filepath.Base(input.FilePath)
						sp.Update(fmt.Sprintf("Reading %s", file))
					case "Glob":
						sp.Update("Searching files...")
					case "Grep":
						sp.Update("Searching content...")
					default:
						sp.Update(fmt.Sprintf("Using %s...", content.Name))
					}
				}
			}

		case "result":
			if event.Result.Text != "" {
				finalText.WriteString(event.Result.Text)
				logf("[RESULT] Final text received")
			}

		case "user":
			logf("[RESULT] Tool result received")
			sp.Update("Analyzing...")
		}
	}

	cmdErr := cmd.Wait()
	sp.Finish()

	if cmdErr != nil {
		logf("[ERROR] Claude exited with error: %s", cmdErr)
		return nil, fmt.Errorf("claude exited with error: %w", cmdErr)
	}

	logf("[DONE] Claude finished successfully")

	// Parse the JSON report from Claude's output
	report, err := parseReport(finalText.String())
	if err != nil {
		logf("[ERROR] Failed to parse report: %s", err)
		logf("[RAW OUTPUT] %s", finalText.String())
		return nil, fmt.Errorf("parsing Claude output as JSON: %w", err)
	}

	return report, nil
}

// parseReport extracts the JSON report from Claude's text output
func parseReport(text string) (*Report, error) {
	text = strings.TrimSpace(text)

	// Try to find JSON object in the text
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in output")
	}

	jsonStr := text[start : end+1]

	var report Report
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &report, nil
}

// computeSummary calculates summary statistics from recommendations
func computeSummary(recs []Recommendation) Summary {
	s := Summary{
		TotalIssues: len(recs),
	}

	catCounts := make(map[string]int)
	for _, r := range recs {
		switch r.Priority {
		case "critical":
			s.CriticalCount++
		case "high":
			s.HighCount++
		case "medium":
			s.MediumCount++
		case "low":
			s.LowCount++
		}
		catCounts[r.Category]++
	}

	for cat, count := range catCounts {
		s.TopCategories = append(s.TopCategories, CategoryCount{
			Category: cat,
			Count:    count,
		})
	}

	return s
}

// priorityRank returns a numeric rank for sorting (lower = higher priority)
func priorityRank(p string) int {
	switch p {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

// isHomePage returns true if the URL is the site's home/index page
func isHomePage(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := strings.TrimRight(parsed.Path, "/")
	return path == "" || path == "/index.html" || path == "/index.php" || path == "/index"
}

// sortRecommendations sorts by priority (critical first), then homepage first, then URL alphabetically
func sortRecommendations(recs []Recommendation) {
	sort.SliceStable(recs, func(i, j int) bool {
		pi, pj := priorityRank(recs[i].Priority), priorityRank(recs[j].Priority)
		if pi != pj {
			return pi < pj
		}
		hi, hj := isHomePage(recs[i].URL), isHomePage(recs[j].URL)
		if hi != hj {
			return hi
		}
		return recs[i].URL < recs[j].URL
	})
}
