# Optimus

AI-powered SEO analyzer that scans any website and generates a detailed report of optimization recommendations to improve search rankings, hit rates, and conversions.

## How It Works

Optimus runs in three phases:

1. **Scrape** — Launches headless Chrome to crawl and render pages, capturing the full DOM HTML
2. **Analyze** — Sends the cleaned HTML to an AI engine that performs comprehensive SEO analysis
3. **Report** — Generates JSON and HTML reports with prioritized, actionable recommendations

Each recommendation includes:
- **Priority level** (critical, high, medium, low)
- **Page URL** the issue applies to
- **Category** (title, meta, headings, content, images, links, structured-data, accessibility, etc.)
- **Current text** that needs changing
- **Multiple suggestion options** for the replacement
- **Expected impact** of making the change

## Installation

### Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Google Chrome](https://www.google.com/chrome/) (for headless scraping)
- [Claude Code CLI](https://claude.ai/code) (for AI analysis)

### Build

```bash
git clone <repo-url>
cd optimus
./build.sh
```

The binary will be at `build/optimus`.

### Cross-platform build

```bash
./build.sh v1.0.0 --all
```

Builds for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, and windows/amd64.

## Usage

```bash
# Analyze a single page (default)
optimus example.com

# Crawl multiple pages
optimus example.com --count 10

# Custom timeout for slow sites
optimus example.com --timeout 300

# Add specific analysis focus
optimus example.com -i "focus on local SEO and Google Business keywords"

# Reuse a previous scrape, just re-run analysis
optimus example.com --skip-scrape

# Only scrape, don't analyze yet
optimus example.com --skip-analyze
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--count` | `-c` | `1` | Number of pages to crawl |
| `--depth` | | `3` | Max link-follow depth |
| `--timeout` | `-t` | `120` | Scraping timeout in seconds |
| `--name` | `-n` | *(from URL)* | Site name for the working directory |
| `--instructions` | `-i` | | Custom focus areas for the analysis |
| `--skip-scrape` | | `false` | Skip scraping, reuse existing HTML |
| `--skip-analyze` | | `false` | Skip AI analysis |
| `--url` | | | URL (alternative to positional argument) |

## Output

All output is stored in a temporary directory at:

```
$TMPDIR/optimus/work/<site-name>/
├── scraped/          # Cleaned HTML files from each page
├── report.json       # Machine-readable SEO report
├── report.html       # Interactive HTML report (opens automatically)
└── optimus.log       # Full analysis session log
```

The HTML report features:
- Summary cards with issue counts by priority
- Filterable recommendations by priority level
- Expandable cards showing current text, suggested changes, and expected impact
- Dark theme, fully responsive

## Project Structure

```
optimus/
├── framework/
│   ├── cli/              # CLI entry point and command definitions
│   │   ├── main.go
│   │   └── cmd/
│   │       ├── root.go       # Cobra CLI setup and flags
│   │       └── optimus.go    # Three-phase pipeline orchestration
│   └── server/           # HTTP server (future)
├── core/lib/
│   ├── scraper/          # Headless Chrome site crawler
│   ├── analyzer/         # AI integration for SEO analysis
│   ├── reporter/         # JSON and HTML report generation
│   └── ui/               # Terminal styling and spinners
├── build.sh              # Build script
└── go.mod
```

## Future Work

### Skills Support

Planned analysis skills to extend Optimus beyond on-page SEO recommendations:

- **Keyword Research** — Identify high-value keyword opportunities, search volume analysis, long-tail keyword suggestions, and keyword gap analysis
- **Competitive Analysis** — Compare SEO performance against competitors, identify ranking opportunities, and benchmark content strategies
- **On-page SEO** — Deep analysis of title tags, meta descriptions, heading hierarchy, internal linking, and content structure
- **Content Optimization** — Readability scoring, content gap identification, topic clustering, and content freshness analysis
- **Technical SEO** — Site speed analysis, mobile-friendliness, crawlability, structured data validation, and Core Web Vitals assessment
- **Content Creation** — AI-generated content briefs, meta description drafts, title tag variations, and schema markup generation
