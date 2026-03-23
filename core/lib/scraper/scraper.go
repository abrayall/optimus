package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"optimus/core/lib/ui"

	"github.com/chromedp/chromedp"
	"golang.org/x/net/html"
)

// Config holds scraper configuration
type Config struct {
	URL       string
	OutputDir string
	Timeout   time.Duration
	MaxPages  int
	MaxDepth  int
}

// PageTiming holds Navigation Timing API metrics captured during scraping
type PageTiming struct {
	TTFB        time.Duration // responseStart - fetchStart
	DOMReady    time.Duration // domInteractive - fetchStart
	DOMComplete time.Duration // domComplete - fetchStart
	FullLoad    time.Duration // loadEventEnd - fetchStart
	TotalTime   time.Duration // Go-level wall clock for the whole scrape
}

// PageResult holds the scraped data for a single page
type PageResult struct {
	URL       string
	Title     string
	HTML      string
	CleanHTML string
	FilePath  string // path to saved content file
	Timing    *PageTiming
}

// Result holds scraper output
type Result struct {
	Pages     []PageResult
	OutputDir string
}

// Scrape crawls a website starting from the given URL, rendering each page with headless Chrome
func Scrape(cfg Config) (*Result, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}
	if cfg.MaxPages == 0 {
		cfg.MaxPages = 50
	}
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 3
	}

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	sp := ui.NewSpinner("Launching browser...")

	// Parse base URL to determine domain boundaries
	baseURL, err := url.Parse(cfg.URL)
	if err != nil {
		sp.Finish()
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	// Per-page timeout (overall timeout divided across pages, with a minimum)
	perPageTimeout := cfg.Timeout / time.Duration(cfg.MaxPages)
	if perPageTimeout < 30*time.Second {
		perPageTimeout = 30 * time.Second
	}

	// Launch headless Chrome — block images/fonts/media via flags for speed
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-web-security", true),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("blink-settings", "imagesEnabled=false"),
			chromedp.Flag("disable-remote-fonts", true),
			chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
			chromedp.WindowSize(1920, 1080),
		)...,
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// BFS crawl
	visited := make(map[string]bool)
	type queueItem struct {
		url   string
		depth int
	}
	queue := []queueItem{{url: cfg.URL, depth: 0}}
	var pages []PageResult

	for len(queue) > 0 && len(pages) < cfg.MaxPages {
		item := queue[0]
		queue = queue[1:]

		// Normalize URL for dedup
		normalizedURL := normalizeURL(item.url)
		if visited[normalizedURL] {
			continue
		}
		visited[normalizedURL] = true

		sp.Update(fmt.Sprintf("Scraping page %d: %s", len(pages)+1, truncateURL(item.url, 40)))

		// Scrape this page with its own timeout
		pageCtx, pageCancel := context.WithTimeout(ctx, perPageTimeout)
		page, links, err := scrapePage(pageCtx, item.url)
		pageCancel()
		if err != nil {
			sp.Finish()
			sp = nil
			fmt.Print("  ")
			ui.PrintWarning("Failed to scrape %s: %s", truncateURL(item.url, 40), err)
			sp = ui.NewSpinner("Scraping...")
			continue
		}

		// Save page content
		safeFilename := urlToFilename(item.url, baseURL)
		contentPath := filepath.Join(cfg.OutputDir, safeFilename)
		if err := os.WriteFile(contentPath, []byte(page.CleanHTML), 0644); err != nil {
			continue
		}
		page.FilePath = contentPath
		pages = append(pages, *page)

		sp.Finish()
		sp = nil
		fmt.Print("  ")
		ui.PrintInfo("Scraped: %s (%s)", page.Title, truncateURL(item.url, 50))

		// Enqueue discovered links (same domain only)
		if item.depth < cfg.MaxDepth {
			for _, link := range links {
				linkURL, err := url.Parse(link)
				if err != nil {
					continue
				}
				resolved := baseURL.ResolveReference(linkURL)

				// Same domain or sibling subdomain check
				if !isSameDomain(resolved.Hostname(), baseURL.Hostname()) {
					continue
				}

				// Skip non-page resources
				if isResourceURL(resolved.Path) {
					continue
				}

				resolvedStr := resolved.String()
				if !visited[normalizeURL(resolvedStr)] {
					queue = append(queue, queueItem{url: resolvedStr, depth: item.depth + 1})
				}
			}
		}

		// Only restart spinner if there's more work
		if len(queue) > 0 && len(pages) < cfg.MaxPages {
			sp = ui.NewSpinner("Scraping...")
		}
	}

	if sp != nil {
		sp.Finish()
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages could be scraped from %s", cfg.URL)
	}

	return &Result{
		Pages:     pages,
		OutputDir: cfg.OutputDir,
	}, nil
}

// scrapePage renders a single page and extracts its content and links
func scrapePage(ctx context.Context, pageURL string) (*PageResult, []string, error) {
	var renderedHTML string
	var title string

	wallStart := time.Now()

	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2*time.Second),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &renderedHTML),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("rendering %s: %w", pageURL, err)
	}

	// Capture Navigation Timing API metrics
	var timingJSON string
	if err2 := chromedp.Run(ctx,
		chromedp.Evaluate(`JSON.stringify((function(){
			var e = performance.getEntriesByType('navigation')[0];
			if (!e) return null;
			return {
				ttfb: e.responseStart - e.fetchStart,
				domReady: e.domInteractive - e.fetchStart,
				domComplete: e.domComplete - e.fetchStart,
				fullLoad: e.loadEventEnd - e.fetchStart
			};
		})())`, &timingJSON),
	); err2 == nil {
		// best-effort — ignore errors
	}

	wallElapsed := time.Since(wallStart)

	timing := &PageTiming{TotalTime: wallElapsed}
	if timingJSON != "" && timingJSON != "null" {
		var raw struct {
			TTFB        float64 `json:"ttfb"`
			DOMReady    float64 `json:"domReady"`
			DOMComplete float64 `json:"domComplete"`
			FullLoad    float64 `json:"fullLoad"`
		}
		if err2 := json.Unmarshal([]byte(timingJSON), &raw); err2 == nil {
			timing.TTFB = time.Duration(raw.TTFB * float64(time.Millisecond))
			timing.DOMReady = time.Duration(raw.DOMReady * float64(time.Millisecond))
			timing.DOMComplete = time.Duration(raw.DOMComplete * float64(time.Millisecond))
			timing.FullLoad = time.Duration(raw.FullLoad * float64(time.Millisecond))
		}
	}

	if title == "" {
		title = pageURL
	}

	// Extract links for crawling
	links := extractLinks(renderedHTML)

	// Clean HTML for analysis
	cleanedHTML := cleanHTML(renderedHTML)

	return &PageResult{
		URL:       pageURL,
		Title:     title,
		HTML:      renderedHTML,
		CleanHTML: cleanedHTML,
		Timing:    timing,
	}, links, nil
}

// extractLinks pulls all href values from anchor tags
func extractLinks(htmlContent string) []string {
	var links []string
	seen := make(map[string]bool)

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return links
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && attr.Val != "" {
					href := strings.TrimSpace(attr.Val)
					// Skip javascript:, mailto:, tel:, #anchors
					if strings.HasPrefix(href, "javascript:") ||
						strings.HasPrefix(href, "mailto:") ||
						strings.HasPrefix(href, "tel:") ||
						href == "#" {
						continue
					}
					if !seen[href] {
						seen[href] = true
						links = append(links, href)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return links
}

// cleanHTML strips scripts, styles, and noise attributes to produce content-focused HTML
func cleanHTML(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return rawHTML
	}

	removeElements(doc)
	cleanAttributes(doc)

	var buf strings.Builder
	html.Render(&buf, doc)
	return buf.String()
}

// removeElements removes script, noscript, style, and iframe elements
func removeElements(n *html.Node) {
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		if c.Type == html.ElementNode {
			switch c.Data {
			case "script", "noscript", "iframe", "style", "svg":
				n.RemoveChild(c)
				continue
			}
		}
		if c.Type == html.CommentNode {
			n.RemoveChild(c)
			continue
		}
		removeElements(c)
	}
}

// cleanAttributes removes data-* attributes, inline styles, class, and other noise
func cleanAttributes(n *html.Node) {
	if n.Type == html.ElementNode {
		var cleaned []html.Attribute
		for _, attr := range n.Attr {
			switch {
			case attr.Key == "href":
				cleaned = append(cleaned, attr)
			case attr.Key == "src":
				cleaned = append(cleaned, attr)
			case attr.Key == "alt":
				cleaned = append(cleaned, attr)
			case attr.Key == "title":
				cleaned = append(cleaned, attr)
			case attr.Key == "id":
				cleaned = append(cleaned, attr)
			case attr.Key == "lang":
				cleaned = append(cleaned, attr)
			case attr.Key == "charset":
				cleaned = append(cleaned, attr)
			case attr.Key == "name" && n.Data == "meta":
				cleaned = append(cleaned, attr)
			case attr.Key == "content" && n.Data == "meta":
				cleaned = append(cleaned, attr)
			case attr.Key == "rel":
				cleaned = append(cleaned, attr)
			case attr.Key == "type":
				cleaned = append(cleaned, attr)
			case attr.Key == "target":
				cleaned = append(cleaned, attr)
			}
		}
		n.Attr = cleaned
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		cleanAttributes(c)
	}
}

// normalizeURL strips fragment, trailing slash, and normalizes scheme for dedup
func normalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.Fragment = ""
	// Normalize http to https for dedup
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
	}
	result := parsed.String()
	result = strings.TrimRight(result, "/")
	return result
}

// urlToFilename converts a URL to a safe filename
func urlToFilename(rawURL string, base *url.URL) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "page.html"
	}

	path := parsed.Path
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		return "index.html"
	}

	// Replace slashes with underscores
	safe := strings.ReplaceAll(path, "/", "_")
	if !strings.HasSuffix(safe, ".html") {
		safe += ".html"
	}

	return safe
}

// truncateURL shortens a URL for display
func truncateURL(u string, maxLen int) string {
	if len(u) <= maxLen {
		return u
	}
	return u[:maxLen-3] + "..."
}

// isSameDomain checks if two hostnames share the same root domain
// e.g. hub.example.com and www.example.com both share example.com
func isSameDomain(host1, host2 string) bool {
	if host1 == host2 {
		return true
	}
	return rootDomain(host1) == rootDomain(host2)
}

// rootDomain extracts the root domain (last two segments) from a hostname
// e.g. hub.responsiveworks.com -> responsiveworks.com
func rootDomain(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// isResourceURL returns true if the path looks like a static resource, not a page
func isResourceURL(path string) bool {
	lower := strings.ToLower(path)
	extensions := []string{
		".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp",
		".ico", ".woff", ".woff2", ".ttf", ".eot", ".pdf", ".zip",
		".mp4", ".mp3", ".avi", ".mov", ".xml", ".json", ".txt",
	}
	for _, ext := range extensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
