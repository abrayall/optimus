package engine

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"optimus/core/lib/scraper"
	"optimus/core/lib/ui"
)

//go:embed skills/*.md skills/*.json
var skillsFS embed.FS

// Skill represents a loaded skill with metadata and prompt template
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Output      string `json:"output"`
	Prompt      string // loaded from .md file, not in JSON
}

// FileEntry represents a file to be written (used by "files" output type)
type FileEntry struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

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

// Scorecard holds the complete ranking scorecard
type Scorecard struct {
	SiteURL        string         `json:"site_url"`
	AnalyzedAt     string         `json:"analyzed_at"`
	PagesAnalyzed  int            `json:"pages_analyzed"`
	OverallScore   int            `json:"overall_score"`
	CategoryScores CategoryScores `json:"category_scores"`
	DomainAuth      *DomainAuthority `json:"domain_authority,omitempty"`
	BacklinkProfile *BacklinkProfile `json:"backlink_profile,omitempty"`
	SerpPositions   []SerpPosition   `json:"serp_positions,omitempty"`
	AICitations     []AICitation     `json:"ai_citations,omitempty"`
	Pages          []ScorecardPage `json:"pages"`
	Findings       []string       `json:"findings"`
}

// CategoryScores holds scores per category (0-100)
type CategoryScores struct {
	SearchRank int `json:"search_rank"`
	AnswerRank int `json:"answer_rank"`
	Technical  int `json:"technical"`
	Content    int `json:"content"`
	Structure  int `json:"structure"`
}

// SerpPosition holds a SERP lookup result for a keyword
type SerpPosition struct {
	Keyword     string `json:"keyword"`
	Engine      string `json:"engine"`
	Position    int    `json:"position"`
	DomainFound bool   `json:"domain_found"`
	URLFound    string `json:"url_found"`
}

// AICitation holds the result of an AI citation check
type AICitation struct {
	Question      string `json:"question"`
	Cited         bool   `json:"cited"`
	AnswerExcerpt string `json:"answer_excerpt"`
}

// DomainAuthority holds domain authority metrics from Moz/Ahrefs
type DomainAuthority struct {
	MozDA              float64 `json:"moz_da"`
	MozPA              float64 `json:"moz_pa"`
	MozSpamScore       float64 `json:"moz_spam_score"`
	LinkingRootDomains int     `json:"linking_root_domains"`
	AhrefsDR           float64 `json:"ahrefs_dr"`
	AhrefsRank         int     `json:"ahrefs_rank"`
}

// BacklinkProfile holds backlink stats for the ranking scorecard
type BacklinkProfile struct {
	LiveBacklinks    int `json:"live_backlinks"`
	ReferringDomains int `json:"referring_domains"`
	ReferringPages   int `json:"referring_pages"`
}

// BacklinkStrategy holds the output of the backlinks skill
type BacklinkStrategy struct {
	SiteURL       string                `json:"site_url"`
	AnalyzedAt    string                `json:"analyzed_at"`
	PagesAnalyzed int                   `json:"pages_analyzed"`
	Summary       BacklinkSummary       `json:"summary"`
	Opportunities []BacklinkOpportunity `json:"opportunities"`
}

// BacklinkSummary holds overview stats for the backlink strategy
type BacklinkSummary struct {
	CurrentDA        float64 `json:"current_da"`
	CurrentDR        float64 `json:"current_dr"`
	ReferringDomains int     `json:"referring_domains"`
	TotalOpps        int     `json:"total_opportunities"`
	QuickWins        int     `json:"quick_wins"`
	HighROI          int     `json:"high_roi"`
}

// BacklinkOpportunity represents a single backlink building idea
type BacklinkOpportunity struct {
	Strategy    string   `json:"strategy"`
	Difficulty  string   `json:"difficulty"`
	Impact      string   `json:"impact"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	TargetURL   string   `json:"target_url"`
	Steps       []string `json:"steps"`
}

// ScorecardPage holds per-page scoring data
type ScorecardPage struct {
	URL             string   `json:"url"`
	Title           string   `json:"title"`
	SearchReadiness int      `json:"search_readiness"`
	AnswerReadiness int      `json:"answer_readiness"`
	PrimaryKeyword  string   `json:"primary_keyword"`
	WordCount       int      `json:"word_count"`
	HasSchema       bool     `json:"has_schema"`
	HasFAQ          bool     `json:"has_faq"`
	Issues          []string `json:"issues"`
}

// Config holds engine configuration
type Config struct {
	SiteURL      string
	ScrapedDir   string
	Pages        []scraper.PageResult
	OutputDir    string
	Instructions string
	Skill        string

	// External API keys
	SerpAPIKey          string
	GoogleAPIKey        string
	GoogleCSEID         string
	GSCCredentials      string
	PerplexityKey       string
	MozAPIKey           string
	AhrefsAPIKey        string
	BingAPIKey          string
	RedditClientID      string
	RedditClientSecret  string
	TwitterBearerToken  string
}

// Result holds engine output (output-type-agnostic)
type Result struct {
	Skill     *Skill
	RawOutput string
	LogPath   string
	SessionID string
}

// LoadSkill loads a skill's metadata and prompt template from embedded files
func LoadSkill(name string) (*Skill, error) {
	// Load JSON metadata
	jsonPath := fmt.Sprintf("skills/%s.json", name)
	jsonData, err := skillsFS.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found: %w", name, err)
	}

	var skill Skill
	if err := json.Unmarshal(jsonData, &skill); err != nil {
		return nil, fmt.Errorf("invalid skill metadata %q: %w", name, err)
	}

	// Load prompt template
	mdPath := fmt.Sprintf("skills/%s.md", name)
	mdData, err := skillsFS.ReadFile(mdPath)
	if err != nil {
		return nil, fmt.Errorf("skill prompt %q not found: %w", name, err)
	}
	skill.Prompt = string(mdData)

	return &skill, nil
}

// ListSkills returns the names of all available skills
func ListSkills() []string {
	entries, err := skillsFS.ReadDir("skills")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}

// ParseReport extracts the JSON report from Claude's text output
func ParseReport(text string) (*Report, error) {
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

	// Sort recommendations by priority, then homepage first, then URL alphabetically
	sortRecommendations(report.Recommendations)

	// Compute summary stats
	report.Summary = computeSummary(report.Recommendations)

	return &report, nil
}

// ParseFiles extracts the JSON array of file entries from Claude's text output
func ParseFiles(text string) ([]FileEntry, error) {
	text = strings.TrimSpace(text)

	// Try to find JSON array in the text
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in output")
	}

	jsonStr := text[start : end+1]

	var files []FileEntry
	if err := json.Unmarshal([]byte(jsonStr), &files); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return files, nil
}

// ParseScorecard extracts a scorecard JSON object from Claude's text output
func ParseScorecard(text string) (*Scorecard, error) {
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in output")
	}

	jsonStr := text[start : end+1]

	var sc Scorecard
	if err := json.Unmarshal([]byte(jsonStr), &sc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &sc, nil
}

// ParseBacklinks extracts a backlink strategy JSON object from Claude's text output
func ParseBacklinks(text string) (*BacklinkStrategy, error) {
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in output")
	}

	jsonStr := text[start : end+1]

	var bs BacklinkStrategy
	if err := json.Unmarshal([]byte(jsonStr), &bs); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &bs, nil
}

// Run uses Claude to execute a skill on scraped pages
func Run(cfg Config) (*Result, error) {
	// Load the skill
	skillName := cfg.Skill
	if skillName == "" {
		skillName = "seo"
	}

	skill, err := LoadSkill(skillName)
	if err != nil {
		available := ListSkills()
		return nil, fmt.Errorf("skill %q not found. Available skills: %s", skillName, strings.Join(available, ", "))
	}

	// Build the prompt
	prompt := buildPrompt(cfg, skill)

	// Write MCP config for Claude to connect to optimus tools
	mcpConfigPath, err := writeMCPConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("writing MCP config: %w", err)
	}
	defer os.Remove(mcpConfigPath)

	// Run Claude
	logPath := filepath.Join(cfg.OutputDir, "optimus.log")
	rawOutput, sessionID, err := runClaude(prompt, skill, cfg.OutputDir, logPath, mcpConfigPath)
	if err != nil {
		return nil, fmt.Errorf("running Claude: %w", err)
	}

	return &Result{
		Skill:     skill,
		RawOutput: rawOutput,
		LogPath:   logPath,
		SessionID: sessionID,
	}, nil
}

// promptData holds the template variables available to skill prompts
type promptData struct {
	SiteURL   string
	PageCount int
	PageList  string
	Timestamp string
}

// buildPrompt creates the Claude prompt by rendering a skill template
func buildPrompt(cfg Config, skill *Skill) string {
	var pageList strings.Builder
	for _, page := range cfg.Pages {
		relPath, _ := filepath.Rel(cfg.OutputDir, page.FilePath)
		pageList.WriteString(fmt.Sprintf("- %s (Title: %q, File: %s)\n", page.URL, page.Title, relPath))
	}

	// Render the template
	tmpl, err := template.New(skill.Name).Parse(skill.Prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid skill template %q: %s\n", skill.Name, err)
		os.Exit(1)
	}

	data := promptData{
		SiteURL:   cfg.SiteURL,
		PageCount: len(cfg.Pages),
		PageList:  pageList.String(),
		Timestamp: time.Now().Format(time.RFC3339),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		fmt.Fprintf(os.Stderr, "Error: rendering skill template %q: %s\n", skill.Name, err)
		os.Exit(1)
	}

	prompt := buf.String()

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
	SessionID string `json:"session_id,omitempty"`
	Result    struct {
		Text string `json:"text,omitempty"`
	} `json:"result"`
}

// toolInput extracts file paths and commands from tool inputs
type toolInput struct {
	FilePath string `json:"file_path"`
	Command  string `json:"command"`
}

// writeMCPConfig creates a temporary MCP config file pointing to the optimus binary
func writeMCPConfig(cfg Config) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable path: %w", err)
	}

	mcpArgs := []string{"--mcp"}

	// Forward API keys to MCP subprocess
	if cfg.SerpAPIKey != "" {
		mcpArgs = append(mcpArgs, "--serp-api-key", cfg.SerpAPIKey)
	}
	if cfg.GoogleAPIKey != "" {
		mcpArgs = append(mcpArgs, "--google-api-key", cfg.GoogleAPIKey)
	}
	if cfg.GoogleCSEID != "" {
		mcpArgs = append(mcpArgs, "--google-cse-id", cfg.GoogleCSEID)
	}
	if cfg.GSCCredentials != "" {
		mcpArgs = append(mcpArgs, "--gsc-credentials", cfg.GSCCredentials)
	}
	if cfg.PerplexityKey != "" {
		mcpArgs = append(mcpArgs, "--perplexity-key", cfg.PerplexityKey)
	}
	if cfg.MozAPIKey != "" {
		mcpArgs = append(mcpArgs, "--moz-api-key", cfg.MozAPIKey)
	}
	if cfg.AhrefsAPIKey != "" {
		mcpArgs = append(mcpArgs, "--ahrefs-api-key", cfg.AhrefsAPIKey)
	}
	if cfg.BingAPIKey != "" {
		mcpArgs = append(mcpArgs, "--bing-api-key", cfg.BingAPIKey)
	}
	if cfg.RedditClientID != "" {
		mcpArgs = append(mcpArgs, "--reddit-client-id", cfg.RedditClientID)
	}
	if cfg.RedditClientSecret != "" {
		mcpArgs = append(mcpArgs, "--reddit-client-secret", cfg.RedditClientSecret)
	}
	if cfg.TwitterBearerToken != "" {
		mcpArgs = append(mcpArgs, "--twitter-bearer-token", cfg.TwitterBearerToken)
	}

	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"optimus": map[string]interface{}{
				"command": exe,
				"args":    mcpArgs,
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(cfg.OutputDir, "mcp-config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", err
	}

	return configPath, nil
}

// runClaude invokes Claude CLI and returns the raw text output
func runClaude(prompt string, skill *Skill, workDir string, logPath string, mcpConfigPath string) (string, string, error) {
	// Open log file
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", "", fmt.Errorf("creating log file: %w", err)
	}
	defer logFile.Close()

	logf := func(format string, a ...interface{}) {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintf(logFile, "[%s] %s\n", time.Now().Format("15:04:05"), msg)
	}

	logf("=== OPTIMUS SESSION ===")
	logf("Skill: %s (%s)", skill.Name, skill.Output)
	logf("Working directory: %s", workDir)
	logf("")
	logf("=== PROMPT ===")
	fmt.Fprintln(logFile, prompt)
	logf("")
	logf("=== CLAUDE SESSION ===")

	sp := ui.NewSpinner(fmt.Sprintf("Starting %s...", skill.Name))

	cmd := exec.Command("claude", "-p",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--mcp-config", mcpConfigPath,
	)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	logf("Command: claude -p --output-format stream-json --verbose --dangerously-skip-permissions --mcp-config %s", mcpConfigPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sp.Finish()
		return "", "", fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		sp.Finish()
		if strings.Contains(err.Error(), "executable file not found") {
			return "", "", fmt.Errorf("claude CLI not found — install it from https://claude.ai/code")
		}
		return "", "", fmt.Errorf("starting claude: %w", err)
	}

	logf("Claude started (PID %d)", cmd.Process.Pid)

	// Collect the final text output
	var finalText strings.Builder
	var sessionID string

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
				sp.Update("AI is working...")
			}

		case "assistant":
			for _, content := range event.Message.Content {
				switch content.Type {
				case "text":
					text := strings.TrimSpace(content.Text)
					if text != "" {
						logf("[CLAUDE] %s", text)
						finalText.WriteString(content.Text)
						sp.Update(fmt.Sprintf("Generating %s output...", skill.Name))
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
					case "fetch_headers":
						sp.Update("Checking HTTP headers...")
					case "fetch_robots_txt":
						sp.Update("Checking robots.txt...")
					case "fetch_sitemap":
						sp.Update("Checking sitemap...")
					case "check_links":
						sp.Update("Checking links...")
					case "check_ssl":
						sp.Update("Checking SSL certificate...")
					case "dns_lookup":
						sp.Update("Looking up DNS records...")
					case "serp_lookup":
						sp.Update("Looking up SERP positions...")
					case "google_search":
						sp.Update("Searching Google...")
					case "search_console_query":
						sp.Update("Querying Search Console...")
					case "perplexity_ask":
						sp.Update("Asking Perplexity AI...")
					case "moz_url_metrics":
						sp.Update("Checking Moz domain authority...")
					case "ahrefs_domain_rating":
						sp.Update("Checking Ahrefs domain rating...")
					case "ahrefs_backlinks_stats":
						sp.Update("Checking Ahrefs backlinks...")
					case "ahrefs_organic_keywords":
						sp.Update("Checking Ahrefs organic keywords...")
					case "pagespeed_insights":
						sp.Update("Running PageSpeed Insights...")
					case "url_inspection":
						sp.Update("Inspecting URL indexing status...")
					case "bing_webmaster_stats":
						sp.Update("Checking Bing Webmaster stats...")
					case "reddit_search":
						sp.Update("Searching Reddit mentions...")
					case "twitter_search":
						sp.Update("Searching Twitter mentions...")
					default:
						sp.Update(fmt.Sprintf("Using %s...", content.Name))
					}
				}
			}

		case "result":
			if event.SessionID != "" {
				sessionID = event.SessionID
				logf("[SESSION] %s", sessionID)
			}
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
		stderrStr := strings.TrimSpace(stderrBuf.String())
		logf("[ERROR] Claude exited with error: %s (stderr: %s)", cmdErr, stderrStr)
		if stderrStr != "" {
			return "", "", fmt.Errorf("claude exited with error: %w\nstderr: %s", cmdErr, stderrStr)
		}
		return "", "", fmt.Errorf("claude exited with error: %w", cmdErr)
	}

	logf("[DONE] Claude finished successfully")

	return finalText.String(), sessionID, nil
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
