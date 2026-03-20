package publisher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Publisher uploads report files to an S3-compatible bucket
type S3Publisher struct {
	bucket   string
	region   string
	endpoint string
	siteName string
	client   *s3.Client
}

// NewS3 creates an S3Publisher using the default AWS credential chain.
// If endpoint is non-empty, it overrides the default AWS S3 endpoint (for Wasabi, MinIO, etc.).
func NewS3(bucket, region, endpoint, siteName string) (*S3Publisher, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}

	return &S3Publisher{
		bucket:   bucket,
		region:   region,
		endpoint: endpoint,
		siteName: siteName,
		client:   s3.NewFromConfig(cfg, s3Opts...),
	}, nil
}

// Publish uploads HTML and JSON files to S3 and returns browser-openable URLs
func (p *S3Publisher) Publish(htmlPath, jsonPath string) (*Result, error) {
	timestamp := time.Now().UTC().Format("2006-01-02-150405")
	prefix := fmt.Sprintf("%s/%s", p.siteName, timestamp)

	result := &Result{}

	if htmlPath != "" {
		key := prefix + "/" + filepath.Base(htmlPath)
		if err := p.upload(htmlPath, key, "text/html"); err != nil {
			return nil, fmt.Errorf("uploading HTML: %w", err)
		}
		result.HTMLURL = p.url(key)
	}

	if jsonPath != "" {
		key := prefix + "/" + filepath.Base(jsonPath)
		if err := p.upload(jsonPath, key, "application/json"); err != nil {
			return nil, fmt.Errorf("uploading JSON: %w", err)
		}
		result.JSONURL = p.url(key)
	}

	return result, nil
}

func (p *S3Publisher) upload(filePath, key, contentType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filePath, err)
	}
	defer f.Close()

	_, err = p.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String(contentType),
		ACL:         types.ObjectCannedACLPublicRead,
	})
	return err
}

func (p *S3Publisher) url(key string) string {
	if p.endpoint != "" {
		base := strings.TrimRight(p.endpoint, "/")
		return fmt.Sprintf("%s/%s/%s", base, p.bucket, key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", p.bucket, p.region, key)
}
