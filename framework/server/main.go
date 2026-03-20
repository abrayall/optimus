package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"optimus/core/lib/engine"
	"optimus/core/lib/mcp"
	"optimus/core/lib/ui"
)

func main() {
	// Check for --mcp mode before parsing flags.
	// This allows engine.writeMCPConfig to spawn this binary with --mcp.
	for _, arg := range os.Args[1:] {
		if arg == "--mcp" {
			parseMCPFlags()
			return
		}
	}

	// Server flags
	port := flag.String("port", envOrDefault("PORT", "8080"), "Listen port")
	host := flag.String("host", envOrDefault("HOST", "0.0.0.0"), "Bind address")

	// Publishing
	publish := flag.String("publish", defaultPublish(), "Publish destination (local, s3)")
	s3Bucket := flag.String("s3-bucket", os.Getenv("S3_BUCKET"), "S3 bucket name")
	s3Region := flag.String("s3-region", envOrDefault("S3_REGION", "us-east-1"), "S3 region")
	s3Endpoint := flag.String("s3-endpoint", os.Getenv("S3_ENDPOINT"), "Custom S3 endpoint URL")

	// API keys
	serpAPIKey := flag.String("serp-api-key", os.Getenv("SERP_API_KEY"), "SerpAPI key")
	googleAPIKey := flag.String("google-api-key", os.Getenv("GOOGLE_API_KEY"), "Google API key")
	googleCSEID := flag.String("google-cse-id", os.Getenv("GOOGLE_CSE_ID"), "Google CSE ID")
	gscCredentials := flag.String("gsc-credentials", os.Getenv("GSC_CREDENTIALS"), "Google Search Console credentials path")
	perplexityKey := flag.String("perplexity-key", os.Getenv("PERPLEXITY_KEY"), "Perplexity API key")
	mozAPIKey := flag.String("moz-api-key", os.Getenv("MOZ_API_KEY"), "Moz API key")
	ahrefsAPIKey := flag.String("ahrefs-api-key", os.Getenv("AHREFS_API_KEY"), "Ahrefs API key")
	bingAPIKey := flag.String("bing-api-key", os.Getenv("BING_API_KEY"), "Bing API key")
	redditClientID := flag.String("reddit-client-id", os.Getenv("REDDIT_CLIENT_ID"), "Reddit client ID")
	redditClientSecret := flag.String("reddit-client-secret", os.Getenv("REDDIT_CLIENT_SECRET"), "Reddit client secret")
	twitterBearerToken := flag.String("twitter-bearer-token", os.Getenv("TWITTER_BEARER_TOKEN"), "Twitter bearer token")

	flag.Parse()

	cfg := ServerConfig{
		Host: *host,
		Port: *port,
		EngineKeys: engine.Config{
			SerpAPIKey:         *serpAPIKey,
			GoogleAPIKey:       *googleAPIKey,
			GoogleCSEID:        *googleCSEID,
			GSCCredentials:     *gscCredentials,
			PerplexityKey:      *perplexityKey,
			MozAPIKey:          *mozAPIKey,
			AhrefsAPIKey:       *ahrefsAPIKey,
			BingAPIKey:         *bingAPIKey,
			RedditClientID:     *redditClientID,
			RedditClientSecret: *redditClientSecret,
			TwitterBearerToken: *twitterBearerToken,
		},
		Publish:    *publish,
		S3Bucket:   *s3Bucket,
		S3Region:   *s3Region,
		S3Endpoint: *s3Endpoint,
	}

	// Set up Claude authentication in background if CLAUDE_TOKEN is provided
	if token := os.Getenv("CLAUDE_TOKEN"); token != "" {
		go setupClaudeAuth(token)
	}

	srv := NewServer(cfg)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %s\n", err)
		os.Exit(1)
	}
}

// parseMCPFlags handles the --mcp subprocess mode
func parseMCPFlags() {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	fs.Bool("mcp", false, "")

	serpAPIKey := fs.String("serp-api-key", "", "")
	googleAPIKey := fs.String("google-api-key", "", "")
	googleCSEID := fs.String("google-cse-id", "", "")
	gscCredentials := fs.String("gsc-credentials", "", "")
	perplexityKey := fs.String("perplexity-key", "", "")
	mozAPIKey := fs.String("moz-api-key", "", "")
	ahrefsAPIKey := fs.String("ahrefs-api-key", "", "")
	bingAPIKey := fs.String("bing-api-key", "", "")
	redditClientID := fs.String("reddit-client-id", "", "")
	redditClientSecret := fs.String("reddit-client-secret", "", "")
	twitterBearerToken := fs.String("twitter-bearer-token", "", "")

	fs.Parse(os.Args[1:])

	mcp.SetAPIKeys(*serpAPIKey, *googleAPIKey, *googleCSEID, *gscCredentials, *perplexityKey, *mozAPIKey, *ahrefsAPIKey, *bingAPIKey, *redditClientID, *redditClientSecret, *twitterBearerToken)

	if err := mcp.RunServer(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %s\n", err)
		os.Exit(1)
	}
}

// setupClaudeAuth checks if Claude is already authenticated, and if not, runs setup-token
func setupClaudeAuth(token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if already authenticated
	if exec.CommandContext(ctx, "claude", "auth", "status").Run() == nil {
		ui.PrintSuccess("Claude already authenticated")
		return
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	cmd := exec.CommandContext(ctx2, "claude", "setup-token")
	cmd.Stdin = strings.NewReader(token + "\n")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		ui.PrintWarning("Claude auth setup failed: %s", err)
	} else {
		ui.PrintSuccess("Claude authenticated via setup-token")
	}
}

// defaultPublish returns "s3" if AWS credentials are present, otherwise "local"
func defaultPublish() string {
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return "s3"
	}
	return "local"
}

// envOrDefault returns the env var value or a default
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
