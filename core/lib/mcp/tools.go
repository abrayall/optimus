package mcp

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

// RegisterSEOTools registers all SEO analysis tools with the server
func RegisterSEOTools(s *Server) {
	s.RegisterTool("fetch_headers",
		"Fetch HTTP response headers for a URL. Returns status code, all response headers, and redirect chain.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"url": {Type: "string", Description: "The URL to fetch headers from"},
			},
			Required: []string{"url"},
		},
		handleFetchHeaders,
	)

	s.RegisterTool("fetch_robots_txt",
		"Fetch and return the robots.txt file for a domain.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"url": {Type: "string", Description: "The website URL (domain will be extracted)"},
			},
			Required: []string{"url"},
		},
		handleFetchRobotsTxt,
	)

	s.RegisterTool("fetch_sitemap",
		"Fetch and parse sitemap.xml for a domain. Returns URLs found in the sitemap.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"url": {Type: "string", Description: "The website URL (domain will be extracted)"},
			},
			Required: []string{"url"},
		},
		handleFetchSitemap,
	)

	s.RegisterTool("check_links",
		"Check a list of URLs for broken links by sending HEAD requests. Returns status code for each URL.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"urls": {Type: "array", Description: "List of URLs to check", Items: &items{Type: "string"}},
			},
			Required: []string{"urls"},
		},
		handleCheckLinks,
	)

	s.RegisterTool("check_ssl",
		"Check SSL/TLS certificate details for a domain. Returns issuer, expiry, validity, and protocol info.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"url": {Type: "string", Description: "The website URL (domain will be extracted)"},
			},
			Required: []string{"url"},
		},
		handleCheckSSL,
	)

	s.RegisterTool("dns_lookup",
		"Perform DNS lookup for a domain. Returns A, AAAA, CNAME, MX, and TXT records.",
		inputSchema{
			Type: "object",
			Properties: map[string]property{
				"url": {Type: "string", Description: "The website URL (domain will be extracted)"},
			},
			Required: []string{"url"},
		},
		handleDNSLookup,
	)
}

type urlArg struct {
	URL string `json:"url"`
}

type urlsArg struct {
	URLs []string `json:"urls"`
}

func extractDomain(rawURL string) string {
	// Ensure scheme
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	// Simple domain extraction
	s := rawURL
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.Index(s, "/"); idx != -1 {
		s = s[:idx]
	}
	if idx := strings.Index(s, ":"); idx != -1 {
		s = s[:idx]
	}
	return s
}

func handleFetchHeaders(args json.RawMessage) (string, error) {
	var a urlArg
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Track redirects
	var redirects []string
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			redirects = append(redirects, req.URL.String())
			return nil
		},
	}

	resp, err := client.Get(a.URL)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", a.URL, err)
	}
	defer resp.Body.Close()

	var b strings.Builder
	fmt.Fprintf(&b, "Status: %s\n", resp.Status)
	if len(redirects) > 0 {
		fmt.Fprintf(&b, "\nRedirect chain:\n")
		for i, r := range redirects {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, r)
		}
	}
	fmt.Fprintf(&b, "\nHeaders:\n")
	for key, values := range resp.Header {
		for _, v := range values {
			fmt.Fprintf(&b, "  %s: %s\n", key, v)
		}
	}

	return b.String(), nil
}

func handleFetchRobotsTxt(args json.RawMessage) (string, error) {
	var a urlArg
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	domain := extractDomain(a.URL)
	robotsURL := fmt.Sprintf("https://%s/robots.txt", domain)

	resp, err := httpClient.Get(robotsURL)
	if err != nil {
		return "", fmt.Errorf("fetching robots.txt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "No robots.txt found (404)", nil
	}
	if resp.StatusCode != 200 {
		return fmt.Sprintf("robots.txt returned status %d", resp.StatusCode), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024))
	if err != nil {
		return "", fmt.Errorf("reading robots.txt: %w", err)
	}

	return string(body), nil
}

func handleFetchSitemap(args json.RawMessage) (string, error) {
	var a urlArg
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	domain := extractDomain(a.URL)
	sitemapURL := fmt.Sprintf("https://%s/sitemap.xml", domain)

	resp, err := httpClient.Get(sitemapURL)
	if err != nil {
		return "", fmt.Errorf("fetching sitemap.xml: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "No sitemap.xml found (404)", nil
	}
	if resp.StatusCode != 200 {
		return fmt.Sprintf("sitemap.xml returned status %d", resp.StatusCode), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500*1024))
	if err != nil {
		return "", fmt.Errorf("reading sitemap.xml: %w", err)
	}

	return string(body), nil
}

func handleCheckLinks(args json.RawMessage) (string, error) {
	var a urlsArg
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if len(a.URLs) > 50 {
		a.URLs = a.URLs[:50]
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	var b strings.Builder
	for _, u := range a.URLs {
		req, err := http.NewRequest("HEAD", u, nil)
		if err != nil {
			fmt.Fprintf(&b, "%s: error creating request: %s\n", u, err)
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Optimus SEO Checker)")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(&b, "%s: error: %s\n", u, err)
			continue
		}
		resp.Body.Close()
		fmt.Fprintf(&b, "%s: %d %s\n", u, resp.StatusCode, resp.Status[4:])
	}

	return b.String(), nil
}

func handleCheckSSL(args json.RawMessage) (string, error) {
	var a urlArg
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	domain := extractDomain(a.URL)

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		domain+":443",
		&tls.Config{},
	)
	if err != nil {
		return "", fmt.Errorf("TLS connection to %s failed: %w", domain, err)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	var b strings.Builder

	fmt.Fprintf(&b, "TLS Version: %s\n", tlsVersionName(state.Version))
	fmt.Fprintf(&b, "Cipher Suite: %s\n", tls.CipherSuiteName(state.CipherSuite))

	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		fmt.Fprintf(&b, "\nCertificate:\n")
		fmt.Fprintf(&b, "  Subject: %s\n", cert.Subject.CommonName)
		fmt.Fprintf(&b, "  Issuer: %s\n", cert.Issuer.CommonName)
		fmt.Fprintf(&b, "  Not Before: %s\n", cert.NotBefore.Format(time.RFC3339))
		fmt.Fprintf(&b, "  Not After: %s\n", cert.NotAfter.Format(time.RFC3339))

		daysLeft := time.Until(cert.NotAfter).Hours() / 24
		if daysLeft < 0 {
			fmt.Fprintf(&b, "  Status: EXPIRED (%.0f days ago)\n", -daysLeft)
		} else if daysLeft < 30 {
			fmt.Fprintf(&b, "  Status: EXPIRING SOON (%.0f days left)\n", daysLeft)
		} else {
			fmt.Fprintf(&b, "  Status: Valid (%.0f days remaining)\n", daysLeft)
		}

		if len(cert.DNSNames) > 0 {
			fmt.Fprintf(&b, "  DNS Names: %s\n", strings.Join(cert.DNSNames, ", "))
		}
	}

	return b.String(), nil
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", v)
	}
}

func handleDNSLookup(args json.RawMessage) (string, error) {
	var a urlArg
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	domain := extractDomain(a.URL)
	var b strings.Builder
	fmt.Fprintf(&b, "DNS records for %s:\n\n", domain)

	// A records
	ips, err := net.LookupHost(domain)
	if err == nil && len(ips) > 0 {
		fmt.Fprintf(&b, "A/AAAA Records:\n")
		for _, ip := range ips {
			fmt.Fprintf(&b, "  %s\n", ip)
		}
	}

	// CNAME
	cname, err := net.LookupCNAME(domain)
	if err == nil && cname != "" && cname != domain+"." {
		fmt.Fprintf(&b, "\nCNAME:\n  %s\n", cname)
	}

	// MX records
	mxs, err := net.LookupMX(domain)
	if err == nil && len(mxs) > 0 {
		fmt.Fprintf(&b, "\nMX Records:\n")
		for _, mx := range mxs {
			fmt.Fprintf(&b, "  %s (priority %d)\n", mx.Host, mx.Pref)
		}
	}

	// TXT records
	txts, err := net.LookupTXT(domain)
	if err == nil && len(txts) > 0 {
		fmt.Fprintf(&b, "\nTXT Records:\n")
		for _, txt := range txts {
			fmt.Fprintf(&b, "  %s\n", txt)
		}
	}

	// NS records
	nss, err := net.LookupNS(domain)
	if err == nil && len(nss) > 0 {
		fmt.Fprintf(&b, "\nNS Records:\n")
		for _, ns := range nss {
			fmt.Fprintf(&b, "  %s\n", ns.Host)
		}
	}

	return b.String(), nil
}
