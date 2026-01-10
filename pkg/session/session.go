package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SessionInfo represents metadata about a fetch session.
type SessionInfo struct {
	SessionID   string    `yaml:"session_id"`
	Created     time.Time `yaml:"created"`
	URLCount    int       `yaml:"url_count"`
	Success     int       `yaml:"success"`
	Failed      int       `yaml:"failed"`
	Features    []string  `yaml:"features,omitempty"`
	URLsPreview []string  `yaml:"urls_preview,omitempty"` // First 3 URLs
}

// SessionIndex represents the sessions/index.yaml file.
type SessionIndex struct {
	Sessions []SessionInfo `yaml:"sessions"`
}

// GenerateSessionID creates a timestamp-first session ID from a list of URLs.
// Format: YYYY-MM-DDTHH-MM-{hash}
// Hash is derived from the sorted, normalized URL list.
func GenerateSessionID(urls []string) string {
	// Sort and normalize URLs for consistent hashing
	normalized := make([]string, len(urls))
	copy(normalized, urls)
	sort.Strings(normalized)

	// Create hash from sorted URLs
	h := sha256.New()
	for _, url := range normalized {
		h.Write([]byte(url))
		h.Write([]byte("\n"))
	}
	hashBytes := h.Sum(nil)
	shortHash := hex.EncodeToString(hashBytes[:6]) // 12 char hex

	// Generate timestamp (minute precision)
	timestamp := time.Now().Format("2006-01-02T15-04")

	return fmt.Sprintf("%s-%s", timestamp, shortHash)
}

// GetSessionDir returns the full path to a session directory.
func GetSessionDir(baseDir, sessionID string) string {
	return filepath.Join(baseDir, "sessions", sessionID)
}

// GetSessionsIndexPath returns the path to the sessions index file (at results root).
func GetSessionsIndexPath(baseDir string) string {
	return filepath.Join(baseDir, "index.yaml")
}

// SessionExists checks if a session directory exists and has summary files.
func SessionExists(baseDir, sessionID string) bool {
	sessionDir := GetSessionDir(baseDir, sessionID)
	indexPath := filepath.Join(sessionDir, "summary-index.yaml")
	detailsPath := filepath.Join(sessionDir, "summary-details.yaml")

	// Check if both summary files exist
	_, err1 := os.Stat(indexPath)
	_, err2 := os.Stat(detailsPath)

	return err1 == nil && err2 == nil
}

// IsSessionFresh checks if a session is fresh based on max age.
// Returns true if the session's summary files are newer than maxAge.
func IsSessionFresh(baseDir, sessionID string, maxAge time.Duration) bool {
	if maxAge <= 0 {
		// No expiry - always fresh if it exists
		return SessionExists(baseDir, sessionID)
	}

	sessionDir := GetSessionDir(baseDir, sessionID)
	detailsPath := filepath.Join(sessionDir, "summary-details.yaml")

	info, err := os.Stat(detailsPath)
	if err != nil {
		return false
	}

	age := time.Since(info.ModTime())
	return age <= maxAge
}

// EnsureSessionDir creates the session directory structure if it doesn't exist.
func EnsureSessionDir(baseDir, sessionID string) error {
	sessionDir := GetSessionDir(baseDir, sessionID)
	sessionsRoot := filepath.Join(baseDir, "sessions")

	// Create sessions/ root
	if err := os.MkdirAll(sessionsRoot, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Create session-specific directory
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	return nil
}

// UpdateSessionIndex adds or updates a session entry in sessions/index.yaml.
func UpdateSessionIndex(baseDir string, info SessionInfo) error {
	indexPath := GetSessionsIndexPath(baseDir)

	// Read existing index
	var index SessionIndex
	data, err := os.ReadFile(indexPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read session index: %w", err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &index); err != nil {
			return fmt.Errorf("failed to parse session index: %w", err)
		}
	}

	// Check if session already exists in index
	found := false
	for i, s := range index.Sessions {
		if s.SessionID == info.SessionID {
			// Update existing entry
			index.Sessions[i] = info
			found = true
			break
		}
	}

	if !found {
		// Append new entry
		index.Sessions = append(index.Sessions, info)
	}

	// Sort sessions by session ID (timestamp-first naming ensures chronological order)
	sort.Slice(index.Sessions, func(i, j int) bool {
		return index.Sessions[i].SessionID > index.Sessions[j].SessionID // Newest first
	})

	// Write updated index
	output, err := yaml.Marshal(&index)
	if err != nil {
		return fmt.Errorf("failed to marshal session index: %w", err)
	}

	if err := os.WriteFile(indexPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write session index: %w", err)
	}

	return nil
}

// GetURLsPreview returns the first N URLs from a list for preview purposes.
func GetURLsPreview(urls []string, n int) []string {
	if len(urls) <= n {
		return urls
	}
	return urls[:n]
}

// FormatFeatures converts a features string (comma-separated) to a slice.
func FormatFeatures(features string) []string {
	if features == "" {
		return []string{"minimal"}
	}
	parts := strings.Split(features, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GenerateFieldsReference creates or updates the FIELDS.yaml reference file.
func GenerateFieldsReference(baseDir string) error {
	fieldsPath := filepath.Join(baseDir, "FIELDS.yaml")

	// Check if file already exists
	if _, err := os.Stat(fieldsPath); err == nil {
		// File exists, don't overwrite
		return nil
	}

	content := `# Summary Fields Reference (LLM-Optimized)
# Auto-generated field documentation for llm-web-parser output

fields:
  # Status & Basic Info
  url: string
  status: [success, failed]
  status_code: int (HTTP status)
  error: string (only if failed)

  # Basic Metadata
  title: string
  excerpt: string (description/summary)
  site_name: string
  author: string
  published_at: string (ISO-8601 date)

  # Domain Classification
  domain_type: [gov, edu, academic, commercial, mobile, unknown]
  domain_category: [gov/health, academic/ai, academic/general, news/tech, docs/api, blog, general]
  country: string (2-letter code or "unknown")
  confidence: float (0-10 quality/credibility score)

  # Academic Signals (boolean, only present if true)
  has_doi: bool
  has_arxiv: bool
  has_latex: bool
  has_citations: bool
  has_references: bool
  has_abstract: bool
  academic_score: float (0-10 composite academic signal strength)
  doi: string (DOI pattern if found)
  arxiv_id: string (ArXiv ID if found)

  # Content Metrics
  word_count: int
  estimated_tokens: int (word_count / 2.5)
  read_time_min: float
  section_count: int (number of sections/headings)
  block_count: int (number of content blocks)

  # Language Detection
  language: string (ISO-639-1 code: en, es, fr, de, etc)
  language_confidence: float (0-1)

  # Content Type
  content_type: [landing, article, documentation, unknown]
  extraction_mode: [minimal, cheap, full]

  # Visual Metadata
  has_favicon: bool (site has favicon)
  image_count: int (number of images detected)

  # HTTP Metadata
  final_url: string (after redirects)
  redirect_chain: [string] (list of redirect URLs)
  http_content_type: string (Content-Type header)

query_examples:
  - desc: Government health sites with high confidence
    yq: '.[] | select(.domain_category == "gov/health" and .confidence >= 7)'

  - desc: Academic papers with citations
    yq: '.[] | select(.has_citations and .academic_score >= 7)'

  - desc: Visual-heavy content (landing pages, galleries)
    yq: '.[] | select(.image_count > 10)'

  - desc: Technical documentation (low images, high structure)
    yq: '.[] | select(.image_count < 3 and .section_count > 5 and .domain_category == "docs/api")'

  - desc: Long-form content worth deep reading
    yq: '.[] | select(.word_count > 1000 and .read_time_min > 5 and .confidence >= 6)'

  - desc: Non-English content
    yq: '.[] | select(.language != "en" and .language_confidence > 0.8)'

  - desc: Failed fetches only
    yq: '.[] | select(.status == "failed")'

  - desc: Academic AI/ML papers
    yq: '.[] | select(.domain_category == "academic/ai" and .has_abstract)'

usage:
  summary_index: Minimal scannable data per session
  summary_details: Full enriched metadata per session
  location: llm-web-parser-results/sessions/{session-id}/
  session_index: llm-web-parser-results/index.yaml (list all sessions)
`

	if err := os.WriteFile(fieldsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write FIELDS.yaml: %w", err)
	}

	return nil
}
