package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"optimus/core/lib/engine"
)

// handleHealth responds with a simple health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleListSkills returns the available analysis skills
func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	names := engine.ListSkills()
	type skillInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Output      string `json:"output"`
	}
	var skills []skillInfo
	for _, name := range names {
		sk, err := engine.LoadSkill(name)
		if err != nil {
			continue
		}
		skills = append(skills, skillInfo{
			Name:        sk.Name,
			Description: sk.Description,
			Output:      sk.Output,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"skills": skills})
}

// handleCreateJob validates input, creates a job, and launches the pipeline goroutine
func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	req.URL = normalizeURL(req.URL)
	req.setDefaults()

	// Validate skill
	if _, err := engine.LoadSkill(req.Skill); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "unknown skill: " + req.Skill,
			"skills": joinSkills(),
		})
		return
	}

	id := generateID()
	now := time.Now()
	job := &Job{
		ID:        id,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
		Input:     req,
	}

	s.mu.Lock()
	s.jobs[id] = job
	s.mu.Unlock()

	go s.runJob(job)

	writeJSON(w, http.StatusAccepted, map[string]string{"id": id, "status": "pending"})
}

// handleListJobs returns a summary of all jobs
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type jobSummary struct {
		ID        string    `json:"id"`
		Status    string    `json:"status"`
		URL       string    `json:"url"`
		Skill     string    `json:"skill"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Error     string    `json:"error,omitempty"`
	}

	jobs := make([]jobSummary, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, jobSummary{
			ID:        j.ID,
			Status:    j.Status,
			URL:       j.Input.URL,
			Skill:     j.Input.Skill,
			CreatedAt: j.CreatedAt,
			UpdatedAt: j.UpdatedAt,
			Error:     j.Error,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"jobs": jobs})
}

// handleGetJob returns full details for a single job
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.RLock()
	job, ok := s.jobs[id]
	s.mu.RUnlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// handleDeleteJob removes a job (cancellation is best-effort since goroutines aren't stopped)
func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	_, ok := s.jobs[id]
	if ok {
		delete(s.jobs, id)
	}
	s.mu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// writeJSON marshals v as JSON and writes it to w
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// generateID creates a random 8-byte hex string
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// joinSkills returns a comma-separated list of available skills
func joinSkills() string {
	names := engine.ListSkills()
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}
