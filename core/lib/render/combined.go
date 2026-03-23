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

// CombinedConfig holds configuration for the combined tabbed report
type CombinedConfig struct {
	SiteURL   string
	Scorecard *engine.Scorecard
	Reports   map[string]*engine.Report          // keyed by skill name: "seo", "aeo", "keywords"
	Backlinks *engine.BacklinkStrategy
	Files     map[string][]engine.FileEntry       // keyed by skill name: "keywords", "blog"
	Errors    map[string]string                   // keyed by skill name
	OutputDir string
	CSS       string
	LogoSVG   string
	Version   string
}

// combinedData is the template data for the combined report
type combinedData struct {
	SiteURL   string
	Scorecard *engine.Scorecard
	Reports   map[string]*engine.Report
	Backlinks *engine.BacklinkStrategy
	Files     map[string][]engine.FileEntry
	Errors    map[string]string
	Tabs      []tabDef
	CSS       template.CSS
	LogoSVG   template.HTML
	Version   string
}

type tabDef struct {
	ID    string
	Label string
}

// GenerateCombined creates a combined tabbed HTML report and JSON summary
func GenerateCombined(cfg CombinedConfig) (*Result, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	jsonPath := filepath.Join(cfg.OutputDir, "report.json")
	htmlPath := filepath.Join(cfg.OutputDir, "report.html")

	// Build JSON summary
	summary := map[string]interface{}{
		"site_url": cfg.SiteURL,
	}
	if cfg.Scorecard != nil {
		summary["scorecard"] = cfg.Scorecard
	}
	for name, report := range cfg.Reports {
		summary[name] = report
	}
	if cfg.Backlinks != nil {
		summary["backlinks"] = cfg.Backlinks
	}
	for name, files := range cfg.Files {
		summary[name+"_files"] = files
	}
	if len(cfg.Errors) > 0 {
		summary["errors"] = cfg.Errors
	}

	jsonData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling combined JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("writing combined JSON: %w", err)
	}

	// Build tab list — only include tabs that have data or an error
	var tabs []tabDef
	skillTabs := []struct {
		id    string
		label string
		has   bool
	}{
		{"summary", "Summary", cfg.Scorecard != nil || cfg.Errors["rank"] != ""},
		{"seo", "SEO", cfg.Reports["seo"] != nil || cfg.Errors["seo"] != ""},
		{"aeo", "AEO", cfg.Reports["aeo"] != nil || cfg.Errors["aeo"] != ""},
		{"keywords", "Keywords", cfg.Files["keywords"] != nil || cfg.Errors["keywords"] != ""},
		{"backlinks", "Backlinks", cfg.Backlinks != nil || cfg.Errors["backlinks"] != ""},
		{"performance", "Performance", cfg.Reports["performance"] != nil || cfg.Errors["performance"] != ""},
		{"blog", "Blog", cfg.Files["blog"] != nil || cfg.Errors["blog"] != ""},
	}
	for _, st := range skillTabs {
		if st.has {
			tabs = append(tabs, tabDef{ID: st.id, Label: st.label})
		}
	}

	data := combinedData{
		SiteURL:   cfg.SiteURL,
		Scorecard: cfg.Scorecard,
		Reports:   cfg.Reports,
		Backlinks: cfg.Backlinks,
		Files:     cfg.Files,
		Errors:    cfg.Errors,
		Tabs:      tabs,
		CSS:       template.CSS(cfg.CSS),
		LogoSVG:   template.HTML(cfg.LogoSVG),
		Version:   cfg.Version,
	}

	tmpl, err := template.New("combined").Funcs(template.FuncMap{
		"priorityColor":  priorityColor,
		"priorityIcon":   priorityIcon,
		"categoryIcon":   categoryIcon,
		"scorecardColor": scorecardColor,
		"scorecardLabel": scorecardLabel,
		"strategyIcon":   strategyIcon,
		"difficultyTag":  difficultyTag,
		"impactTag":      impactTag,
		"upper":          strings.ToUpper,
		"join":           strings.Join,
		"add":            func(a, b int) int { return a + b },
		"degrees":        func(score int) float64 { return float64(score) * 3.6 },
		"hasReport":      func(name string) bool { return cfg.Reports[name] != nil },
		"hasFiles":       func(name string) bool { return cfg.Files[name] != nil },
		"hasError":       func(name string) bool { return cfg.Errors[name] != "" },
		"getError":       func(name string) string { return cfg.Errors[name] },
		"getReport":      func(name string) *engine.Report { return cfg.Reports[name] },
		"getFiles":       func(name string) []engine.FileEntry { return cfg.Files[name] },
		"json": func(s string) template.JS {
			b, _ := json.Marshal(s)
			return template.JS(b)
		},
	}).Parse(combinedHTMLTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing combined template: %w", err)
	}

	f, err := os.Create(htmlPath)
	if err != nil {
		return nil, fmt.Errorf("creating combined HTML: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return nil, fmt.Errorf("executing combined template: %w", err)
	}

	return &Result{
		JSONPath: jsonPath,
		HTMLPath: htmlPath,
	}, nil
}

var combinedHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Optimus Report — {{.SiteURL}}</title>
    <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg fill='%230090C1' viewBox='0 0 32 32' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M25.115 5.265l0.045-1.455c-2.694-1.348-5.686-2.155-8.973-2.155s-6.653 0.807-9.347 2.155l0.045 1.455 9.115 5.981 9.115-5.981zM16 4.556c1.875 0 3.448 0.601 3.448 0.601l-3.448 2.317-3.448-2.317c0 0 1.573-0.601 3.448-0.601zM18.073 23.977v-7.020l0.468-6.035-2.541 1.671-2.479-1.671 0.53 6.035v7.020h4.022zM12.39 10.276l-6.789-4.526-0.045-1.761h-3.996l0.593 8.55 4.85 3.466h5.927l-0.54-5.729zM4.685 10.653l6.412 3.018 0.107 0.989-6.357-2.875-0.162-1.132zM4.415 7.582l6.358 3.071 0.161 1.069-6.411-2.955-0.108-1.185zM4.308 26.549l4.735 2.424v-9.412l-2.796-1.74v-1.078l-2.748-1.886 0.809 11.692zM13.054 24.992v-7.012l-3.071 1.562v10.186l1.384 0.861 1.886-4.598h5.496l1.886 4.598 1.446-0.861v-10.185l-3.071-1.562v7.012h-5.956zM14.168 27.070l-1.401 3.932h6.466l-1.4-3.932h-3.665zM19.071 16.004h5.927l4.85-3.466 0.593-8.55h-3.996l-0.046 1.762-6.79 4.526-0.538 5.728zM27.153 11.785l-6.421 3 0.171-1.114 6.412-3.018-0.162 1.132zM27.477 8.767l-6.412 3.018 0.162-1.132 6.358-3.071-0.108 1.185zM28.5 14.856l-2.747 1.886v1.078l-2.755 1.74v9.412l4.694-2.424 0.808-11.692z'/%3E%3C/svg%3E">
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
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }
        header {
            text-align: center;
            padding: 3rem 0 2rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            margin-bottom: 0;
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

        /* Tab bar */
        .tab-bar {
            display: flex;
            gap: 0;
            border-bottom: 2px solid rgba(255,255,255,0.1);
            margin-bottom: 2rem;
            flex-wrap: wrap;
            background: var(--dark-gray, #183446);
            border-radius: 12px 12px 0 0;
            overflow: hidden;
        }
        .tab-btn {
            padding: 1rem 1.5rem;
            background: transparent;
            border: none;
            color: rgba(255,255,255,0.6);
            cursor: pointer;
            font-size: 0.9rem;
            font-family: inherit;
            font-weight: 500;
            border-bottom: 3px solid transparent;
            margin-bottom: -2px;
            transition: all 0.2s;
            white-space: nowrap;
        }
        .tab-btn:hover {
            color: #e2e8f0;
            background: rgba(255,255,255,0.05);
        }
        .tab-btn.active {
            color: var(--blue, #0090C1);
            border-bottom-color: var(--blue, #0090C1);
            background: rgba(0,144,193,0.08);
        }
        .tab-btn.has-error {
            color: rgba(239,68,68,0.7);
        }
        .tab-btn.has-error.active {
            color: #EF4444;
            border-bottom-color: #EF4444;
        }

        /* Tab panels */
        .tab-panel {
            display: none;
        }
        .tab-panel.active {
            display: block;
        }

        /* Error card */
        .error-card {
            background: rgba(239,68,68,0.1);
            border: 1px solid rgba(239,68,68,0.3);
            border-radius: 12px;
            padding: 2rem;
            text-align: center;
            color: #f87171;
        }
        .error-card h3 {
            font-size: 1.2rem;
            margin-bottom: 0.5rem;
            color: #EF4444;
        }
        .error-card p {
            font-size: 0.9rem;
            color: rgba(248,113,113,0.8);
            word-break: break-word;
        }

        /* ===== Scorecard (Summary tab) ===== */
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

        .section { margin-bottom: 2rem; }
        .section h2 {
            font-size: 1.1rem;
            margin-bottom: 1rem;
            color: #f8fafc;
        }

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
        .finding {
            padding: 0.5rem 0;
            padding-left: 1.25rem;
            position: relative;
            color: #cbd5e1;
            font-size: 0.9rem;
        }
        .finding::before {
            content: "\2022";
            position: absolute;
            left: 0;
            color: var(--blue, #0090C1);
            font-weight: bold;
        }

        /* ===== SEO/AEO Report styles ===== */
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
        .rec-card:hover { border-color: rgba(255,255,255,0.2); }
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
        .rec-issue { flex: 1; font-weight: 500; }
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
        .rec-card.open .rec-details { display: block; }
        .rec-section { margin-top: 1rem; }
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
        .rec-card.open .chevron { transform: rotate(90deg); }

        /* ===== Backlinks styles ===== */
        .summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 12px; margin-bottom: 32px; }
        .bl-summary-card { background: var(--dark-gray, #183446); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; padding: 16px; text-align: center; }
        .bl-summary-card .value { font-size: 1.8rem; font-weight: 700; color: #fff; }
        .bl-summary-card .label { font-size: 0.75rem; color: rgba(255,255,255,0.5); margin-top: 4px; text-transform: uppercase; letter-spacing: 0.5px; }

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

        /* ===== Files (Keywords/Blog) ===== */
        .file-tabs {
            display: flex;
            gap: 0.25rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            margin-bottom: 1.5rem;
            flex-wrap: wrap;
        }
        .file-tab {
            padding: 0.5rem 1rem;
            background: transparent;
            border: none;
            color: rgba(255,255,255,0.6);
            cursor: pointer;
            font-size: 0.85rem;
            font-family: inherit;
            border-bottom: 2px solid transparent;
            margin-bottom: -1px;
            transition: all 0.2s;
        }
        .file-tab:hover { color: #e2e8f0; }
        .file-tab.active {
            color: var(--blue, #0090C1);
            border-bottom-color: var(--blue, #0090C1);
        }
        .file-panel { display: none; }
        .file-panel.active { display: block; }

        .markdown-body {
            background: var(--dark-gray, #183446);
            border-radius: 12px;
            border: 1px solid rgba(255,255,255,0.1);
            padding: 2rem;
        }
        .markdown-body h1 { font-size: 2rem; margin-bottom: 1rem; padding-bottom: 0.5rem; border-bottom: 1px solid rgba(255,255,255,0.1); color: #f8fafc; }
        .markdown-body h2 { font-size: 1.5rem; margin-top: 2rem; margin-bottom: 0.75rem; color: #f8fafc; }
        .markdown-body h3 { font-size: 1.25rem; margin-top: 1.5rem; margin-bottom: 0.5rem; color: #f8fafc; }
        .markdown-body p { margin-bottom: 1rem; color: #cbd5e1; }
        .markdown-body ul, .markdown-body ol { margin-bottom: 1rem; padding-left: 1.5rem; }
        .markdown-body li { margin-bottom: 0.25rem; color: #cbd5e1; }
        .markdown-body a { color: var(--blue, #0090C1); text-decoration: none; }
        .markdown-body a:hover { text-decoration: underline; }
        .markdown-body blockquote { border-left: 3px solid var(--blue, #0090C1); padding-left: 1rem; margin-bottom: 1rem; color: rgba(255,255,255,0.7); }
        .markdown-body code { background: var(--dark, #022F40); padding: 0.15rem 0.4rem; border-radius: 4px; font-size: 0.9em; color: var(--blue, #0090C1); }
        .markdown-body pre { background: var(--dark, #022F40); padding: 1rem; border-radius: 8px; overflow-x: auto; margin-bottom: 1rem; }
        .markdown-body pre code { background: none; padding: 0; }
        .markdown-body strong { color: #f8fafc; }
        .markdown-body hr { border: none; border-top: 1px solid rgba(255,255,255,0.1); margin: 2rem 0; }
        .filename { font-size: 0.8rem; color: rgba(255,255,255,0.5); margin-bottom: 0.75rem; font-family: monospace; }

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
            .tab-btn { padding: 0.75rem 1rem; font-size: 0.8rem; }
            .rec-header { flex-wrap: wrap; }
            .rec-url { max-width: 100%; }
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
            <h1>Site Report</h1>
            <div class="site-url">{{.SiteURL}}</div>
            {{if .Scorecard}}<div class="meta">Analyzed {{.Scorecard.AnalyzedAt}} · {{.Scorecard.PagesAnalyzed}} pages{{if .Version}} · v{{.Version}}{{end}}</div>{{end}}
        </header>

        <!-- Tab Bar -->
        <div class="tab-bar">
            {{range $i, $tab := .Tabs}}
            <button class="tab-btn{{if eq $i 0}} active{{end}}{{if hasError $tab.ID}} has-error{{end}}" onclick="switchTab('{{$tab.ID}}')" data-tab="{{$tab.ID}}">{{$tab.Label}}</button>
            {{end}}
        </div>

        <!-- ===== Summary (Rank Scorecard) Tab ===== -->
        {{if or .Scorecard (hasError "rank")}}
        <div class="tab-panel{{if eq (index .Tabs 0).ID "summary"}} active{{end}}" id="tab-summary">
            {{if hasError "rank"}}
            <div class="error-card">
                <h3>Rank Analysis Failed</h3>
                <p>{{getError "rank"}}</p>
            </div>
            {{else}}
            <div class="gauge-wrap">
                <div class="gauge" style="background: conic-gradient({{scorecardColor .Scorecard.OverallScore}} {{degrees .Scorecard.OverallScore}}deg, {{scorecardColor .Scorecard.OverallScore}}22 {{degrees .Scorecard.OverallScore}}deg); padding: 12px;">
                    <div style="background: var(--dark, #022F40); width: 100%; height: 100%; border-radius: 50%; display: flex; flex-direction: column; align-items: center; justify-content: center;">
                        <div class="score" style="color: {{scorecardColor .Scorecard.OverallScore}}">{{.Scorecard.OverallScore}}</div>
                        <div class="score-label">out of 100</div>
                    </div>
                </div>
            </div>

            <div class="section">
                <h2>Score Breakdown</h2>
                <div class="meter-row">
                    <span class="meter-label">Search Rank</span>
                    <div class="meter-bar"><div class="meter-fill" style="width:{{.Scorecard.CategoryScores.SearchRank}}%; background:{{scorecardColor .Scorecard.CategoryScores.SearchRank}}"></div></div>
                    <span class="meter-value" style="color:{{scorecardColor .Scorecard.CategoryScores.SearchRank}}">{{.Scorecard.CategoryScores.SearchRank}}</span>
                </div>
                <div class="meter-row">
                    <span class="meter-label">Answer Rank</span>
                    <div class="meter-bar"><div class="meter-fill" style="width:{{.Scorecard.CategoryScores.AnswerRank}}%; background:{{scorecardColor .Scorecard.CategoryScores.AnswerRank}}"></div></div>
                    <span class="meter-value" style="color:{{scorecardColor .Scorecard.CategoryScores.AnswerRank}}">{{.Scorecard.CategoryScores.AnswerRank}}</span>
                </div>
                <div class="meter-row">
                    <span class="meter-label">Technical</span>
                    <div class="meter-bar"><div class="meter-fill" style="width:{{.Scorecard.CategoryScores.Technical}}%; background:{{scorecardColor .Scorecard.CategoryScores.Technical}}"></div></div>
                    <span class="meter-value" style="color:{{scorecardColor .Scorecard.CategoryScores.Technical}}">{{.Scorecard.CategoryScores.Technical}}</span>
                </div>
                <div class="meter-row">
                    <span class="meter-label">Content</span>
                    <div class="meter-bar"><div class="meter-fill" style="width:{{.Scorecard.CategoryScores.Content}}%; background:{{scorecardColor .Scorecard.CategoryScores.Content}}"></div></div>
                    <span class="meter-value" style="color:{{scorecardColor .Scorecard.CategoryScores.Content}}">{{.Scorecard.CategoryScores.Content}}</span>
                </div>
                <div class="meter-row">
                    <span class="meter-label">Structure</span>
                    <div class="meter-bar"><div class="meter-fill" style="width:{{.Scorecard.CategoryScores.Structure}}%; background:{{scorecardColor .Scorecard.CategoryScores.Structure}}"></div></div>
                    <span class="meter-value" style="color:{{scorecardColor .Scorecard.CategoryScores.Structure}}">{{.Scorecard.CategoryScores.Structure}}</span>
                </div>
            </div>

            {{if .Scorecard.DomainAuth}}
            <div class="section">
                <h2>Domain Authority</h2>
                <table>
                    <tr><th>Metric</th><th>Value</th></tr>
                    {{if .Scorecard.DomainAuth.MozDA}}<tr><td>Moz Domain Authority (DA)</td><td>{{printf "%.0f" .Scorecard.DomainAuth.MozDA}}/100</td></tr>{{end}}
                    {{if .Scorecard.DomainAuth.MozPA}}<tr><td>Moz Page Authority (PA)</td><td>{{printf "%.0f" .Scorecard.DomainAuth.MozPA}}/100</td></tr>{{end}}
                    <tr><td>Moz Spam Score</td><td>{{printf "%.0f" .Scorecard.DomainAuth.MozSpamScore}}%</td></tr>
                    {{if .Scorecard.DomainAuth.LinkingRootDomains}}<tr><td>Linking Root Domains</td><td>{{.Scorecard.DomainAuth.LinkingRootDomains}}</td></tr>{{end}}
                    {{if .Scorecard.DomainAuth.AhrefsDR}}<tr><td>Ahrefs Domain Rating (DR)</td><td>{{printf "%.0f" .Scorecard.DomainAuth.AhrefsDR}}/100</td></tr>{{end}}
                    {{if .Scorecard.DomainAuth.AhrefsRank}}<tr><td>Ahrefs Rank</td><td>{{.Scorecard.DomainAuth.AhrefsRank}}</td></tr>{{end}}
                </table>
            </div>
            {{end}}

            {{if .Scorecard.BacklinkProfile}}
            <div class="section">
                <h2>Backlink Profile</h2>
                <table>
                    <tr><th>Metric</th><th>Value</th></tr>
                    <tr><td>Live Backlinks</td><td>{{.Scorecard.BacklinkProfile.LiveBacklinks}}</td></tr>
                    <tr><td>Referring Domains</td><td>{{.Scorecard.BacklinkProfile.ReferringDomains}}</td></tr>
                    <tr><td>Referring Pages</td><td>{{.Scorecard.BacklinkProfile.ReferringPages}}</td></tr>
                </table>
            </div>
            {{end}}

            {{if .Scorecard.SerpPositions}}
            <div class="section">
                <h2>Search Positions</h2>
                <table>
                    <tr><th>Keyword</th><th>Engine</th><th>Position</th><th>Found</th></tr>
                    {{range .Scorecard.SerpPositions}}
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

            {{if .Scorecard.AICitations}}
            <div class="section">
                <h2>AI Citations</h2>
                <table>
                    <tr><th>Question</th><th>Cited</th><th>Excerpt</th></tr>
                    {{range .Scorecard.AICitations}}
                    <tr>
                        <td>{{.Question}}</td>
                        <td>{{if .Cited}}<span class="badge-yes">Cited</span>{{else}}<span class="badge-no">Not cited</span>{{end}}</td>
                        <td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="{{.AnswerExcerpt}}">{{.AnswerExcerpt}}</td>
                    </tr>
                    {{end}}
                </table>
            </div>
            {{end}}

            {{if .Scorecard.Pages}}
            <div class="section">
                <h2>Page Scores</h2>
                <table>
                    <tr><th>Page</th><th>Search</th><th>Answer</th><th>Keyword</th><th>Words</th><th>Schema</th><th>FAQ</th></tr>
                    {{range .Scorecard.Pages}}
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

            {{if .Scorecard.Findings}}
            <div class="section">
                <h2>Key Findings</h2>
                {{range .Scorecard.Findings}}
                <div class="finding">{{.}}</div>
                {{end}}
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}

        <!-- ===== SEO Tab ===== -->
        {{if or (hasReport "seo") (hasError "seo")}}
        <div class="tab-panel" id="tab-seo">
            {{if hasError "seo"}}
            <div class="error-card">
                <h3>SEO Analysis Failed</h3>
                <p>{{getError "seo"}}</p>
            </div>
            {{else}}
            {{with getReport "seo"}}
            <div class="summary">
                <div class="summary-card total"><div class="number">{{.Summary.TotalIssues}}</div><div class="label">Total Issues</div></div>
                <div class="summary-card critical"><div class="number">{{.Summary.CriticalCount}}</div><div class="label">Critical</div></div>
                <div class="summary-card high"><div class="number">{{.Summary.HighCount}}</div><div class="label">High</div></div>
                <div class="summary-card medium"><div class="number">{{.Summary.MediumCount}}</div><div class="label">Medium</div></div>
                <div class="summary-card low"><div class="number">{{.Summary.LowCount}}</div><div class="label">Low</div></div>
            </div>
            <div class="filters">
                <button class="filter-btn active" onclick="filterRecs('all', 'seo')">All</button>
                <button class="filter-btn" onclick="filterRecs('critical', 'seo')"><span class="filter-dot" style="background:#EF4444"></span>Critical</button>
                <button class="filter-btn" onclick="filterRecs('high', 'seo')"><span class="filter-dot" style="background:#F97316"></span>High</button>
                <button class="filter-btn" onclick="filterRecs('medium', 'seo')"><span class="filter-dot" style="background:#0090C1"></span>Medium</button>
                <button class="filter-btn" onclick="filterRecs('low', 'seo')"><span class="filter-dot" style="background:#ADEEE3"></span>Low</button>
            </div>
            <div class="recommendations" id="recs-seo">
                {{range $i, $rec := .Recommendations}}
                <div class="rec-card" data-priority="{{$rec.Priority}}">
                    <div class="rec-header" onclick="toggleRec(this)">
                        <span class="chevron">&#9654;</span>
                        <div class="rec-tags">
                            <span class="rec-priority" style="background: {{priorityColor $rec.Priority}}22; color: {{priorityColor $rec.Priority}}">{{upper $rec.Priority}}</span>
                            <span class="rec-category">{{$rec.Category}}</span>
                        </div>
                        <span class="rec-issue">{{$rec.Issue}}</span>
                        <span class="rec-url" title="{{$rec.URL}}">{{$rec.URL}}</span>
                    </div>
                    <div class="rec-details">
                        {{if $rec.CurrentText}}<div class="rec-section"><div class="rec-section-title">Current</div><div class="current-text">{{$rec.CurrentText}}</div></div>{{end}}
                        {{if $rec.Suggestions}}<div class="rec-section"><div class="rec-section-title">Suggested Changes</div>{{range $j, $sug := $rec.Suggestions}}<div class="suggestion"><span class="suggestion-number">Option {{add $j 1}}:</span>{{$sug}}</div>{{end}}</div>{{end}}
                        {{if $rec.Impact}}<div class="rec-section"><div class="rec-section-title">Expected Impact</div><div class="impact">{{$rec.Impact}}</div></div>{{end}}
                    </div>
                </div>
                {{end}}
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}

        <!-- ===== AEO Tab ===== -->
        {{if or (hasReport "aeo") (hasError "aeo")}}
        <div class="tab-panel" id="tab-aeo">
            {{if hasError "aeo"}}
            <div class="error-card">
                <h3>AEO Analysis Failed</h3>
                <p>{{getError "aeo"}}</p>
            </div>
            {{else}}
            {{with getReport "aeo"}}
            <div class="summary">
                <div class="summary-card total"><div class="number">{{.Summary.TotalIssues}}</div><div class="label">Total Issues</div></div>
                <div class="summary-card critical"><div class="number">{{.Summary.CriticalCount}}</div><div class="label">Critical</div></div>
                <div class="summary-card high"><div class="number">{{.Summary.HighCount}}</div><div class="label">High</div></div>
                <div class="summary-card medium"><div class="number">{{.Summary.MediumCount}}</div><div class="label">Medium</div></div>
                <div class="summary-card low"><div class="number">{{.Summary.LowCount}}</div><div class="label">Low</div></div>
            </div>
            <div class="filters">
                <button class="filter-btn active" onclick="filterRecs('all', 'aeo')">All</button>
                <button class="filter-btn" onclick="filterRecs('critical', 'aeo')"><span class="filter-dot" style="background:#EF4444"></span>Critical</button>
                <button class="filter-btn" onclick="filterRecs('high', 'aeo')"><span class="filter-dot" style="background:#F97316"></span>High</button>
                <button class="filter-btn" onclick="filterRecs('medium', 'aeo')"><span class="filter-dot" style="background:#0090C1"></span>Medium</button>
                <button class="filter-btn" onclick="filterRecs('low', 'aeo')"><span class="filter-dot" style="background:#ADEEE3"></span>Low</button>
            </div>
            <div class="recommendations" id="recs-aeo">
                {{range $i, $rec := .Recommendations}}
                <div class="rec-card" data-priority="{{$rec.Priority}}">
                    <div class="rec-header" onclick="toggleRec(this)">
                        <span class="chevron">&#9654;</span>
                        <div class="rec-tags">
                            <span class="rec-priority" style="background: {{priorityColor $rec.Priority}}22; color: {{priorityColor $rec.Priority}}">{{upper $rec.Priority}}</span>
                            <span class="rec-category">{{$rec.Category}}</span>
                        </div>
                        <span class="rec-issue">{{$rec.Issue}}</span>
                        <span class="rec-url" title="{{$rec.URL}}">{{$rec.URL}}</span>
                    </div>
                    <div class="rec-details">
                        {{if $rec.CurrentText}}<div class="rec-section"><div class="rec-section-title">Current</div><div class="current-text">{{$rec.CurrentText}}</div></div>{{end}}
                        {{if $rec.Suggestions}}<div class="rec-section"><div class="rec-section-title">Suggested Changes</div>{{range $j, $sug := $rec.Suggestions}}<div class="suggestion"><span class="suggestion-number">Option {{add $j 1}}:</span>{{$sug}}</div>{{end}}</div>{{end}}
                        {{if $rec.Impact}}<div class="rec-section"><div class="rec-section-title">Expected Impact</div><div class="impact">{{$rec.Impact}}</div></div>{{end}}
                    </div>
                </div>
                {{end}}
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}

        <!-- ===== Keywords Tab ===== -->
        {{if or (hasFiles "keywords") (hasError "keywords")}}
        <div class="tab-panel" id="tab-keywords">
            {{if hasError "keywords"}}
            <div class="error-card">
                <h3>Keywords Analysis Failed</h3>
                <p>{{getError "keywords"}}</p>
            </div>
            {{else}}
            <div class="file-tabs">
                {{range $i, $f := getFiles "keywords"}}
                <button class="file-tab{{if eq $i 0}} active{{end}}" onclick="showFileTab(this, 'keywords', {{$i}})">{{$f.Filename}}</button>
                {{end}}
            </div>
            {{range $i, $f := getFiles "keywords"}}
            <div class="file-panel{{if eq $i 0}} active{{end}}" id="file-keywords-{{$i}}">
                <div class="filename">{{$f.Filename}}</div>
                <div class="markdown-body" id="md-keywords-{{$i}}"></div>
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}

        <!-- ===== Backlinks Tab ===== -->
        {{if or .Backlinks (hasError "backlinks")}}
        <div class="tab-panel" id="tab-backlinks">
            {{if hasError "backlinks"}}
            <div class="error-card">
                <h3>Backlinks Analysis Failed</h3>
                <p>{{getError "backlinks"}}</p>
            </div>
            {{else}}
            <div class="summary-grid">
                {{if .Backlinks.Summary.CurrentDA}}<div class="bl-summary-card"><div class="value">{{printf "%.0f" .Backlinks.Summary.CurrentDA}}</div><div class="label">Moz DA</div></div>{{end}}
                {{if .Backlinks.Summary.CurrentDR}}<div class="bl-summary-card"><div class="value">{{printf "%.0f" .Backlinks.Summary.CurrentDR}}</div><div class="label">Ahrefs DR</div></div>{{end}}
                {{if .Backlinks.Summary.ReferringDomains}}<div class="bl-summary-card"><div class="value">{{.Backlinks.Summary.ReferringDomains}}</div><div class="label">Referring Domains</div></div>{{end}}
                <div class="bl-summary-card"><div class="value">{{.Backlinks.Summary.TotalOpps}}</div><div class="label">Opportunities</div></div>
                <div class="bl-summary-card"><div class="value" style="color:#ADEEE3">{{.Backlinks.Summary.QuickWins}}</div><div class="label">Quick Wins</div></div>
                <div class="bl-summary-card"><div class="value" style="color:#0090C1">{{.Backlinks.Summary.HighROI}}</div><div class="label">High ROI</div></div>
            </div>

            {{if .Backlinks.Opportunities}}
            <div class="section">
                <h2>Opportunities</h2>
                {{range $i, $opp := .Backlinks.Opportunities}}
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
                    {{if $opp.Steps}}<ol class="opp-steps">{{range $opp.Steps}}<li>{{.}}</li>{{end}}</ol>{{end}}
                </div>
                {{end}}
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}

        <!-- ===== Performance Tab ===== -->
        {{if or (hasReport "performance") (hasError "performance")}}
        <div class="tab-panel" id="tab-performance">
            {{if hasError "performance"}}
            <div class="error-card">
                <h3>Performance Analysis Failed</h3>
                <p>{{getError "performance"}}</p>
            </div>
            {{else}}
            {{with getReport "performance"}}
            <div class="summary">
                <div class="summary-card total"><div class="number">{{.Summary.TotalIssues}}</div><div class="label">Total Issues</div></div>
                <div class="summary-card critical"><div class="number">{{.Summary.CriticalCount}}</div><div class="label">Critical</div></div>
                <div class="summary-card high"><div class="number">{{.Summary.HighCount}}</div><div class="label">High</div></div>
                <div class="summary-card medium"><div class="number">{{.Summary.MediumCount}}</div><div class="label">Medium</div></div>
                <div class="summary-card low"><div class="number">{{.Summary.LowCount}}</div><div class="label">Low</div></div>
            </div>
            <div class="filters">
                <button class="filter-btn active" onclick="filterRecs('all', 'performance')">All</button>
                <button class="filter-btn" onclick="filterRecs('critical', 'performance')"><span class="filter-dot" style="background:#EF4444"></span>Critical</button>
                <button class="filter-btn" onclick="filterRecs('high', 'performance')"><span class="filter-dot" style="background:#F97316"></span>High</button>
                <button class="filter-btn" onclick="filterRecs('medium', 'performance')"><span class="filter-dot" style="background:#0090C1"></span>Medium</button>
                <button class="filter-btn" onclick="filterRecs('low', 'performance')"><span class="filter-dot" style="background:#ADEEE3"></span>Low</button>
            </div>
            <div class="recommendations" id="recs-performance">
                {{range $i, $rec := .Recommendations}}
                <div class="rec-card" data-priority="{{$rec.Priority}}">
                    <div class="rec-header" onclick="toggleRec(this)">
                        <span class="chevron">&#9654;</span>
                        <div class="rec-tags">
                            <span class="rec-priority" style="background: {{priorityColor $rec.Priority}}22; color: {{priorityColor $rec.Priority}}">{{upper $rec.Priority}}</span>
                            <span class="rec-category">{{$rec.Category}}</span>
                        </div>
                        <span class="rec-issue">{{$rec.Issue}}</span>
                        <span class="rec-url" title="{{$rec.URL}}">{{$rec.URL}}</span>
                    </div>
                    <div class="rec-details">
                        {{if $rec.CurrentText}}<div class="rec-section"><div class="rec-section-title">Current</div><div class="current-text">{{$rec.CurrentText}}</div></div>{{end}}
                        {{if $rec.Suggestions}}<div class="rec-section"><div class="rec-section-title">Suggested Changes</div>{{range $j, $sug := $rec.Suggestions}}<div class="suggestion"><span class="suggestion-number">Option {{add $j 1}}:</span>{{$sug}}</div>{{end}}</div>{{end}}
                        {{if $rec.Impact}}<div class="rec-section"><div class="rec-section-title">Expected Impact</div><div class="impact">{{$rec.Impact}}</div></div>{{end}}
                    </div>
                </div>
                {{end}}
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}

        <!-- ===== Blog Tab ===== -->
        {{if or (hasFiles "blog") (hasError "blog")}}
        <div class="tab-panel" id="tab-blog">
            {{if hasError "blog"}}
            <div class="error-card">
                <h3>Blog Content Generation Failed</h3>
                <p>{{getError "blog"}}</p>
            </div>
            {{else}}
            <div class="file-tabs">
                {{range $i, $f := getFiles "blog"}}
                <button class="file-tab{{if eq $i 0}} active{{end}}" onclick="showFileTab(this, 'blog', {{$i}})">{{$f.Filename}}</button>
                {{end}}
            </div>
            {{range $i, $f := getFiles "blog"}}
            <div class="file-panel{{if eq $i 0}} active{{end}}" id="file-blog-{{$i}}">
                <div class="filename">{{$f.Filename}}</div>
                <div class="markdown-body" id="md-blog-{{$i}}"></div>
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}

        <div class="report-footer">
            Generated by <strong>Optimus</strong>{{if .Version}} <span style="opacity:0.5">v{{.Version}}</span>{{end}}
        </div>
    </div>

    <script>
        // Tab switching
        function switchTab(tabId) {
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
            document.querySelector('.tab-btn[data-tab="' + tabId + '"]').classList.add('active');
            document.getElementById('tab-' + tabId).classList.add('active');
        }

        // Recommendation toggle
        function toggleRec(header) {
            header.parentElement.classList.toggle('open');
        }

        // Recommendation filter (scoped to a panel)
        function filterRecs(priority, panel) {
            var container = document.getElementById('recs-' + panel);
            if (!container) return;
            var btns = container.previousElementSibling;
            while (btns && !btns.classList.contains('filters')) btns = btns.previousElementSibling;
            if (btns) btns.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
            event.target.closest('.filter-btn').classList.add('active');
            container.querySelectorAll('.rec-card').forEach(function(card) {
                card.style.display = (priority === 'all' || card.dataset.priority === priority) ? 'block' : 'none';
            });
        }

        // File sub-tabs
        function showFileTab(btn, group, index) {
            var panel = btn.closest('.tab-panel');
            panel.querySelectorAll('.file-tab').forEach(t => t.classList.remove('active'));
            panel.querySelectorAll('.file-panel').forEach(p => p.classList.remove('active'));
            btn.classList.add('active');
            document.getElementById('file-' + group + '-' + index).classList.add('active');
        }

        // Expand critical issues
        document.querySelectorAll('.rec-card[data-priority="critical"]').forEach(function(card) {
            card.classList.add('open');
        });

        // Render markdown files
        if (typeof marked !== 'undefined') {
            var mdFiles = [
                {{if hasFiles "keywords"}}{{range $i, $f := getFiles "keywords"}}
                {id: 'md-keywords-{{$i}}', content: {{json $f.Content}}},
                {{end}}{{end}}
                {{if hasFiles "blog"}}{{range $i, $f := getFiles "blog"}}
                {id: 'md-blog-{{$i}}', content: {{json $f.Content}}},
                {{end}}{{end}}
            ];
            mdFiles.forEach(function(f) {
                var el = document.getElementById(f.id);
                if (el) el.innerHTML = marked.parse(f.content);
            });
        }
    </script>
</body>
</html>`
