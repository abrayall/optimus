package publisher

import (
	"fmt"
	"path/filepath"
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
