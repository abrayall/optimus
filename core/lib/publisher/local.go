package publisher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalPublisher returns file:// URLs pointing to existing local paths
type LocalPublisher struct{}

// NewLocal creates a new LocalPublisher
func NewLocal() *LocalPublisher {
	return &LocalPublisher{}
}

// Publish returns file:// URLs for the given paths
func (p *LocalPublisher) Publish(htmlPath, jsonPath string) (*Result, error) {
	result := &Result{}

	if htmlPath != "" {
		abs, err := filepath.Abs(htmlPath)
		if err != nil {
			return nil, fmt.Errorf("resolving HTML path: %w", err)
		}
		result.HTMLURL = "file://" + abs
	}

	if jsonPath != "" {
		abs, err := filepath.Abs(jsonPath)
		if err != nil {
			return nil, fmt.Errorf("resolving JSON path: %w", err)
		}
		result.JSONURL = "file://" + abs
	}

	return result, nil
}

// ListScans walks the local server output directory and returns all published scans
func (p *LocalPublisher) ListScans() ([]SiteScans, error) {
	baseDir := filepath.Join(os.TempDir(), "optimus", "server")

	siteEntries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading base dir: %w", err)
	}

	var sites []SiteScans
	for _, siteEntry := range siteEntries {
		if !siteEntry.IsDir() {
			continue
		}

		siteName := siteEntry.Name()
		siteDir := filepath.Join(baseDir, siteName)

		jobEntries, err := os.ReadDir(siteDir)
		if err != nil {
			continue
		}

		site := SiteScans{Name: siteName}
		for _, jobEntry := range jobEntries {
			if !jobEntry.IsDir() {
				continue
			}

			jobID := jobEntry.Name()
			jobDir := filepath.Join(siteDir, jobID)

			info, err := jobEntry.Info()
			if err != nil {
				continue
			}

			entry := ScanEntry{ID: jobID, Timestamp: info.ModTime()}

			// Walk the job directory for report files (html/json)
			filepath.Walk(jobDir, func(path string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(fi.Name()))
				if ext == ".html" || ext == ".json" {
					entry.Reports = append(entry.Reports, ReportFile{
						Name: fi.Name(),
						URL:  "file://" + path,
					})
				}
				return nil
			})

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
