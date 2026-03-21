package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"optimus/core/lib/engine"
	"optimus/core/lib/publisher"
	"optimus/core/lib/render"
	"optimus/core/lib/scraper"
	"optimus/core/lib/ui"
)

// Job represents an async pipeline job
type Job struct {
	ID        string           `json:"id"`
	Status    string           `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Input     JobRequest       `json:"input"`
	Result    interface{}      `json:"result,omitempty"`
	Published *publisher.Result `json:"published,omitempty"`
	Error     string           `json:"error,omitempty"`
}

// JobRequest holds the input parameters for a job
type JobRequest struct {
	URL          string `json:"url"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Skill        string `json:"skill"`
	Count        int    `json:"count"`
	Depth        int    `json:"depth"`
	Timeout      int    `json:"timeout"`
	Instructions string `json:"instructions"`
}

// setDefaults fills in missing fields with sensible defaults
func (r *JobRequest) setDefaults() {
	if r.Skill == "" {
		r.Skill = "all"
	}
	if r.Count < 1 {
		r.Count = 1
	}
	if r.Depth < 1 {
		r.Depth = 3
	}
	if r.Timeout < 1 {
		r.Timeout = 120
	}
	if r.Name == "" {
		r.Name = deriveNameFromURL(r.URL)
	}
}

// normalizeURL ensures the URL has a scheme
func normalizeURL(raw string) string {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		if strings.HasPrefix(raw, "localhost") || strings.HasPrefix(raw, "127.0.0.1") {
			return "http://" + raw
		}
		return "https://" + raw
	}
	return raw
}

// deriveNameFromURL extracts a site name from a URL
func deriveNameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "site"
	}
	hostname := parsed.Hostname()
	hostname = strings.TrimPrefix(hostname, "www.")
	return strings.ReplaceAll(hostname, ".", "-")
}

// runJob executes the full pipeline for a job: scrape -> analyze -> render -> publish
func (s *Server) runJob(job *Job) {
	fmt.Println()
	ui.PrintInfo("Starting job %s [%s] %s", job.ID, job.Input.Skill, job.Input.URL)

	s.setStatus(job, "scraping")

	targetURL := job.Input.URL
	name := job.Input.Name

	// Set up working directory
	baseDir := filepath.Join(os.TempDir(), "optimus", "server", name, job.ID)
	scrapedDir := filepath.Join(baseDir, "scraped")
	if err := os.MkdirAll(scrapedDir, 0755); err != nil {
		s.failJob(job, fmt.Sprintf("creating work directory: %s", err))
		return
	}

	// Phase 1: Scrape
	scrapeResult, err := scraper.Scrape(scraper.Config{
		URL:       targetURL,
		OutputDir: scrapedDir,
		Timeout:   time.Duration(job.Input.Timeout) * time.Second,
		MaxPages:  job.Input.Count,
		MaxDepth:  job.Input.Depth,
	})
	if err != nil {
		s.failJob(job, fmt.Sprintf("scraping failed: %s", err))
		return
	}

	// Load shared assets for report branding
	cssBytes, _ := staticFiles.ReadFile("www/static/css/style.css")
	svgBytes, _ := staticFiles.ReadFile("www/static/images/logo.svg")
	css, logoSVG := string(cssBytes), string(svgBytes)

	cfg := engine.Config{
		SiteURL:            targetURL,
		ScrapedDir:         scrapedDir,
		Pages:              scrapeResult.Pages,
		OutputDir:          baseDir,
		Instructions:       job.Input.Instructions,
		Skill:              job.Input.Skill,
		SerpAPIKey:         s.config.EngineKeys.SerpAPIKey,
		GoogleAPIKey:       s.config.EngineKeys.GoogleAPIKey,
		GoogleCSEID:        s.config.EngineKeys.GoogleCSEID,
		GSCCredentials:     s.config.EngineKeys.GSCCredentials,
		PerplexityKey:      s.config.EngineKeys.PerplexityKey,
		MozAPIKey:          s.config.EngineKeys.MozAPIKey,
		AhrefsAPIKey:       s.config.EngineKeys.AhrefsAPIKey,
		BingAPIKey:         s.config.EngineKeys.BingAPIKey,
		RedditClientID:     s.config.EngineKeys.RedditClientID,
		RedditClientSecret: s.config.EngineKeys.RedditClientSecret,
		TwitterBearerToken: s.config.EngineKeys.TwitterBearerToken,
	}

	var renderResult *render.Result

	if job.Input.Skill == "all" {
		// Combined mode: run all skills
		s.setStatus(job, "analyzing")

		fullResult, err := engine.RunAll(cfg)
		if err != nil {
			s.failJob(job, fmt.Sprintf("analysis failed: %s", err))
			return
		}

		s.setStatus(job, "rendering")

		combinedCfg := render.CombinedConfig{
			SiteURL:   targetURL,
			Reports:   make(map[string]*engine.Report),
			Files:     make(map[string][]engine.FileEntry),
			Errors:    make(map[string]string),
			OutputDir: baseDir,
			CSS:       css,
			LogoSVG:   logoSVG,
			Version:   Version,
		}

		for _, sr := range fullResult.Skills {
			if sr.Skill == nil {
				continue
			}
			// Find the skill key
			skillKey := ""
			for _, name := range engine.AnalysisSkills() {
				sk, _ := engine.LoadSkill(name)
				if sk != nil && sk.Name == sr.Skill.Name {
					skillKey = name
					break
				}
			}
			if skillKey == "" {
				continue
			}

			if sr.Error != "" {
				combinedCfg.Errors[skillKey] = sr.Error
				continue
			}

			switch sr.Skill.Output {
			case "scorecard":
				scorecard, err := engine.ParseScorecard(sr.RawOutput)
				if err != nil {
					combinedCfg.Errors[skillKey] = fmt.Sprintf("parse error: %s", err)
				} else {
					combinedCfg.Scorecard = scorecard
				}
			case "report":
				report, err := engine.ParseReport(sr.RawOutput)
				if err != nil {
					combinedCfg.Errors[skillKey] = fmt.Sprintf("parse error: %s", err)
				} else {
					combinedCfg.Reports[skillKey] = report
				}
			case "backlinks":
				strategy, err := engine.ParseBacklinks(sr.RawOutput)
				if err != nil {
					combinedCfg.Errors[skillKey] = fmt.Sprintf("parse error: %s", err)
				} else {
					combinedCfg.Backlinks = strategy
				}
			case "files":
				files, err := engine.ParseFiles(sr.RawOutput)
				if err != nil {
					combinedCfg.Errors[skillKey] = fmt.Sprintf("parse error: %s", err)
				} else {
					combinedCfg.Files[skillKey] = files
				}
			}
		}

		s.setResult(job, combinedCfg)
		renderResult, err = render.GenerateCombined(combinedCfg)
		if err != nil {
			s.failJob(job, fmt.Sprintf("rendering combined report: %s", err))
			return
		}
	} else {
		// Single skill mode
		s.setStatus(job, "analyzing")

		result, err := engine.Run(cfg)
		if err != nil {
			s.failJob(job, fmt.Sprintf("analysis failed: %s", err))
			return
		}

		s.setStatus(job, "rendering")

		switch result.Skill.Output {
		case "report":
			report, err := engine.ParseReport(result.RawOutput)
			if err != nil {
				s.failJob(job, fmt.Sprintf("parsing report: %s", err))
				return
			}
			s.setResult(job, report)
			renderResult, err = render.Generate(render.Config{Report: report, OutputDir: baseDir, CSS: css, LogoSVG: logoSVG, Version: Version})
			if err != nil {
				s.failJob(job, fmt.Sprintf("rendering report: %s", err))
				return
			}

		case "scorecard":
			scorecard, err := engine.ParseScorecard(result.RawOutput)
			if err != nil {
				s.failJob(job, fmt.Sprintf("parsing scorecard: %s", err))
				return
			}
			s.setResult(job, scorecard)
			renderResult, err = render.GenerateScorecard(render.ScorecardConfig{Scorecard: scorecard, OutputDir: baseDir, CSS: css, LogoSVG: logoSVG, Version: Version})
			if err != nil {
				s.failJob(job, fmt.Sprintf("rendering scorecard: %s", err))
				return
			}

		case "backlinks":
			strategy, err := engine.ParseBacklinks(result.RawOutput)
			if err != nil {
				s.failJob(job, fmt.Sprintf("parsing backlinks: %s", err))
				return
			}
			s.setResult(job, strategy)
			renderResult, err = render.GenerateBacklinks(render.BacklinksConfig{Strategy: strategy, OutputDir: baseDir, CSS: css, LogoSVG: logoSVG, Version: Version})
			if err != nil {
				s.failJob(job, fmt.Sprintf("rendering backlinks: %s", err))
				return
			}

		case "files":
			files, err := engine.ParseFiles(result.RawOutput)
			if err != nil {
				s.failJob(job, fmt.Sprintf("parsing files: %s", err))
				return
			}
			s.setResult(job, files)
			outputDir := filepath.Join(baseDir, "output")
			renderResult, err = render.GenerateFiles(render.FilesConfig{
				Files:     files,
				SiteURL:   targetURL,
				SkillName: result.Skill.Name,
				OutputDir: outputDir,
				CSS:       css,
				LogoSVG:   logoSVG,
				Version:   Version,
			})
			if err != nil {
				s.failJob(job, fmt.Sprintf("rendering files: %s", err))
				return
			}

		default:
			s.failJob(job, fmt.Sprintf("unknown skill output type: %s", result.Skill.Output))
			return
		}
	}

	// Phase 4: Publish
	s.setStatus(job, "publishing")

	pub := s.createPublisher(name)
	pubResult, err := pub.Publish(renderResult.HTMLPath, renderResult.JSONPath)
	if err != nil {
		s.failJob(job, fmt.Sprintf("publishing failed: %s", err))
		return
	}

	s.mu.Lock()
	job.Published = pubResult
	job.Status = "completed"
	job.UpdatedAt = time.Now()
	s.mu.Unlock()

	elapsed := time.Since(job.CreatedAt).Truncate(time.Second)
	ui.PrintSuccess("Finished job %s [%s] %s (%s)", job.ID, job.Input.Skill, job.Input.URL, elapsed)
	fmt.Print("  ")
	ui.PrintKeyValue("Report", pubResult.HTMLURL)
	fmt.Println()
}

// createPublisher returns a Publisher based on server config
func (s *Server) createPublisher(siteName string) publisher.Publisher {
	switch s.config.Publish {
	case "s3":
		pub, err := publisher.NewS3(s.config.S3Bucket, s.config.S3Region, s.config.S3Endpoint, siteName)
		if err != nil {
			return publisher.NewLocal()
		}
		return pub
	default:
		return publisher.NewLocal()
	}
}

// setStatus updates a job's status under lock
func (s *Server) setStatus(job *Job, status string) {
	s.mu.Lock()
	job.Status = status
	job.UpdatedAt = time.Now()
	s.mu.Unlock()
}

// setResult updates a job's result under lock
func (s *Server) setResult(job *Job, result interface{}) {
	s.mu.Lock()
	job.Result = result
	s.mu.Unlock()
}

// failJob marks a job as failed with an error message
func (s *Server) failJob(job *Job, errMsg string) {
	s.mu.Lock()
	job.Status = "failed"
	job.Error = errMsg
	job.UpdatedAt = time.Now()
	s.mu.Unlock()

	elapsed := time.Since(job.CreatedAt).Truncate(time.Second)
	ui.PrintError("Failed job %s [%s] %s (%s): %s", job.ID, job.Input.Skill, job.Input.URL, elapsed, errMsg)
}
