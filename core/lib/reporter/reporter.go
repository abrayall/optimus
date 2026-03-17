package reporter

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"optimus/core/lib/analyzer"
)

// Config holds reporter configuration
type Config struct {
	Report    *analyzer.Report
	OutputDir string
}

// Result holds reporter output
type Result struct {
	JSONPath string
	HTMLPath string
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
	if err := writeHTML(cfg.Report, htmlPath); err != nil {
		return nil, fmt.Errorf("writing HTML report: %w", err)
	}

	return &Result{
		JSONPath: jsonPath,
		HTMLPath: htmlPath,
	}, nil
}

// writeJSON writes the report as formatted JSON
func writeJSON(report *analyzer.Report, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// writeHTML generates an HTML report
func writeHTML(report *analyzer.Report, path string) error {
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

	return tmpl.Execute(f, report)
}

func priorityColor(priority string) string {
	switch priority {
	case "critical":
		return "#EF4444"
	case "high":
		return "#F97316"
	case "medium":
		return "#F59E0B"
	case "low":
		return "#3B82F6"
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
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0f172a;
            color: #e2e8f0;
            line-height: 1.6;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }
        header {
            text-align: center;
            padding: 3rem 0;
            border-bottom: 1px solid #1e293b;
            margin-bottom: 2rem;
        }
        header h1 {
            font-size: 2.5rem;
            background: linear-gradient(135deg, #f59e0b, #f97316);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            margin-bottom: 0.5rem;
        }
        header .site-url {
            color: #94a3b8;
            font-size: 1.1rem;
        }
        header .meta {
            color: #64748b;
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
            background: #1e293b;
            border-radius: 12px;
            padding: 1.5rem;
            text-align: center;
            border: 1px solid #334155;
        }
        .summary-card .number {
            font-size: 2.5rem;
            font-weight: 700;
        }
        .summary-card .label {
            color: #94a3b8;
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .summary-card.total .number { color: #f59e0b; }
        .summary-card.critical .number { color: #ef4444; }
        .summary-card.high .number { color: #f97316; }
        .summary-card.medium .number { color: #f59e0b; }
        .summary-card.low .number { color: #3b82f6; }

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
            border: 1px solid #334155;
            background: #1e293b;
            color: #94a3b8;
            cursor: pointer;
            font-size: 0.85rem;
            transition: all 0.2s;
        }
        .filter-btn:hover, .filter-btn.active {
            background: #334155;
            color: #f59e0b;
            border-color: #f59e0b;
        }

        /* Recommendations */
        .recommendations {
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }
        .rec-card {
            background: #1e293b;
            border-radius: 12px;
            border: 1px solid #334155;
            overflow: hidden;
            transition: border-color 0.2s;
        }
        .rec-card:hover {
            border-color: #475569;
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
            background: #334155;
            color: #94a3b8;
        }
        .rec-issue {
            flex: 1;
            font-weight: 500;
        }
        .rec-url {
            font-size: 0.8rem;
            color: #64748b;
            max-width: 200px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .rec-details {
            display: none;
            padding: 0 1.5rem 1.5rem;
            border-top: 1px solid #334155;
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
            color: #64748b;
            margin-bottom: 0.5rem;
        }
        .current-text {
            background: #0f172a;
            border: 1px solid #334155;
            border-left: 3px solid #ef4444;
            padding: 0.75rem 1rem;
            border-radius: 6px;
            font-family: monospace;
            font-size: 0.9rem;
            color: #f87171;
            word-break: break-word;
        }
        .suggestion {
            background: #0f172a;
            border: 1px solid #334155;
            border-left: 3px solid #27c93f;
            padding: 0.75rem 1rem;
            border-radius: 6px;
            font-size: 0.9rem;
            color: #4ade80;
            margin-bottom: 0.5rem;
            word-break: break-word;
        }
        .suggestion-number {
            color: #64748b;
            font-size: 0.75rem;
            margin-right: 0.5rem;
        }
        .impact {
            background: #0f172a;
            border: 1px solid #334155;
            border-left: 3px solid #3b82f6;
            padding: 0.75rem 1rem;
            border-radius: 6px;
            font-size: 0.9rem;
            color: #93c5fd;
        }
        .chevron {
            transition: transform 0.2s;
            color: #64748b;
        }
        .rec-card.open .chevron {
            transform: rotate(90deg);
        }

        /* Footer */
        footer {
            text-align: center;
            padding: 2rem 0;
            margin-top: 2rem;
            border-top: 1px solid #1e293b;
            color: #475569;
            font-size: 0.85rem;
        }

        @media (max-width: 768px) {
            .container { padding: 1rem; }
            header h1 { font-size: 1.8rem; }
            .rec-header { flex-wrap: wrap; }
            .rec-url { max-width: 100%; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>⚡ Optimus SEO Report</h1>
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
            <button class="filter-btn" onclick="filterRecs('critical')">🔴 Critical</button>
            <button class="filter-btn" onclick="filterRecs('high')">🟠 High</button>
            <button class="filter-btn" onclick="filterRecs('medium')">🟡 Medium</button>
            <button class="filter-btn" onclick="filterRecs('low')">🔵 Low</button>
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

        <footer>
            Generated by Optimus SEO Analyzer · Powered by AI
        </footer>
    </div>

    <script>
        function toggleRec(header) {
            header.parentElement.classList.toggle('open');
        }

        function filterRecs(priority) {
            document.querySelectorAll('.filter-btn').forEach(btn => btn.classList.remove('active'));
            event.target.classList.add('active');

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
