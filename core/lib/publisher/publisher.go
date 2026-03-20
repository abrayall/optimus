package publisher

// Result holds the published URLs for browser access
type Result struct {
	HTMLURL string // browser-openable URL (file:// or https://)
	JSONURL string
}

// Publisher publishes rendered report files to a destination
type Publisher interface {
	Publish(htmlPath, jsonPath string) (*Result, error)
}
