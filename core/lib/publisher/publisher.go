package publisher

import "time"

// Result holds the published URLs for browser access
type Result struct {
	HTMLURL string // browser-openable URL (file:// or https://)
	JSONURL string
}

// Publisher publishes rendered report files to a destination
type Publisher interface {
	Publish(htmlPath, jsonPath string) (*Result, error)
}

// ReportFile is a single report file with a name and URL
type ReportFile struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ScanEntry represents one scan run (a timestamp directory)
type ScanEntry struct {
	ID        string       `json:"id"`
	Timestamp time.Time    `json:"timestamp"`
	Reports   []ReportFile `json:"reports"`
}

// SiteScans groups all scans for a given site
type SiteScans struct {
	Name  string      `json:"name"`
	Scans []ScanEntry `json:"scans"`
}

// Lister can enumerate previously published scans
type Lister interface {
	ListScans() ([]SiteScans, error)
}
