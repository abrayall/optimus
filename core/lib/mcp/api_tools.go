package mcp

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// apiConfig holds API keys for external ranking services
var apiConfig struct {
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

// slowHTTPClient has a longer timeout for slow APIs like PageSpeed Insights
var slowHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

// SetAPIKeys configures the API keys for external ranking tools
func SetAPIKeys(serpAPIKey, googleAPIKey, googleCSEID, gscCredentials, perplexityKey, mozAPIKey, ahrefsAPIKey, bingAPIKey, redditClientID, redditClientSecret, twitterBearerToken string) {
	apiConfig.SerpAPIKey = serpAPIKey
	apiConfig.GoogleAPIKey = googleAPIKey
	apiConfig.GoogleCSEID = googleCSEID
	apiConfig.GSCCredentials = gscCredentials
	apiConfig.PerplexityKey = perplexityKey
	apiConfig.MozAPIKey = mozAPIKey
	apiConfig.AhrefsAPIKey = ahrefsAPIKey
	apiConfig.BingAPIKey = bingAPIKey
	apiConfig.RedditClientID = redditClientID
	apiConfig.RedditClientSecret = redditClientSecret
	apiConfig.TwitterBearerToken = twitterBearerToken
}

// RegisterAPITools registers external ranking API tools with the server
func RegisterAPITools(s *Server) {
	s.RegisterTool("serp_lookup",
		"Look up actual SERP (Search Engine Results Page) positions for keywords using SerpAPI. Returns ranking positions, URLs, and snippets from Google/Bing results. Requires --serpapi-key flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"query":  {Type: "string", Description: "Search query / keyword to look up"},
				"domain": {Type: "string", Description: "Domain to find in results (e.g. example.com)"},
				"engine": {Type: "string", Description: "Search engine: google (default) or bing"},
				"num":    {Type: "string", Description: "Number of results to return (default 10, max 100)"},
			},
			Required: []string{"query"},
		},
		handleSerpLookup,
	)

	s.RegisterTool("google_search",
		"Search Google using the Custom Search JSON API. Check if a site appears in results for specific queries. Requires --google-api-key and --google-cse-id flags.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"query": {Type: "string", Description: "Search query to execute"},
				"num":   {Type: "string", Description: "Number of results (1-10, default 10)"},
			},
			Required: []string{"query"},
		},
		handleGoogleSearch,
	)

	s.RegisterTool("search_console_query",
		"Query Google Search Console for real performance data: clicks, impressions, CTR, and average position. Requires --gsc-credentials flag pointing to a service account JSON file.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"site_url":   {Type: "string", Description: "Site URL as registered in Search Console (e.g. https://example.com/)"},
				"query":      {Type: "string", Description: "Optional: filter by search query (substring match)"},
				"start_date": {Type: "string", Description: "Start date in YYYY-MM-DD format (default: 30 days ago)"},
				"end_date":   {Type: "string", Description: "End date in YYYY-MM-DD format (default: today)"},
				"dimensions": {Type: "string", Description: "Comma-separated dimensions: query,page,country,device (default: query,page)"},
			},
			Required: []string{"site_url"},
		},
		handleSearchConsoleQuery,
	)

	s.RegisterTool("perplexity_ask",
		"Ask Perplexity AI a question and check if a specific site/URL gets cited in the answer. Useful for checking AI answer engine visibility. Requires --perplexity-key flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"question": {Type: "string", Description: "Question to ask Perplexity"},
				"domain":   {Type: "string", Description: "Domain to look for in citations (e.g. example.com)"},
			},
			Required: []string{"question"},
		},
		handlePerplexityAsk,
	)

	s.RegisterTool("moz_url_metrics",
		"Get Moz domain authority, page authority, spam score, and linking domains count for a URL or domain. Requires --moz-api-key flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"target": {Type: "string", Description: "URL or domain to analyze (e.g. example.com)"},
			},
			Required: []string{"target"},
		},
		handleMozURLMetrics,
	)

	s.RegisterTool("ahrefs_domain_rating",
		"Get Ahrefs Domain Rating (DR) and Ahrefs Rank for a domain. Requires --ahrefs-api-key flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"target": {Type: "string", Description: "Domain to analyze (e.g. example.com)"},
			},
			Required: []string{"target"},
		},
		handleAhrefsDomainRating,
	)

	s.RegisterTool("ahrefs_backlinks_stats",
		"Get Ahrefs backlink statistics: live backlinks count, referring domains, and referring pages. Requires --ahrefs-api-key flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"target": {Type: "string", Description: "Domain to analyze (e.g. example.com)"},
			},
			Required: []string{"target"},
		},
		handleAhrefsBacklinksStats,
	)

	s.RegisterTool("ahrefs_organic_keywords",
		"Get top organic keywords from Ahrefs with position, search volume, traffic estimate, and ranking URL. Requires --ahrefs-api-key flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"target":  {Type: "string", Description: "Domain to analyze (e.g. example.com)"},
				"country": {Type: "string", Description: "Country code for keyword data (default: us)"},
				"limit":   {Type: "string", Description: "Number of keywords to return (default: 10)"},
			},
			Required: []string{"target"},
		},
		handleAhrefsOrganicKeywords,
	)

	s.RegisterTool("pagespeed_insights",
		"Get Google PageSpeed Insights scores (performance, accessibility, best-practices, SEO) and Core Web Vitals (LCP, CLS, INP) for a URL. Requires --google-api-key flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"url":      {Type: "string", Description: "Page URL to analyze"},
				"strategy": {Type: "string", Description: "Analysis strategy: mobile (default) or desktop"},
			},
			Required: []string{"url"},
		},
		handlePageSpeedInsights,
	)

	s.RegisterTool("url_inspection",
		"Inspect a URL's indexing status in Google Search using the URL Inspection API. Returns indexing state, crawl info, and rich results status. Requires --gsc-credentials flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"inspection_url": {Type: "string", Description: "URL to inspect"},
				"site_url":       {Type: "string", Description: "Site URL as registered in Search Console (e.g. https://example.com/)"},
			},
			Required: []string{"inspection_url", "site_url"},
		},
		handleURLInspection,
	)

	s.RegisterTool("bing_webmaster_stats",
		"Get Bing Webmaster Tools query stats — impressions, clicks, and average position for top queries. Requires --bing-api-key flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"site_url": {Type: "string", Description: "Site URL as registered in Bing Webmaster Tools"},
			},
			Required: []string{"site_url"},
		},
		handleBingWebmasterStats,
	)

	s.RegisterTool("reddit_search",
		"Search Reddit for brand or domain mentions. Returns recent posts with title, subreddit, score, and comments. Requires --reddit-client-id and --reddit-client-secret flags.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"query": {Type: "string", Description: "Search query (domain or brand name)"},
				"limit": {Type: "string", Description: "Number of results to return (default: 10)"},
			},
			Required: []string{"query"},
		},
		handleRedditSearch,
	)

	s.RegisterTool("twitter_search",
		"Search recent tweets for brand or domain mentions. Returns tweets with text, author, likes, retweets. Requires --twitter-bearer-token flag.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"query":       {Type: "string", Description: "Search query (domain or brand name)"},
				"max_results": {Type: "string", Description: "Number of results (10-100, default: 10)"},
			},
			Required: []string{"query"},
		},
		handleTwitterSearch,
	)
}

// --- serp_lookup handler ---

type serpLookupArgs struct {
	Query  string `json:"query"`
	Domain string `json:"domain"`
	Engine string `json:"engine"`
	Num    string `json:"num"`
}

func handleSerpLookup(args json.RawMessage) (string, error) {
	if apiConfig.SerpAPIKey == "" {
		return "serp_lookup is not configured. Pass --serpapi-key to enable SERP position lookups. Skipping this check.", nil
	}

	var a serpLookupArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	engine := a.Engine
	if engine == "" {
		engine = "google"
	}
	num := a.Num
	if num == "" {
		num = "10"
	}

	params := url.Values{}
	params.Set("q", a.Query)
	params.Set("engine", engine)
	params.Set("num", num)
	params.Set("api_key", apiConfig.SerpAPIKey)

	resp, err := httpClient.Get("https://serpapi.com/search?" + params.Encode())
	if err != nil {
		return "", fmt.Errorf("SerpAPI request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading SerpAPI response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("SerpAPI error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	// Parse response to extract organic results
	var serpResp struct {
		OrganicResults []struct {
			Position int    `json:"position"`
			Title    string `json:"title"`
			Link     string `json:"link"`
			Snippet  string `json:"snippet"`
		} `json:"organic_results"`
		SearchMetadata struct {
			TotalResults string `json:"total_results"`
		} `json:"search_metadata"`
	}

	if err := json.Unmarshal(body, &serpResp); err != nil {
		return "", fmt.Errorf("parsing SerpAPI response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "SERP Results for %q (engine: %s):\n\n", a.Query, engine)

	domainFound := false
	for _, r := range serpResp.OrganicResults {
		marker := ""
		if a.Domain != "" && strings.Contains(r.Link, a.Domain) {
			marker = " <<<< FOUND"
			domainFound = true
		}
		fmt.Fprintf(&b, "#%d: %s%s\n    %s\n    %s\n\n", r.Position, r.Title, marker, r.Link, r.Snippet)
	}

	if a.Domain != "" {
		fmt.Fprintf(&b, "---\n")
		if domainFound {
			fmt.Fprintf(&b, "Domain %q WAS found in the results.\n", a.Domain)
		} else {
			fmt.Fprintf(&b, "Domain %q was NOT found in the top %s results.\n", a.Domain, num)
		}
	}

	return b.String(), nil
}

// --- google_search handler ---

type googleSearchArgs struct {
	Query string `json:"query"`
	Num   string `json:"num"`
}

func handleGoogleSearch(args json.RawMessage) (string, error) {
	if apiConfig.GoogleAPIKey == "" || apiConfig.GoogleCSEID == "" {
		return "google_search is not configured. Pass --google-api-key and --google-cse-id to enable Google Custom Search. Skipping this check.", nil
	}

	var a googleSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	num := a.Num
	if num == "" {
		num = "10"
	}

	params := url.Values{}
	params.Set("q", a.Query)
	params.Set("key", apiConfig.GoogleAPIKey)
	params.Set("cx", apiConfig.GoogleCSEID)
	params.Set("num", num)

	resp, err := httpClient.Get("https://www.googleapis.com/customsearch/v1?" + params.Encode())
	if err != nil {
		return "", fmt.Errorf("Google Custom Search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Google search response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Google Custom Search error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var searchResp struct {
		SearchInformation struct {
			TotalResults     string  `json:"totalResults"`
			SearchTime       float64 `json:"searchTime"`
			FormattedResults string  `json:"formattedTotalResults"`
		} `json:"searchInformation"`
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("parsing Google search response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Google Search Results for %q:\n", a.Query)
	fmt.Fprintf(&b, "Total results: %s (%.2fs)\n\n", searchResp.SearchInformation.FormattedResults, searchResp.SearchInformation.SearchTime)

	for i, item := range searchResp.Items {
		fmt.Fprintf(&b, "#%d: %s\n    %s\n    %s\n\n", i+1, item.Title, item.Link, item.Snippet)
	}

	return b.String(), nil
}

// --- search_console_query handler ---

type gscArgs struct {
	SiteURL    string `json:"site_url"`
	Query      string `json:"query"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	Dimensions string `json:"dimensions"`
}

// serviceAccountKey represents the JSON key file for a Google service account
type serviceAccountKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

func handleSearchConsoleQuery(args json.RawMessage) (string, error) {
	if apiConfig.GSCCredentials == "" {
		return "search_console_query is not configured. Pass --gsc-credentials with a path to a Google service account JSON file to enable Search Console data. Skipping this check.", nil
	}

	var a gscArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Default dates: last 30 days
	endDate := a.EndDate
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}
	startDate := a.StartDate
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}

	dimensions := a.Dimensions
	if dimensions == "" {
		dimensions = "query,page"
	}

	// Get access token via service account JWT
	accessToken, err := getGSCAccessToken(apiConfig.GSCCredentials)
	if err != nil {
		return "", fmt.Errorf("authenticating with Google: %w", err)
	}

	// Build Search Console API request
	dimList := strings.Split(dimensions, ",")
	for i := range dimList {
		dimList[i] = strings.TrimSpace(dimList[i])
	}

	reqBody := map[string]interface{}{
		"startDate":  startDate,
		"endDate":    endDate,
		"dimensions": dimList,
		"rowLimit":   100,
	}

	if a.Query != "" {
		reqBody["dimensionFilterGroups"] = []map[string]interface{}{
			{
				"filters": []map[string]interface{}{
					{
						"dimension":  "query",
						"operator":   "contains",
						"expression": a.Query,
					},
				},
			},
		}
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	encodedSiteURL := url.QueryEscape(a.SiteURL)
	apiURL := fmt.Sprintf("https://www.googleapis.com/webmasters/v3/sites/%s/searchAnalytics/query", encodedSiteURL)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(reqJSON)))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Search Console API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Search Console response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Search Console API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var gscResp struct {
		Rows []struct {
			Keys        []string `json:"keys"`
			Clicks      float64  `json:"clicks"`
			Impressions float64  `json:"impressions"`
			CTR         float64  `json:"ctr"`
			Position    float64  `json:"position"`
		} `json:"rows"`
	}

	if err := json.Unmarshal(body, &gscResp); err != nil {
		return "", fmt.Errorf("parsing Search Console response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Google Search Console Data for %s\n", a.SiteURL)
	fmt.Fprintf(&b, "Period: %s to %s\n", startDate, endDate)
	fmt.Fprintf(&b, "Dimensions: %s\n\n", dimensions)

	if len(gscResp.Rows) == 0 {
		fmt.Fprintf(&b, "No data found for the specified parameters.\n")
		return b.String(), nil
	}

	// Header
	for _, dim := range dimList {
		fmt.Fprintf(&b, "%-30s", strings.Title(dim))
	}
	fmt.Fprintf(&b, "%10s %12s %8s %10s\n", "Clicks", "Impressions", "CTR", "Position")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 30*len(dimList)+42))

	for _, row := range gscResp.Rows {
		for _, key := range row.Keys {
			fmt.Fprintf(&b, "%-30s", truncate(key, 29))
		}
		fmt.Fprintf(&b, "%10.0f %12.0f %7.1f%% %10.1f\n", row.Clicks, row.Impressions, row.CTR*100, row.Position)
	}

	fmt.Fprintf(&b, "\nTotal rows: %d\n", len(gscResp.Rows))

	return b.String(), nil
}

// getGSCAccessToken performs manual JWT authentication for Google APIs
func getGSCAccessToken(credentialsPath string) (string, error) {
	// Read service account key file
	keyData, err := os.ReadFile(credentialsPath)
	if err != nil {
		return "", fmt.Errorf("reading credentials file: %w", err)
	}

	var key serviceAccountKey
	if err := json.Unmarshal(keyData, &key); err != nil {
		return "", fmt.Errorf("parsing credentials file: %w", err)
	}

	if key.TokenURI == "" {
		key.TokenURI = "https://oauth2.googleapis.com/token"
	}

	// Build JWT
	now := time.Now()
	header := base64URLEncode(mustJSON(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}))

	claims := base64URLEncode(mustJSON(map[string]interface{}{
		"iss":   key.ClientEmail,
		"scope": "https://www.googleapis.com/auth/webmasters.readonly",
		"aud":   key.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}))

	signingInput := header + "." + claims

	// Parse RSA private key
	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block from private key")
	}

	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing private key: %w", err)
	}

	rsaKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not RSA")
	}

	// Sign
	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(nil, rsaKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	jwt := signingInput + "." + base64URLEncode(sig)

	// Exchange JWT for access token
	resp, err := http.PostForm(key.TokenURI, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwt},
	})
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func mustJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// --- perplexity_ask handler ---

type perplexityArgs struct {
	Question string `json:"question"`
	Domain   string `json:"domain"`
}

func handlePerplexityAsk(args json.RawMessage) (string, error) {
	if apiConfig.PerplexityKey == "" {
		return "perplexity_ask is not configured. Pass --perplexity-key to enable Perplexity AI citation checks. Skipping this check.", nil
	}

	var a perplexityArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	reqBody := map[string]interface{}{
		"model": "sonar",
		"messages": []map[string]string{
			{"role": "user", "content": a.Question},
		},
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.perplexity.ai/chat/completions", strings.NewReader(string(reqJSON)))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiConfig.PerplexityKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Perplexity API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Perplexity response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Perplexity API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var pResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Citations []string `json:"citations"`
	}

	if err := json.Unmarshal(body, &pResp); err != nil {
		return "", fmt.Errorf("parsing Perplexity response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Perplexity AI Answer for: %q\n\n", a.Question)

	if len(pResp.Choices) > 0 {
		fmt.Fprintf(&b, "Answer:\n%s\n\n", pResp.Choices[0].Message.Content)
	}

	if len(pResp.Citations) > 0 {
		fmt.Fprintf(&b, "Citations:\n")
		domainCited := false
		for i, citation := range pResp.Citations {
			marker := ""
			if a.Domain != "" && strings.Contains(citation, a.Domain) {
				marker = " <<<< CITED"
				domainCited = true
			}
			fmt.Fprintf(&b, "  [%d] %s%s\n", i+1, citation, marker)
		}

		if a.Domain != "" {
			fmt.Fprintf(&b, "\n---\n")
			if domainCited {
				fmt.Fprintf(&b, "Domain %q WAS cited by Perplexity.\n", a.Domain)
			} else {
				fmt.Fprintf(&b, "Domain %q was NOT cited by Perplexity.\n", a.Domain)
			}
		}
	} else {
		fmt.Fprintf(&b, "No citations returned.\n")
	}

	return b.String(), nil
}

// --- moz_url_metrics handler ---

type mozURLMetricsArgs struct {
	Target string `json:"target"`
}

func handleMozURLMetrics(args json.RawMessage) (string, error) {
	if apiConfig.MozAPIKey == "" {
		return "moz_url_metrics is not configured. Pass --moz-api-key to enable Moz domain authority lookups. Skipping this check.", nil
	}

	var a mozURLMetricsArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	reqBody, err := json.Marshal(map[string][]string{
		"targets": {a.Target},
	})
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://lsapi.seomoz.com/v2/url_metrics", strings.NewReader(string(reqBody)))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("x-moz-token", apiConfig.MozAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Moz API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Moz response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Moz API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var mozResp struct {
		Results []struct {
			DomainAuthority         float64 `json:"domain_authority"`
			PageAuthority           float64 `json:"page_authority"`
			SpamScore               float64 `json:"spam_score"`
			RootDomainsToRootDomain int     `json:"root_domains_to_root_domain"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &mozResp); err != nil {
		return "", fmt.Errorf("parsing Moz response: %w", err)
	}

	if len(mozResp.Results) == 0 {
		return "Moz returned no results for this target.", nil
	}

	r := mozResp.Results[0]
	var b strings.Builder
	fmt.Fprintf(&b, "Moz URL Metrics for %q:\n\n", a.Target)
	fmt.Fprintf(&b, "Domain Authority (DA): %.0f\n", r.DomainAuthority)
	fmt.Fprintf(&b, "Page Authority (PA):   %.0f\n", r.PageAuthority)
	fmt.Fprintf(&b, "Spam Score:            %.0f%%\n", r.SpamScore)
	fmt.Fprintf(&b, "Linking Root Domains:  %d\n", r.RootDomainsToRootDomain)

	return b.String(), nil
}

// --- ahrefs_domain_rating handler ---

type ahrefsDomainRatingArgs struct {
	Target string `json:"target"`
}

func handleAhrefsDomainRating(args json.RawMessage) (string, error) {
	if apiConfig.AhrefsAPIKey == "" {
		return "ahrefs_domain_rating is not configured. Pass --ahrefs-api-key to enable Ahrefs domain rating lookups. Skipping this check.", nil
	}

	var a ahrefsDomainRatingArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	params := url.Values{}
	params.Set("target", a.Target)

	req, err := http.NewRequest("GET", "https://api.ahrefs.com/v3/site-explorer/domain-rating?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiConfig.AhrefsAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ahrefs API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Ahrefs response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Ahrefs API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var ahrefsResp struct {
		DomainRating float64 `json:"domain_rating"`
		AhrefsRank   int     `json:"ahrefs_rank"`
	}

	if err := json.Unmarshal(body, &ahrefsResp); err != nil {
		return "", fmt.Errorf("parsing Ahrefs response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Ahrefs Domain Rating for %q:\n\n", a.Target)
	fmt.Fprintf(&b, "Domain Rating (DR): %.0f\n", ahrefsResp.DomainRating)
	fmt.Fprintf(&b, "Ahrefs Rank:        %d\n", ahrefsResp.AhrefsRank)

	return b.String(), nil
}

// --- ahrefs_backlinks_stats handler ---

type ahrefsBacklinksStatsArgs struct {
	Target string `json:"target"`
}

func handleAhrefsBacklinksStats(args json.RawMessage) (string, error) {
	if apiConfig.AhrefsAPIKey == "" {
		return "ahrefs_backlinks_stats is not configured. Pass --ahrefs-api-key to enable Ahrefs backlink statistics. Skipping this check.", nil
	}

	var a ahrefsBacklinksStatsArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	params := url.Values{}
	params.Set("target", a.Target)

	req, err := http.NewRequest("GET", "https://api.ahrefs.com/v3/site-explorer/backlinks-stats?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiConfig.AhrefsAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ahrefs API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Ahrefs response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Ahrefs API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var ahrefsResp struct {
		LiveBacklinks    int `json:"live"`
		ReferringDomains int `json:"ref_domains"`
		ReferringPages   int `json:"ref_pages"`
	}

	if err := json.Unmarshal(body, &ahrefsResp); err != nil {
		return "", fmt.Errorf("parsing Ahrefs response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Ahrefs Backlink Stats for %q:\n\n", a.Target)
	fmt.Fprintf(&b, "Live Backlinks:    %d\n", ahrefsResp.LiveBacklinks)
	fmt.Fprintf(&b, "Referring Domains: %d\n", ahrefsResp.ReferringDomains)
	fmt.Fprintf(&b, "Referring Pages:   %d\n", ahrefsResp.ReferringPages)

	return b.String(), nil
}

// --- ahrefs_organic_keywords handler ---

type ahrefsOrganicKeywordsArgs struct {
	Target  string `json:"target"`
	Country string `json:"country"`
	Limit   string `json:"limit"`
}

func handleAhrefsOrganicKeywords(args json.RawMessage) (string, error) {
	if apiConfig.AhrefsAPIKey == "" {
		return "ahrefs_organic_keywords is not configured. Pass --ahrefs-api-key to enable Ahrefs organic keyword data. Skipping this check.", nil
	}

	var a ahrefsOrganicKeywordsArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	country := a.Country
	if country == "" {
		country = "us"
	}
	limit := a.Limit
	if limit == "" {
		limit = "10"
	}

	params := url.Values{}
	params.Set("target", a.Target)
	params.Set("country", country)
	params.Set("limit", limit)

	req, err := http.NewRequest("GET", "https://api.ahrefs.com/v3/site-explorer/organic-keywords?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiConfig.AhrefsAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ahrefs API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Ahrefs response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Ahrefs API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var ahrefsResp struct {
		Keywords []struct {
			Keyword  string `json:"keyword"`
			Position int    `json:"position"`
			Volume   int    `json:"volume"`
			Traffic  int    `json:"traffic"`
			URL      string `json:"url"`
		} `json:"keywords"`
	}

	if err := json.Unmarshal(body, &ahrefsResp); err != nil {
		return "", fmt.Errorf("parsing Ahrefs response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Ahrefs Organic Keywords for %q (country: %s):\n\n", a.Target, country)

	if len(ahrefsResp.Keywords) == 0 {
		fmt.Fprintf(&b, "No organic keywords found.\n")
		return b.String(), nil
	}

	fmt.Fprintf(&b, "%-40s %8s %8s %8s  %s\n", "Keyword", "Position", "Volume", "Traffic", "URL")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 100))

	for _, kw := range ahrefsResp.Keywords {
		fmt.Fprintf(&b, "%-40s %8d %8d %8d  %s\n", truncate(kw.Keyword, 39), kw.Position, kw.Volume, kw.Traffic, kw.URL)
	}

	fmt.Fprintf(&b, "\nTotal keywords returned: %d\n", len(ahrefsResp.Keywords))

	return b.String(), nil
}

// --- pagespeed_insights handler ---

type pageSpeedArgs struct {
	URL      string `json:"url"`
	Strategy string `json:"strategy"`
}

func handlePageSpeedInsights(args json.RawMessage) (string, error) {
	if apiConfig.GoogleAPIKey == "" {
		return "pagespeed_insights is not configured. Pass --google-api-key to enable PageSpeed Insights. Skipping this check.", nil
	}

	var a pageSpeedArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	strategy := a.Strategy
	if strategy == "" {
		strategy = "mobile"
	}

	params := url.Values{}
	params.Set("url", a.URL)
	params.Set("strategy", strategy)
	params.Set("key", apiConfig.GoogleAPIKey)
	params.Add("category", "PERFORMANCE")
	params.Add("category", "ACCESSIBILITY")
	params.Add("category", "BEST_PRACTICES")
	params.Add("category", "SEO")

	resp, err := slowHTTPClient.Get("https://pagespeedonline.googleapis.com/pagespeedonline/v5/runPagespeed?" + params.Encode())
	if err != nil {
		return "", fmt.Errorf("PageSpeed API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", fmt.Errorf("reading PageSpeed response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("PageSpeed API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var psResp struct {
		LighthouseResult struct {
			Categories map[string]struct {
				Score float64 `json:"score"`
				Title string  `json:"title"`
			} `json:"categories"`
			Audits map[string]struct {
				NumericValue float64 `json:"numericValue"`
				DisplayValue string  `json:"displayValue"`
			} `json:"audits"`
		} `json:"lighthouseResult"`
	}

	if err := json.Unmarshal(body, &psResp); err != nil {
		return "", fmt.Errorf("parsing PageSpeed response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "PageSpeed Insights for %q (strategy: %s):\n\n", a.URL, strategy)

	fmt.Fprintf(&b, "Lighthouse Scores:\n")
	for _, key := range []string{"performance", "accessibility", "best-practices", "seo"} {
		if cat, ok := psResp.LighthouseResult.Categories[key]; ok {
			fmt.Fprintf(&b, "  %-20s %3.0f/100\n", cat.Title+":", cat.Score*100)
		}
	}

	fmt.Fprintf(&b, "\nCore Web Vitals:\n")
	vitals := []struct {
		key   string
		label string
	}{
		{"largest-contentful-paint", "LCP (Largest Contentful Paint)"},
		{"cumulative-layout-shift", "CLS (Cumulative Layout Shift)"},
		{"interaction-to-next-paint", "INP (Interaction to Next Paint)"},
		{"first-contentful-paint", "FCP (First Contentful Paint)"},
		{"total-blocking-time", "TBT (Total Blocking Time)"},
		{"speed-index", "Speed Index"},
	}
	for _, v := range vitals {
		if audit, ok := psResp.LighthouseResult.Audits[v.key]; ok && audit.DisplayValue != "" {
			fmt.Fprintf(&b, "  %-35s %s\n", v.label+":", audit.DisplayValue)
		}
	}

	return b.String(), nil
}

// --- url_inspection handler ---

type urlInspectionArgs struct {
	InspectionURL string `json:"inspection_url"`
	SiteURL       string `json:"site_url"`
}

func handleURLInspection(args json.RawMessage) (string, error) {
	if apiConfig.GSCCredentials == "" {
		return "url_inspection is not configured. Pass --gsc-credentials to enable URL Inspection. Skipping this check.", nil
	}

	var a urlInspectionArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	accessToken, err := getGSCAccessToken(apiConfig.GSCCredentials)
	if err != nil {
		return "", fmt.Errorf("authenticating with Google: %w", err)
	}

	reqBody, err := json.Marshal(map[string]string{
		"inspectionUrl": a.InspectionURL,
		"siteUrl":       a.SiteURL,
	})
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://searchconsole.googleapis.com/v1/urlInspection/index:inspect", strings.NewReader(string(reqBody)))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("URL Inspection API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading URL Inspection response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("URL Inspection API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var inspResp struct {
		InspectionResult struct {
			IndexStatusResult struct {
				Verdict           string `json:"verdict"`
				CoverageState     string `json:"coverageState"`
				LastCrawlTime     string `json:"lastCrawlTime"`
				PageFetchState    string `json:"pageFetchState"`
				CrawledAs         string `json:"crawledAs"`
				RobotsTxtState    string `json:"robotsTxtState"`
				IndexingState     string `json:"indexingState"`
			} `json:"indexStatusResult"`
			RichResultsResult struct {
				Verdict          string `json:"verdict"`
				DetectedItems    []struct {
					RichResultType string `json:"richResultType"`
					Items          []struct {
						Name string `json:"name"`
					} `json:"items"`
				} `json:"detectedItems"`
			} `json:"richResultsResult"`
		} `json:"inspectionResult"`
	}

	if err := json.Unmarshal(body, &inspResp); err != nil {
		return "", fmt.Errorf("parsing URL Inspection response: %w", err)
	}

	idx := inspResp.InspectionResult.IndexStatusResult
	rich := inspResp.InspectionResult.RichResultsResult

	var b strings.Builder
	fmt.Fprintf(&b, "URL Inspection for %q:\n\n", a.InspectionURL)
	fmt.Fprintf(&b, "Index Status:\n")
	fmt.Fprintf(&b, "  Verdict:         %s\n", idx.Verdict)
	fmt.Fprintf(&b, "  Coverage State:  %s\n", idx.CoverageState)
	fmt.Fprintf(&b, "  Indexing State:  %s\n", idx.IndexingState)
	fmt.Fprintf(&b, "  Page Fetch:      %s\n", idx.PageFetchState)
	fmt.Fprintf(&b, "  Crawled As:      %s\n", idx.CrawledAs)
	fmt.Fprintf(&b, "  Robots.txt:      %s\n", idx.RobotsTxtState)
	if idx.LastCrawlTime != "" {
		fmt.Fprintf(&b, "  Last Crawl:      %s\n", idx.LastCrawlTime)
	}

	if rich.Verdict != "" {
		fmt.Fprintf(&b, "\nRich Results:\n")
		fmt.Fprintf(&b, "  Verdict: %s\n", rich.Verdict)
		for _, item := range rich.DetectedItems {
			fmt.Fprintf(&b, "  Type: %s\n", item.RichResultType)
			for _, i := range item.Items {
				fmt.Fprintf(&b, "    - %s\n", i.Name)
			}
		}
	}

	return b.String(), nil
}

// --- bing_webmaster_stats handler ---

type bingWebmasterArgs struct {
	SiteURL string `json:"site_url"`
}

func handleBingWebmasterStats(args json.RawMessage) (string, error) {
	if apiConfig.BingAPIKey == "" {
		return "bing_webmaster_stats is not configured. Pass --bing-api-key to enable Bing Webmaster Tools data. Skipping this check.", nil
	}

	var a bingWebmasterArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	params := url.Values{}
	params.Set("siteUrl", a.SiteURL)
	params.Set("apikey", apiConfig.BingAPIKey)

	resp, err := httpClient.Get("https://ssl.bing.com/webmaster/api.svc/json/GetQueryStats?" + params.Encode())
	if err != nil {
		return "", fmt.Errorf("Bing Webmaster API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Bing response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Bing Webmaster API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var bingResp struct {
		D []struct {
			Query       string  `json:"Query"`
			Impressions int     `json:"Impressions"`
			Clicks      int     `json:"Clicks"`
			AvgPosition float64 `json:"AvgClickPosition"`
		} `json:"d"`
	}

	if err := json.Unmarshal(body, &bingResp); err != nil {
		return "", fmt.Errorf("parsing Bing response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Bing Webmaster Query Stats for %q:\n\n", a.SiteURL)

	if len(bingResp.D) == 0 {
		fmt.Fprintf(&b, "No query stats available.\n")
		return b.String(), nil
	}

	fmt.Fprintf(&b, "%-40s %12s %8s %10s\n", "Query", "Impressions", "Clicks", "Avg Pos")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 72))

	for _, q := range bingResp.D {
		fmt.Fprintf(&b, "%-40s %12d %8d %10.1f\n", truncate(q.Query, 39), q.Impressions, q.Clicks, q.AvgPosition)
	}

	fmt.Fprintf(&b, "\nTotal queries: %d\n", len(bingResp.D))
	return b.String(), nil
}

// --- reddit_search handler ---

type redditSearchArgs struct {
	Query string `json:"query"`
	Limit string `json:"limit"`
}

func handleRedditSearch(args json.RawMessage) (string, error) {
	if apiConfig.RedditClientID == "" || apiConfig.RedditClientSecret == "" {
		return "reddit_search is not configured. Pass --reddit-client-id and --reddit-client-secret to enable Reddit search. Skipping this check.", nil
	}

	var a redditSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	limit := a.Limit
	if limit == "" {
		limit = "10"
	}

	accessToken, err := getRedditAccessToken()
	if err != nil {
		return "", fmt.Errorf("authenticating with Reddit: %w", err)
	}

	params := url.Values{}
	params.Set("q", a.Query)
	params.Set("limit", limit)
	params.Set("sort", "relevance")
	params.Set("t", "year")

	req, err := http.NewRequest("GET", "https://oauth.reddit.com/search?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "optimus:seo-tool:v1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Reddit API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Reddit response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Reddit API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var redditResp struct {
		Data struct {
			Children []struct {
				Data struct {
					Title       string  `json:"title"`
					Subreddit   string  `json:"subreddit"`
					Score       int     `json:"score"`
					NumComments int     `json:"num_comments"`
					Permalink   string  `json:"permalink"`
					CreatedUTC  float64 `json:"created_utc"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &redditResp); err != nil {
		return "", fmt.Errorf("parsing Reddit response: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Reddit Search Results for %q:\n\n", a.Query)

	if len(redditResp.Data.Children) == 0 {
		fmt.Fprintf(&b, "No posts found.\n")
		return b.String(), nil
	}

	for i, child := range redditResp.Data.Children {
		post := child.Data
		t := time.Unix(int64(post.CreatedUTC), 0)
		fmt.Fprintf(&b, "#%d: %s\n", i+1, post.Title)
		fmt.Fprintf(&b, "    r/%s | Score: %d | Comments: %d | %s\n", post.Subreddit, post.Score, post.NumComments, t.Format("2006-01-02"))
		fmt.Fprintf(&b, "    https://reddit.com%s\n\n", post.Permalink)
	}

	fmt.Fprintf(&b, "Total posts: %d\n", len(redditResp.Data.Children))
	return b.String(), nil
}

// getRedditAccessToken obtains an OAuth2 access token using client credentials
func getRedditAccessToken() (string, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.SetBasicAuth(apiConfig.RedditClientID, apiConfig.RedditClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "optimus:seo-tool:v1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Reddit token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Reddit token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// --- twitter_search handler ---

type twitterSearchArgs struct {
	Query      string `json:"query"`
	MaxResults string `json:"max_results"`
}

func handleTwitterSearch(args json.RawMessage) (string, error) {
	if apiConfig.TwitterBearerToken == "" {
		return "twitter_search is not configured. Pass --twitter-bearer-token to enable Twitter search. Skipping this check.", nil
	}

	var a twitterSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	maxResults := a.MaxResults
	if maxResults == "" {
		maxResults = "10"
	}

	params := url.Values{}
	params.Set("query", a.Query)
	params.Set("max_results", maxResults)
	params.Set("tweet.fields", "created_at,public_metrics,author_id")
	params.Set("expansions", "author_id")
	params.Set("user.fields", "username")

	req, err := http.NewRequest("GET", "https://api.twitter.com/2/tweets/search/recent?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiConfig.TwitterBearerToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Twitter API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("reading Twitter response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Twitter API error (HTTP %d): %s", resp.StatusCode, string(body)), nil
	}

	var twitterResp struct {
		Data []struct {
			Text          string `json:"text"`
			AuthorID      string `json:"author_id"`
			CreatedAt     string `json:"created_at"`
			PublicMetrics struct {
				LikeCount    int `json:"like_count"`
				RetweetCount int `json:"retweet_count"`
				ReplyCount   int `json:"reply_count"`
			} `json:"public_metrics"`
		} `json:"data"`
		Includes struct {
			Users []struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			} `json:"users"`
		} `json:"includes"`
	}

	if err := json.Unmarshal(body, &twitterResp); err != nil {
		return "", fmt.Errorf("parsing Twitter response: %w", err)
	}

	// Build author lookup map
	authorMap := make(map[string]string)
	for _, u := range twitterResp.Includes.Users {
		authorMap[u.ID] = u.Username
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Twitter Search Results for %q:\n\n", a.Query)

	if len(twitterResp.Data) == 0 {
		fmt.Fprintf(&b, "No tweets found.\n")
		return b.String(), nil
	}

	for i, tweet := range twitterResp.Data {
		username := authorMap[tweet.AuthorID]
		if username == "" {
			username = tweet.AuthorID
		}
		fmt.Fprintf(&b, "#%d @%s", i+1, username)
		if tweet.CreatedAt != "" {
			fmt.Fprintf(&b, " (%s)", tweet.CreatedAt)
		}
		fmt.Fprintf(&b, "\n")
		fmt.Fprintf(&b, "    %s\n", tweet.Text)
		fmt.Fprintf(&b, "    Likes: %d | Retweets: %d | Replies: %d\n\n",
			tweet.PublicMetrics.LikeCount, tweet.PublicMetrics.RetweetCount, tweet.PublicMetrics.ReplyCount)
	}

	fmt.Fprintf(&b, "Total tweets: %d\n", len(twitterResp.Data))
	return b.String(), nil
}
