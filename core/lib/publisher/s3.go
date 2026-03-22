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

// ListScans enumerates all sites, timestamps, and report files in the bucket
func (p *S3Publisher) ListScans() ([]SiteScans, error) {
	ctx := context.Background()

	// List site prefixes (top-level "directories")
	sitesOut, err := p.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(p.bucket),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return nil, fmt.Errorf("listing sites: %w", err)
	}

	var sites []SiteScans
	for _, prefix := range sitesOut.CommonPrefixes {
		siteName := strings.TrimSuffix(*prefix.Prefix, "/")

		// List timestamp prefixes under this site
		tsOut, err := p.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:    aws.String(p.bucket),
			Prefix:    prefix.Prefix,
			Delimiter: aws.String("/"),
		})
		if err != nil {
			continue
		}

		site := SiteScans{Name: siteName}
		for _, tsPrefix := range tsOut.CommonPrefixes {
			tsStr := strings.TrimSuffix(strings.TrimPrefix(*tsPrefix.Prefix, siteName+"/"), "/")
			ts, err := time.Parse("2006-01-02-150405", tsStr)
			if err != nil {
				ts = time.Time{}
			}

			// List files under this timestamp
			filesOut, err := p.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket: aws.String(p.bucket),
				Prefix: tsPrefix.Prefix,
			})
			if err != nil {
				continue
			}

			entry := ScanEntry{ID: tsStr, Timestamp: ts}
			for _, obj := range filesOut.Contents {
				name := strings.TrimPrefix(*obj.Key, *tsPrefix.Prefix)
				if name == "" {
					continue
				}
				entry.Reports = append(entry.Reports, ReportFile{
					Name: name,
					URL:  p.url(*obj.Key),
				})
			}
			if len(entry.Reports) > 0 {
				site.Scans = append(site.Scans, entry)
			}
		}
		if len(site.Scans) > 0 {
			sites = append(sites, site)
		}
	}

	return sites, nil
}
