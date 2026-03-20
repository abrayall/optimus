package render

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"optimus/core/lib/engine"
)

// Config holds report renderer configuration
type Config struct {
	Report    *engine.Report
	OutputDir string
	CSS       string
	LogoSVG   string
}

// ScorecardConfig holds scorecard renderer configuration
type ScorecardConfig struct {
	Scorecard *engine.Scorecard
	OutputDir string
	CSS       string
	LogoSVG   string
}

// BacklinksConfig holds backlinks renderer configuration
type BacklinksConfig struct {
	Strategy  *engine.BacklinkStrategy
	OutputDir string
	CSS       string
	LogoSVG   string
}

// FilesConfig holds files renderer configuration
type FilesConfig struct {
	Files     []engine.FileEntry
	SiteURL   string
	SkillName string
	OutputDir string
	CSS       string
	LogoSVG   string
}

// Result holds renderer output
type Result struct {
	JSONPath string
	HTMLPath string
}

// Internal template data wrappers that embed the data types and add CSS/LogoSVG
type reportData struct {
	*engine.Report
	CSS     template.CSS
	LogoSVG template.HTML
}

type scorecardData struct {
	*engine.Scorecard
	CSS     template.CSS
	LogoSVG template.HTML
}

type backlinksData struct {
	*engine.BacklinkStrategy
	CSS     template.CSS
	LogoSVG template.HTML
}

type filesData struct {
	Files     []engine.FileEntry
	SiteURL   string
	SkillName string
	CSS       template.CSS
	LogoSVG   template.HTML
}

// Generate creates both JSON and HTML reports
func Generate(cfg Config) (*Result, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	jsonPath := filepath.Join(cfg.OutputDir, "report.json")
	htmlPath := filepath.Join(cfg.OutputDir, "report.html")

	// Write JSON report
	if err := writeJSON(cfg.Report, jsonPath); err != nil {
		return nil, fmt.Errorf("writing JSON report: %w", err)
	}

	// Write HTML report
	if err := writeHTML(cfg.Report, cfg.CSS, cfg.LogoSVG, htmlPath); err != nil {
		return nil, fmt.Errorf("writing HTML report: %w", err)
	}

	return &Result{
		JSONPath: jsonPath,
		HTMLPath: htmlPath,
	}, nil
}

// GenerateFiles writes files to disk and creates an HTML viewer page
func GenerateFiles(cfg FilesConfig) (*Result, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Write individual files
	for _, f := range cfg.Files {
		filePath := filepath.Join(cfg.OutputDir, f.Filename)
		if err := os.WriteFile(filePath, []byte(f.Content), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", f.Filename, err)
		}
	}

	// Generate HTML viewer
	htmlPath := filepath.Join(cfg.OutputDir, "index.html")
	if err := writeFilesHTML(cfg, htmlPath); err != nil {
		return nil, fmt.Errorf("writing HTML viewer: %w", err)
	}

	return &Result{
		HTMLPath: htmlPath,
	}, nil
}

// GenerateScorecard creates both JSON and HTML scorecard files
func GenerateScorecard(cfg ScorecardConfig) (*Result, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	jsonPath := filepath.Join(cfg.OutputDir, "scorecard.json")
	htmlPath := filepath.Join(cfg.OutputDir, "scorecard.html")

	// Write JSON
	data, err := json.MarshalIndent(cfg.Scorecard, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling scorecard JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return nil, fmt.Errorf("writing scorecard JSON: %w", err)
	}

	// Write HTML
	if err := writeScorecardHTML(cfg.Scorecard, cfg.CSS, cfg.LogoSVG, htmlPath); err != nil {
		return nil, fmt.Errorf("writing scorecard HTML: %w", err)
	}

	return &Result{
		JSONPath: jsonPath,
		HTMLPath: htmlPath,
	}, nil
}

// GenerateBacklinks creates both JSON and HTML backlink strategy reports
func GenerateBacklinks(cfg BacklinksConfig) (*Result, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	jsonPath := filepath.Join(cfg.OutputDir, "backlinks.json")
	htmlPath := filepath.Join(cfg.OutputDir, "backlinks.html")

	// Write JSON
	data, err := json.MarshalIndent(cfg.Strategy, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling backlinks JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return nil, fmt.Errorf("writing backlinks JSON: %w", err)
	}

	// Write HTML
	if err := writeBacklinksHTML(cfg.Strategy, cfg.CSS, cfg.LogoSVG, htmlPath); err != nil {
		return nil, fmt.Errorf("writing backlinks HTML: %w", err)
	}

	return &Result{
		JSONPath: jsonPath,
		HTMLPath: htmlPath,
	}, nil
}

func writeBacklinksHTML(bs *engine.BacklinkStrategy, css, logoSVG, path string) error {
	tmpl, err := template.New("backlinks").Funcs(template.FuncMap{
		"strategyIcon":  strategyIcon,
		"difficultyTag": difficultyTag,
		"impactTag":     impactTag,
		"add":           func(a, b int) int { return a + b },
	}).Parse(backlinksHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	data := backlinksData{
		BacklinkStrategy: bs,
		CSS:              template.CSS(css),
		LogoSVG:          template.HTML(logoSVG),
	}
	return tmpl.Execute(f, data)
}

func strategyIcon(strategy string) string {
	switch strategy {
	case "content-creation":
		return "📝"
	case "guest-posting":
		return "✍️"
	case "resource-pages":
		return "📚"
	case "digital-pr":
		return "📰"
	case "competitor-gap":
		return "🎯"
	case "broken-links":
		return "🔗"
	case "directories":
		return "📁"
	case "community":
		return "💬"
	case "unlinked-mentions":
		return "🔔"
	default:
		return "📌"
	}
}

func difficultyTag(difficulty string) string {
	switch difficulty {
	case "easy":
		return "#ADEEE3"
	case "medium":
		return "#0090C1"
	case "hard":
		return "#EF4444"
	default:
		return "#888888"
	}
}

func impactTag(impact string) string {
	switch impact {
	case "high":
		return "#ADEEE3"
	case "medium":
		return "#0090C1"
	case "low":
		return "#046E8F"
	default:
		return "#888888"
	}
}

func writeScorecardHTML(sc *engine.Scorecard, css, logoSVG, path string) error {
	tmpl, err := template.New("scorecard").Funcs(template.FuncMap{
		"scorecardColor": scorecardColor,
		"scorecardLabel": scorecardLabel,
		"degrees":        func(score int) float64 { return float64(score) * 3.6 },
	}).Parse(scorecardHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	data := scorecardData{
		Scorecard: sc,
		CSS:       template.CSS(css),
		LogoSVG:   template.HTML(logoSVG),
	}
	return tmpl.Execute(f, data)
}

func scorecardColor(score int) string {
	switch {
	case score >= 80:
		return "#ADEEE3" // mint
	case score >= 60:
		return "#0090C1" // blue
	case score >= 40:
		return "#046E8F" // teal
	default:
		return "#EF4444" // red
	}
}

func scorecardLabel(score int) string {
	switch {
	case score >= 80:
		return "Good"
	case score >= 60:
		return "Fair"
	case score >= 40:
		return "Needs Work"
	default:
		return "Poor"
	}
}

func writeFilesHTML(cfg FilesConfig, path string) error {
	tmpl, err := template.New("files").Funcs(template.FuncMap{
		"json": func(s string) template.JS {
			b, _ := json.Marshal(s)
			return template.JS(b)
		},
	}).Parse(filesHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	data := filesData{
		Files:     cfg.Files,
		SiteURL:   cfg.SiteURL,
		SkillName: cfg.SkillName,
		CSS:       template.CSS(cfg.CSS),
		LogoSVG:   template.HTML(cfg.LogoSVG),
	}
	return tmpl.Execute(f, data)
}

// writeJSON writes the report as formatted JSON
func writeJSON(report *engine.Report, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// writeHTML generates an HTML report
func writeHTML(report *engine.Report, css, logoSVG, path string) error {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"priorityColor": priorityColor,
		"priorityIcon":  priorityIcon,
		"categoryIcon":  categoryIcon,
		"upper":         strings.ToUpper,
		"join":          strings.Join,
		"add":           func(a, b int) int { return a + b },
	}).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	data := reportData{
		Report:  report,
		CSS:     template.CSS(css),
		LogoSVG: template.HTML(logoSVG),
	}
	return tmpl.Execute(f, data)
}

func priorityColor(priority string) string {
	switch priority {
	case "critical":
		return "#EF4444"
	case "high":
		return "#F97316"
	case "medium":
		return "#0090C1"
	case "low":
		return "#ADEEE3"
	default:
		return "#888888"
	}
}

func priorityIcon(priority string) string {
	switch priority {
	case "critical":
		return "🔴"
	case "high":
		return "🟠"
	case "medium":
		return "🟡"
	case "low":
		return "🔵"
	default:
		return "⚪"
	}
}

func categoryIcon(category string) string {
	switch category {
	case "title":
		return "🏷️"
	case "meta":
		return "📋"
	case "headings":
		return "📑"
	case "content":
		return "📝"
	case "images":
		return "🖼️"
	case "links":
		return "🔗"
	case "structured-data":
		return "📊"
	case "accessibility":
		return "♿"
	case "url-structure":
		return "🌐"
	case "performance":
		return "⚡"
	default:
		return "📌"
	}
}

var htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Optimus SEO Report — {{.SiteURL}}</title>
    <style>{{.CSS}}</style>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--dark, #022F40);
            color: #e2e8f0;
            line-height: 1.6;
            overflow-x: auto;
        }
        .report-container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }
        header {
            text-align: center;
            padding: 3rem 0;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            margin-bottom: 2rem;
        }
        .logo-row {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 12px;
            margin-bottom: 16px;
        }
        .logo-row svg { width: 36px; height: 36px; }
        .logo-text {
            font-size: 1.5rem;
            font-weight: 700;
            color: #fff;
        }
        header h1 {
            font-size: 2.5rem;
            background: linear-gradient(135deg, var(--primary, #046E8F), var(--blue, #0090C1));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 0.5rem;
        }
        header .site-url {
            color: rgba(255,255,255,0.7);
            font-size: 1.1rem;
        }
        header .meta {
            color: rgba(255,255,255,0.5);
            font-size: 0.9rem;
            margin-top: 0.5rem;
        }

        /* Summary Cards */
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .summary-card {
            background: var(--dark-gray, #183446);
            border-radius: 12px;
            padding: 1.5rem;
            text-align: center;
            border: 1px solid rgba(255,255,255,0.1);
        }
        .summary-card .number {
            font-size: 2.5rem;
            font-weight: 700;
        }
        .summary-card .label {
            color: rgba(255,255,255,0.7);
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .summary-card.total .number { color: var(--blue, #0090C1); }
        .summary-card.critical .number { color: #ef4444; }
        .summary-card.high .number { color: #f97316; }
        .summary-card.medium .number { color: var(--blue, #0090C1); }
        .summary-card.low .number { color: var(--mint, #ADEEE3); }

        /* Filters */
        .filters {
            display: flex;
            gap: 0.5rem;
            margin-bottom: 1.5rem;
            flex-wrap: wrap;
        }
        .filter-btn {
            padding: 0.5rem 1rem;
            border-radius: 8px;
            border: 1px solid rgba(255,255,255,0.1);
            background: var(--dark-gray, #183446);
            color: rgba(255,255,255,0.7);
            cursor: pointer;
            font-size: 0.85rem;
            transition: all 0.2s;
            font-family: inherit;
        }
        .filter-btn:hover, .filter-btn.active {
            background: rgba(255,255,255,0.1);
            color: var(--blue, #0090C1);
            border-color: var(--blue, #0090C1);
        }
        .filter-dot {
            display: inline-block;
            width: 8px;
            height: 8px;
            border-radius: 50%;
            margin-right: 6px;
            vertical-align: middle;
        }

        /* Recommendations */
        .recommendations {
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }
        .rec-card {
            background: var(--dark-gray, #183446);
            border-radius: 12px;
            border: 1px solid rgba(255,255,255,0.1);
            overflow: hidden;
            transition: border-color 0.2s;
        }
        .rec-card:hover {
            border-color: rgba(255,255,255,0.2);
        }
        .rec-header {
            display: flex;
            align-items: center;
            gap: 1rem;
            padding: 1rem 1.5rem;
            cursor: pointer;
        }
        .rec-tags {
            display: flex;
            flex-direction: column;
            gap: 0.25rem;
            flex-shrink: 0;
            width: 110px;
        }
        .rec-priority {
            padding: 0.2rem 0.6rem;
            border-radius: 4px;
            font-size: 0.7rem;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            white-space: nowrap;
            text-align: center;
            width: fit-content;
        }
        .rec-category {
            padding: 0.15rem 0.5rem;
            border-radius: 4px;
            font-size: 0.6rem;
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            white-space: nowrap;
            width: fit-content;
            background: rgba(255,255,255,0.1);
            color: rgba(255,255,255,0.7);
        }
        .rec-issue {
            flex: 1;
            font-weight: 500;
        }
        .rec-url {
            font-size: 0.8rem;
            color: rgba(255,255,255,0.5);
            max-width: 200px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .rec-details {
            display: none;
            padding: 0 1.5rem 1.5rem;
            border-top: 1px solid rgba(255,255,255,0.1);
        }
        .rec-card.open .rec-details {
            display: block;
        }
        .rec-section {
            margin-top: 1rem;
        }
        .rec-section-title {
            font-size: 0.8rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: rgba(255,255,255,0.5);
            margin-bottom: 0.5rem;
        }
        .current-text {
            background: var(--dark, #022F40);
            border: 1px solid rgba(255,255,255,0.1);
            border-left: 3px solid #ef4444;
            padding: 0.75rem 1rem;
            border-radius: 6px;
            font-family: monospace;
            font-size: 0.9rem;
            color: #f87171;
            word-break: break-word;
        }
        .suggestion {
            background: var(--dark, #022F40);
            border: 1px solid rgba(255,255,255,0.1);
            border-left: 3px solid var(--mint, #ADEEE3);
            padding: 0.75rem 1rem;
            border-radius: 6px;
            font-size: 0.9rem;
            color: var(--mint, #ADEEE3);
            margin-bottom: 0.5rem;
            word-break: break-word;
        }
        .suggestion-number {
            color: rgba(255,255,255,0.5);
            font-size: 0.75rem;
            margin-right: 0.5rem;
        }
        .impact {
            background: var(--dark, #022F40);
            border: 1px solid rgba(255,255,255,0.1);
            border-left: 3px solid var(--blue, #0090C1);
            padding: 0.75rem 1rem;
            border-radius: 6px;
            font-size: 0.9rem;
            color: #7dd3fc;
        }
        .chevron {
            transition: transform 0.2s;
            color: rgba(255,255,255,0.5);
        }
        .rec-card.open .chevron {
            transform: rotate(90deg);
        }

        /* Footer */
        .report-footer {
            text-align: center;
            padding: 2rem 0;
            margin-top: 2rem;
            border-top: 1px solid rgba(255,255,255,0.1);
            color: rgba(255,255,255,0.3);
            font-size: 0.85rem;
        }

        @media (max-width: 768px) {
            .report-container { padding: 1rem; }
            header h1 { font-size: 1.8rem; }
            .rec-header { flex-wrap: wrap; }
            .rec-url { max-width: 100%; }
        }
    </style>
</head>
<body>
    <div class="report-container">
        <header>
            <div class="logo-row">
                {{.LogoSVG}}
                <span class="logo-text">Optimus</span>
            </div>
            <h1>SEO Report</h1>
            <div class="site-url">{{.SiteURL}}</div>
            <div class="meta">Analyzed {{.AnalyzedAt}} · {{.PagesAnalyzed}} pages scanned</div>
        </header>

        <div class="summary">
            <div class="summary-card total">
                <div class="number">{{.Summary.TotalIssues}}</div>
                <div class="label">Total Issues</div>
            </div>
            <div class="summary-card critical">
                <div class="number">{{.Summary.CriticalCount}}</div>
                <div class="label">Critical</div>
            </div>
            <div class="summary-card high">
                <div class="number">{{.Summary.HighCount}}</div>
                <div class="label">High</div>
            </div>
            <div class="summary-card medium">
                <div class="number">{{.Summary.MediumCount}}</div>
                <div class="label">Medium</div>
            </div>
            <div class="summary-card low">
                <div class="number">{{.Summary.LowCount}}</div>
                <div class="label">Low</div>
            </div>
        </div>

        <div class="filters">
            <button class="filter-btn active" onclick="filterRecs('all')">All</button>
            <button class="filter-btn" onclick="filterRecs('critical')"><span class="filter-dot" style="background:#EF4444"></span>Critical</button>
            <button class="filter-btn" onclick="filterRecs('high')"><span class="filter-dot" style="background:#F97316"></span>High</button>
            <button class="filter-btn" onclick="filterRecs('medium')"><span class="filter-dot" style="background:#0090C1"></span>Medium</button>
            <button class="filter-btn" onclick="filterRecs('low')"><span class="filter-dot" style="background:#ADEEE3"></span>Low</button>
        </div>

        <div class="recommendations">
            {{range $i, $rec := .Recommendations}}
            <div class="rec-card" data-priority="{{$rec.Priority}}" data-category="{{$rec.Category}}">
                <div class="rec-header" onclick="toggleRec(this)">
                    <span class="chevron">▶</span>
                    <div class="rec-tags">
                        <span class="rec-priority" style="background: {{priorityColor $rec.Priority}}22; color: {{priorityColor $rec.Priority}}">
                            {{upper $rec.Priority}}
                        </span>
                        <span class="rec-category">{{$rec.Category}}</span>
                    </div>
                    <span class="rec-issue">{{$rec.Issue}}</span>
                    <span class="rec-url" title="{{$rec.URL}}">{{$rec.URL}}</span>
                </div>
                <div class="rec-details">
                    {{if $rec.CurrentText}}
                    <div class="rec-section">
                        <div class="rec-section-title">Current</div>
                        <div class="current-text">{{$rec.CurrentText}}</div>
                    </div>
                    {{end}}
                    {{if $rec.Suggestions}}
                    <div class="rec-section">
                        <div class="rec-section-title">Suggested Changes</div>
                        {{range $j, $sug := $rec.Suggestions}}
                        <div class="suggestion">
                            <span class="suggestion-number">Option {{add $j 1}}:</span>
                            {{$sug}}
                        </div>
                        {{end}}
                    </div>
                    {{end}}
                    {{if $rec.Impact}}
                    <div class="rec-section">
                        <div class="rec-section-title">Expected Impact</div>
                        <div class="impact">{{$rec.Impact}}</div>
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
        </div>

        <div class="report-footer">
            Generated by <strong>Optimus</strong>
        </div>
    </div>

    <script>
        function toggleRec(header) {
            header.parentElement.classList.toggle('open');
        }

        function filterRecs(priority) {
            document.querySelectorAll('.filter-btn').forEach(btn => btn.classList.remove('active'));
            event.target.closest('.filter-btn').classList.add('active');

            document.querySelectorAll('.rec-card').forEach(card => {
                if (priority === 'all' || card.dataset.priority === priority) {
                    card.style.display = 'block';
                } else {
                    card.style.display = 'none';
                }
            });
        }

        // Expand all critical issues by default
        document.querySelectorAll('.rec-card[data-priority="critical"]').forEach(card => {
            card.classList.add('open');
        });
    </script>
</body>
</html>`

var filesHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Optimus {{.SkillName}} — {{.SiteURL}}</title>
    <script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
    <style>{{.CSS}}</style>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--dark, #022F40);
            color: #e2e8f0;
            line-height: 1.6;
            overflow-x: auto;
        }
        .report-container {
            max-width: 1000px;
            margin: 0 auto;
            padding: 2rem;
        }
        header {
            text-align: center;
            padding: 3rem 0;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            margin-bottom: 2rem;
        }
        .logo-row {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 12px;
            margin-bottom: 16px;
        }
        .logo-row svg { width: 36px; height: 36px; }
        .logo-text {
            font-size: 1.5rem;
            font-weight: 700;
            color: #fff;
        }
        header h1 {
            font-size: 2.5rem;
            background: linear-gradient(135deg, var(--primary, #046E8F), var(--blue, #0090C1));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 0.5rem;
        }
        header .site-url {
            color: rgba(255,255,255,0.7);
            font-size: 1.1rem;
        }
        header .meta {
            color: rgba(255,255,255,0.5);
            font-size: 0.9rem;
            margin-top: 0.5rem;
        }

        /* Tabs */
        .tabs {
            display: flex;
            gap: 0.25rem;
            border-bottom: 2px solid rgba(255,255,255,0.1);
            margin-bottom: 2rem;
            flex-wrap: wrap;
        }
        .tab {
            padding: 0.75rem 1.25rem;
            background: transparent;
            border: none;
            color: rgba(255,255,255,0.7);
            cursor: pointer;
            font-size: 0.9rem;
            font-family: inherit;
            border-bottom: 2px solid transparent;
            margin-bottom: -2px;
            transition: all 0.2s;
            white-space: nowrap;
        }
        .tab:hover {
            color: #e2e8f0;
            background: var(--dark-gray, #183446);
        }
        .tab.active {
            color: var(--blue, #0090C1);
            border-bottom-color: var(--blue, #0090C1);
        }

        /* Content panels */
        .panel {
            display: none;
        }
        .panel.active {
            display: block;
        }

        /* Rendered markdown */
        .markdown-body {
            background: var(--dark-gray, #183446);
            border-radius: 12px;
            border: 1px solid rgba(255,255,255,0.1);
            padding: 2rem;
        }
        .markdown-body h1 {
            font-size: 2rem;
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            color: #f8fafc;
        }
        .markdown-body h2 {
            font-size: 1.5rem;
            margin-top: 2rem;
            margin-bottom: 0.75rem;
            color: #f8fafc;
        }
        .markdown-body h3 {
            font-size: 1.25rem;
            margin-top: 1.5rem;
            margin-bottom: 0.5rem;
            color: #f8fafc;
        }
        .markdown-body p {
            margin-bottom: 1rem;
            color: #cbd5e1;
        }
        .markdown-body ul, .markdown-body ol {
            margin-bottom: 1rem;
            padding-left: 1.5rem;
        }
        .markdown-body li {
            margin-bottom: 0.25rem;
            color: #cbd5e1;
        }
        .markdown-body a {
            color: var(--blue, #0090C1);
            text-decoration: none;
        }
        .markdown-body a:hover {
            text-decoration: underline;
        }
        .markdown-body blockquote {
            border-left: 3px solid var(--blue, #0090C1);
            padding-left: 1rem;
            margin-bottom: 1rem;
            color: rgba(255,255,255,0.7);
        }
        .markdown-body code {
            background: var(--dark, #022F40);
            padding: 0.15rem 0.4rem;
            border-radius: 4px;
            font-size: 0.9em;
            color: var(--blue, #0090C1);
        }
        .markdown-body pre {
            background: var(--dark, #022F40);
            padding: 1rem;
            border-radius: 8px;
            overflow-x: auto;
            margin-bottom: 1rem;
        }
        .markdown-body pre code {
            background: none;
            padding: 0;
        }
        .markdown-body strong {
            color: #f8fafc;
        }
        .markdown-body hr {
            border: none;
            border-top: 1px solid rgba(255,255,255,0.1);
            margin: 2rem 0;
        }

        .filename {
            font-size: 0.8rem;
            color: rgba(255,255,255,0.5);
            margin-bottom: 0.75rem;
            font-family: monospace;
        }

        /* Footer */
        .report-footer {
            text-align: center;
            padding: 2rem 0;
            margin-top: 2rem;
            border-top: 1px solid rgba(255,255,255,0.1);
            color: rgba(255,255,255,0.3);
            font-size: 0.85rem;
        }

        @media (max-width: 768px) {
            .report-container { padding: 1rem; }
            header h1 { font-size: 1.8rem; }
            .markdown-body { padding: 1rem; }
        }
    </style>
</head>
<body>
    <div class="report-container">
        <header>
            <div class="logo-row">
                {{.LogoSVG}}
                <span class="logo-text">Optimus</span>
            </div>
            <h1>{{.SkillName}}</h1>
            <div class="site-url">{{.SiteURL}}</div>
            <div class="meta">{{len .Files}} files generated</div>
        </header>

        <div class="tabs">
            {{range $i, $f := .Files}}
            <button class="tab{{if eq $i 0}} active{{end}}" onclick="showTab({{$i}})">{{$f.Filename}}</button>
            {{end}}
        </div>

        {{range $i, $f := .Files}}
        <div class="panel{{if eq $i 0}} active{{end}}" id="panel-{{$i}}">
            <div class="filename">{{$f.Filename}}</div>
            <div class="markdown-body" id="content-{{$i}}"></div>
        </div>
        {{end}}

        <div class="report-footer">
            Generated by <strong>Optimus</strong>
        </div>
    </div>

    <script>
        function showTab(index) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
            document.querySelectorAll('.tab')[index].classList.add('active');
            document.getElementById('panel-' + index).classList.add('active');
        }

        // Render markdown content
        const files = [
            {{range $i, $f := .Files}}
            {index: {{$i}}, content: {{json $f.Content}}},
            {{end}}
        ];

        files.forEach(f => {
            document.getElementById('content-' + f.index).innerHTML = marked.parse(f.content);
        });
    </script>
</body>
</html>`

var scorecardHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Optimus Rank Scorecard — {{.SiteURL}}</title>
    <style>{{.CSS}}</style>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--dark, #022F40);
            color: #e2e8f0;
            line-height: 1.6;
            overflow-x: auto;
        }
        .report-container {
            max-width: 1000px;
            margin: 0 auto;
            padding: 2rem;
        }
        header {
            text-align: center;
            padding: 3rem 0 2rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            margin-bottom: 2rem;
        }
        .logo-row {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 12px;
            margin-bottom: 16px;
        }
        .logo-row svg { width: 36px; height: 36px; }
        .logo-text {
            font-size: 1.5rem;
            font-weight: 700;
            color: #fff;
        }
        header h1 {
            font-size: 2.5rem;
            background: linear-gradient(135deg, var(--primary, #046E8F), var(--blue, #0090C1));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 0.5rem;
        }
        header .site-url { color: rgba(255,255,255,0.7); font-size: 1.1rem; }
        header .meta { color: rgba(255,255,255,0.5); font-size: 0.9rem; margin-top: 0.5rem; }

        /* Overall score gauge */
        .gauge-wrap {
            display: flex;
            justify-content: center;
            margin: 2rem 0 2.5rem;
        }
        .gauge {
            position: relative;
            width: 200px;
            height: 200px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .gauge .score {
            font-size: 3.5rem;
            font-weight: 800;
            z-index: 1;
        }
        .gauge .score-label {
            position: absolute;
            bottom: 38px;
            font-size: 0.85rem;
            color: rgba(255,255,255,0.7);
            z-index: 1;
        }

        /* Section */
        .section { margin-bottom: 2rem; }
        .section h2 {
            font-size: 1.1rem;
            margin-bottom: 1rem;
            color: #f8fafc;
        }

        /* Category meters */
        .meter-row {
            display: flex;
            align-items: center;
            margin-bottom: 0.75rem;
        }
        .meter-label {
            width: 140px;
            font-size: 0.9rem;
            color: rgba(255,255,255,0.7);
            flex-shrink: 0;
        }
        .meter-bar {
            flex: 1;
            height: 24px;
            background: var(--dark-gray, #183446);
            border-radius: 12px;
            overflow: hidden;
            margin: 0 1rem;
        }
        .meter-fill {
            height: 100%;
            border-radius: 12px;
            transition: width 0.5s;
        }
        .meter-value {
            width: 50px;
            text-align: right;
            font-weight: 700;
            font-size: 0.95rem;
        }

        /* Tables */
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 0.5rem;
        }
        th {
            text-align: left;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: rgba(255,255,255,0.5);
            padding: 0.5rem 0.75rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
        }
        td {
            padding: 0.6rem 0.75rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            font-size: 0.9rem;
        }
        tr:last-child td { border-bottom: none; }
        .badge-yes {
            background: rgba(173,238,227,0.15);
            color: var(--mint, #ADEEE3);
            padding: 0.15rem 0.5rem;
            border-radius: 4px;
            font-size: 0.8rem;
            font-weight: 600;
        }
        .badge-no {
            background: #EF444422;
            color: #EF4444;
            padding: 0.15rem 0.5rem;
            border-radius: 4px;
            font-size: 0.8rem;
            font-weight: 600;
        }

        /* Mini meter for page table */
        .mini-meter {
            width: 80px;
            height: 8px;
            background: var(--dark-gray, #183446);
            border-radius: 4px;
            overflow: hidden;
            display: inline-block;
            vertical-align: middle;
            margin-right: 0.5rem;
        }
        .mini-fill {
            height: 100%;
            border-radius: 4px;
        }

        /* Findings */
        .finding {
            padding: 0.5rem 0;
            padding-left: 1.25rem;
            position: relative;
            color: #cbd5e1;
            font-size: 0.9rem;
        }
        .finding::before {
            content: "•";
            position: absolute;
            left: 0;
            color: var(--blue, #0090C1);
            font-weight: bold;
        }

        /* Footer */
        .report-footer {
            text-align: center;
            padding: 2rem 0;
            margin-top: 2rem;
            border-top: 1px solid rgba(255,255,255,0.1);
            color: rgba(255,255,255,0.3);
            font-size: 0.85rem;
        }

        @media (max-width: 768px) {
            .report-container { padding: 1rem; }
            header h1 { font-size: 1.8rem; }
            .gauge { width: 150px; height: 150px; }
            .gauge .score { font-size: 2.5rem; }
            .meter-label { width: 100px; font-size: 0.8rem; }
        }
    </style>
</head>
<body>
    <div class="report-container">
        <header>
            <div class="logo-row">
                {{.LogoSVG}}
                <span class="logo-text">Optimus</span>
            </div>
            <h1>Rank Scorecard</h1>
            <div class="site-url">{{.SiteURL}}</div>
            <div class="meta">Analyzed {{.AnalyzedAt}} · {{.PagesAnalyzed}} pages</div>
        </header>

        <!-- Overall Score Gauge -->
        <div class="gauge-wrap">
            <div class="gauge" style="background: conic-gradient({{scorecardColor .OverallScore}} {{degrees .OverallScore}}deg, {{scorecardColor .OverallScore}}22 {{degrees .OverallScore}}deg); padding: 12px;">
                <div style="background: var(--dark, #022F40); width: 100%; height: 100%; border-radius: 50%; display: flex; flex-direction: column; align-items: center; justify-content: center;">
                    <div class="score" style="color: {{scorecardColor .OverallScore}}">{{.OverallScore}}</div>
                    <div class="score-label">out of 100</div>
                </div>
            </div>
        </div>

        <!-- Category Scores -->
        <div class="section">
            <h2>▸ Score Breakdown</h2>
            <div class="meter-row">
                <span class="meter-label">Search Rank</span>
                <div class="meter-bar"><div class="meter-fill" style="width:{{.CategoryScores.SearchRank}}%; background:{{scorecardColor .CategoryScores.SearchRank}}"></div></div>
                <span class="meter-value" style="color:{{scorecardColor .CategoryScores.SearchRank}}">{{.CategoryScores.SearchRank}}</span>
            </div>
            <div class="meter-row">
                <span class="meter-label">Answer Rank</span>
                <div class="meter-bar"><div class="meter-fill" style="width:{{.CategoryScores.AnswerRank}}%; background:{{scorecardColor .CategoryScores.AnswerRank}}"></div></div>
                <span class="meter-value" style="color:{{scorecardColor .CategoryScores.AnswerRank}}">{{.CategoryScores.AnswerRank}}</span>
            </div>
            <div class="meter-row">
                <span class="meter-label">Technical</span>
                <div class="meter-bar"><div class="meter-fill" style="width:{{.CategoryScores.Technical}}%; background:{{scorecardColor .CategoryScores.Technical}}"></div></div>
                <span class="meter-value" style="color:{{scorecardColor .CategoryScores.Technical}}">{{.CategoryScores.Technical}}</span>
            </div>
            <div class="meter-row">
                <span class="meter-label">Content</span>
                <div class="meter-bar"><div class="meter-fill" style="width:{{.CategoryScores.Content}}%; background:{{scorecardColor .CategoryScores.Content}}"></div></div>
                <span class="meter-value" style="color:{{scorecardColor .CategoryScores.Content}}">{{.CategoryScores.Content}}</span>
            </div>
            <div class="meter-row">
                <span class="meter-label">Structure</span>
                <div class="meter-bar"><div class="meter-fill" style="width:{{.CategoryScores.Structure}}%; background:{{scorecardColor .CategoryScores.Structure}}"></div></div>
                <span class="meter-value" style="color:{{scorecardColor .CategoryScores.Structure}}">{{.CategoryScores.Structure}}</span>
            </div>
        </div>

        {{if .DomainAuth}}
        <!-- Domain Authority -->
        <div class="section">
            <h2>▸ Domain Authority</h2>
            <table>
                <tr><th>Metric</th><th>Value</th></tr>
                {{if .DomainAuth.MozDA}}<tr><td>Moz Domain Authority (DA)</td><td>{{printf "%.0f" .DomainAuth.MozDA}}/100</td></tr>{{end}}
                {{if .DomainAuth.MozPA}}<tr><td>Moz Page Authority (PA)</td><td>{{printf "%.0f" .DomainAuth.MozPA}}/100</td></tr>{{end}}
                <tr><td>Moz Spam Score</td><td>{{printf "%.0f" .DomainAuth.MozSpamScore}}%</td></tr>
                {{if .DomainAuth.LinkingRootDomains}}<tr><td>Linking Root Domains</td><td>{{.DomainAuth.LinkingRootDomains}}</td></tr>{{end}}
                {{if .DomainAuth.AhrefsDR}}<tr><td>Ahrefs Domain Rating (DR)</td><td>{{printf "%.0f" .DomainAuth.AhrefsDR}}/100</td></tr>{{end}}
                {{if .DomainAuth.AhrefsRank}}<tr><td>Ahrefs Rank</td><td>{{.DomainAuth.AhrefsRank}}</td></tr>{{end}}
            </table>
        </div>
        {{end}}

        {{if .BacklinkProfile}}
        <!-- Backlink Profile -->
        <div class="section">
            <h2>▸ Backlink Profile</h2>
            <table>
                <tr><th>Metric</th><th>Value</th></tr>
                <tr><td>Live Backlinks</td><td>{{.BacklinkProfile.LiveBacklinks}}</td></tr>
                <tr><td>Referring Domains</td><td>{{.BacklinkProfile.ReferringDomains}}</td></tr>
                <tr><td>Referring Pages</td><td>{{.BacklinkProfile.ReferringPages}}</td></tr>
            </table>
        </div>
        {{end}}

        {{if .SerpPositions}}
        <!-- Search Positions -->
        <div class="section">
            <h2>▸ Search Positions</h2>
            <table>
                <tr><th>Keyword</th><th>Engine</th><th>Position</th><th>Found</th></tr>
                {{range .SerpPositions}}
                <tr>
                    <td>{{.Keyword}}</td>
                    <td>{{.Engine}}</td>
                    <td>{{if .DomainFound}}#{{.Position}}{{else}}—{{end}}</td>
                    <td>{{if .DomainFound}}<span class="badge-yes">Yes</span>{{else}}<span class="badge-no">No</span>{{end}}</td>
                </tr>
                {{end}}
            </table>
        </div>
        {{end}}

        {{if .AICitations}}
        <!-- AI Citations -->
        <div class="section">
            <h2>▸ AI Citations</h2>
            <table>
                <tr><th>Question</th><th>Cited</th><th>Excerpt</th></tr>
                {{range .AICitations}}
                <tr>
                    <td>{{.Question}}</td>
                    <td>{{if .Cited}}<span class="badge-yes">Cited</span>{{else}}<span class="badge-no">Not cited</span>{{end}}</td>
                    <td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="{{.AnswerExcerpt}}">{{.AnswerExcerpt}}</td>
                </tr>
                {{end}}
            </table>
        </div>
        {{end}}

        {{if .Pages}}
        <!-- Per-Page Scores -->
        <div class="section">
            <h2>▸ Page Scores</h2>
            <table>
                <tr><th>Page</th><th>Search</th><th>Answer</th><th>Keyword</th><th>Words</th><th>Schema</th><th>FAQ</th></tr>
                {{range .Pages}}
                <tr>
                    <td title="{{.URL}}" style="max-width:220px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">{{.Title}}</td>
                    <td><span class="mini-meter"><span class="mini-fill" style="width:{{.SearchReadiness}}%;background:{{scorecardColor .SearchReadiness}}"></span></span>{{.SearchReadiness}}</td>
                    <td><span class="mini-meter"><span class="mini-fill" style="width:{{.AnswerReadiness}}%;background:{{scorecardColor .AnswerReadiness}}"></span></span>{{.AnswerReadiness}}</td>
                    <td>{{.PrimaryKeyword}}</td>
                    <td>{{.WordCount}}</td>
                    <td>{{if .HasSchema}}<span class="badge-yes">Yes</span>{{else}}<span class="badge-no">No</span>{{end}}</td>
                    <td>{{if .HasFAQ}}<span class="badge-yes">Yes</span>{{else}}<span class="badge-no">No</span>{{end}}</td>
                </tr>
                {{end}}
            </table>
        </div>
        {{end}}

        {{if .Findings}}
        <!-- Key Findings -->
        <div class="section">
            <h2>▸ Key Findings</h2>
            {{range .Findings}}
            <div class="finding">{{.}}</div>
            {{end}}
        </div>
        {{end}}

        <div class="report-footer">
            Generated by <strong>Optimus</strong>
        </div>
    </div>
</body>
</html>`

var backlinksHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Backlink Strategy — {{.SiteURL}}</title>
    <style>{{.CSS}}</style>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--dark, #022F40);
            color: #e2e8f0;
            overflow-x: auto;
        }
        .report-container { max-width: 960px; margin: 0 auto; padding: 32px 24px; }

        header {
            text-align: center;
            padding: 2rem 0;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            margin-bottom: 2rem;
        }
        .logo-row {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 12px;
            margin-bottom: 16px;
        }
        .logo-row svg { width: 36px; height: 36px; }
        .logo-text {
            font-size: 1.5rem;
            font-weight: 700;
            color: #fff;
        }
        header h1 {
            font-size: 2rem;
            background: linear-gradient(135deg, var(--primary, #046E8F), var(--blue, #0090C1));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 0.5rem;
        }
        .subtitle { color: rgba(255,255,255,0.5); font-size: 0.9rem; }

        .section { margin-bottom: 32px; }
        .section h2 { font-size: 1.1rem; color: rgba(255,255,255,0.7); margin-bottom: 16px; border-bottom: 1px solid rgba(255,255,255,0.1); padding-bottom: 8px; }

        /* Summary cards */
        .summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 12px; margin-bottom: 32px; }
        .summary-card { background: var(--dark-gray, #183446); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; padding: 16px; text-align: center; }
        .summary-card .value { font-size: 1.8rem; font-weight: 700; color: #fff; }
        .summary-card .label { font-size: 0.75rem; color: rgba(255,255,255,0.5); margin-top: 4px; text-transform: uppercase; letter-spacing: 0.5px; }

        /* Opportunity cards */
        .opp-card { background: var(--dark-gray, #183446); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; padding: 20px; margin-bottom: 12px; }
        .opp-header { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
        .opp-icon { font-size: 1.4rem; }
        .opp-title { font-size: 1rem; font-weight: 600; color: #fff; flex: 1; }
        .opp-num { font-size: 0.75rem; color: rgba(255,255,255,0.35); }
        .opp-tags { display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
        .tag { font-size: 0.7rem; padding: 3px 10px; border-radius: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.3px; }
        .tag-strategy { background: rgba(0,144,193,0.15); color: var(--blue, #0090C1); }
        .opp-desc { color: rgba(255,255,255,0.6); font-size: 0.9rem; line-height: 1.5; margin-bottom: 14px; }
        .opp-target { color: rgba(255,255,255,0.4); font-size: 0.8rem; margin-bottom: 12px; }
        .opp-target a { color: var(--blue, #0090C1); text-decoration: none; }
        .opp-steps { list-style: none; counter-reset: step; }
        .opp-steps li { counter-increment: step; position: relative; padding-left: 28px; margin-bottom: 8px; color: rgba(255,255,255,0.7); font-size: 0.85rem; line-height: 1.4; }
        .opp-steps li::before { content: counter(step); position: absolute; left: 0; top: 0; width: 20px; height: 20px; background: rgba(255,255,255,0.1); border-radius: 50%; text-align: center; line-height: 20px; font-size: 0.7rem; color: rgba(255,255,255,0.5); }

        .report-footer { text-align: center; color: rgba(255,255,255,0.3); font-size: 0.8rem; padding: 32px 0 16px; border-top: 1px solid rgba(255,255,255,0.1); margin-top: 32px; }
    </style>
</head>
<body>
    <div class="report-container">
        <header>
            <div class="logo-row">
                {{.LogoSVG}}
                <span class="logo-text">Optimus</span>
            </div>
            <h1>Backlink Strategy</h1>
            <div class="subtitle">{{.SiteURL}} · {{.AnalyzedAt}} · {{.PagesAnalyzed}} pages analyzed</div>
        </header>

        <!-- Summary -->
        <div class="summary-grid">
            {{if .Summary.CurrentDA}}<div class="summary-card"><div class="value">{{printf "%.0f" .Summary.CurrentDA}}</div><div class="label">Moz DA</div></div>{{end}}
            {{if .Summary.CurrentDR}}<div class="summary-card"><div class="value">{{printf "%.0f" .Summary.CurrentDR}}</div><div class="label">Ahrefs DR</div></div>{{end}}
            {{if .Summary.ReferringDomains}}<div class="summary-card"><div class="value">{{.Summary.ReferringDomains}}</div><div class="label">Referring Domains</div></div>{{end}}
            <div class="summary-card"><div class="value">{{.Summary.TotalOpps}}</div><div class="label">Opportunities</div></div>
            <div class="summary-card"><div class="value" style="color:#ADEEE3">{{.Summary.QuickWins}}</div><div class="label">Quick Wins</div></div>
            <div class="summary-card"><div class="value" style="color:#0090C1">{{.Summary.HighROI}}</div><div class="label">High ROI</div></div>
        </div>

        {{if .Opportunities}}
        <!-- Opportunities -->
        <div class="section">
            <h2>▸ Opportunities</h2>
            {{range $i, $opp := .Opportunities}}
            <div class="opp-card">
                <div class="opp-header">
                    <span class="opp-icon">{{strategyIcon $opp.Strategy}}</span>
                    <span class="opp-title">{{$opp.Title}}</span>
                    <span class="opp-num">#{{add $i 1}}</span>
                </div>
                <div class="opp-tags">
                    <span class="tag tag-strategy">{{$opp.Strategy}}</span>
                    <span class="tag" style="background:{{difficultyTag $opp.Difficulty}}22;color:{{difficultyTag $opp.Difficulty}}">{{$opp.Difficulty}}</span>
                    <span class="tag" style="background:{{impactTag $opp.Impact}}22;color:{{impactTag $opp.Impact}}">{{$opp.Impact}} impact</span>
                </div>
                <div class="opp-desc">{{$opp.Description}}</div>
                {{if $opp.TargetURL}}<div class="opp-target">Target: <a href="{{$opp.TargetURL}}">{{$opp.TargetURL}}</a></div>{{end}}
                {{if $opp.Steps}}
                <ol class="opp-steps">
                    {{range $opp.Steps}}<li>{{.}}</li>{{end}}
                </ol>
                {{end}}
            </div>
            {{end}}
        </div>
        {{end}}

        <div class="report-footer">
            Generated by <strong>Optimus</strong>
        </div>
    </div>
</body>
</html>`
