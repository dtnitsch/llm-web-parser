package fetch

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/db"
	"gopkg.in/yaml.v3"
)

func BuildSummary(r Result) ResultSummary {
	summary := ResultSummary{
		URL:           r.URL,
		FilePath:      r.FilePath,
		FileSizeBytes: r.FileSizeBytes,
	}
	if r.Error != nil {
		summary.Status = "failed"
		summary.Error = r.Error.Error()
	} else {
		summary.Status = "success"
		summary.EstimatedTokens = int(math.Round(float64(r.Page.Metadata.WordCount) / 2.5))
		summary.ContentType = r.Page.Metadata.ContentType
		summary.ExtractionQuality = r.Page.Metadata.ExtractionQuality
		summary.ConfidenceDist = ComputeConfidenceDist(r.Page)
		summary.BlockTypeDist = ComputeBlockTypeDist(r.Page)
	}
	return summary
}

// buildSummaryIndex creates minimal index entry (only for successful fetches)
func BuildSummaryIndex(r Result) *SummaryIndex {
	if r.Error != nil {
		return nil // Only include successful fetches
	}

	return &SummaryIndex{
		URL:    r.URL,
		Cat:    r.Page.Metadata.DomainCategory,
		Conf:   r.Page.Metadata.Confidence,
		Title:  r.Page.Title,
		Desc:   r.Page.Metadata.Excerpt,
		Tokens: int(math.Round(float64(r.Page.Metadata.WordCount) / 2.5)),
	}
}

// buildSummaryDetails creates full details entry (all URLs)
func BuildSummaryDetails(r Result) SummaryDetails {
	details := SummaryDetails{
		URL:      r.URL,
		FilePath: r.FilePath,
	}

	if r.Error != nil {
		details.Status = "failed"
		details.Error = r.Error.Error()
		return details
	}

	details.Status = "success"
	meta := r.Page.Metadata

	// Basic metadata
	details.Title = r.Page.Title
	details.Excerpt = meta.Excerpt
	details.SiteName = meta.SiteName
	details.Author = meta.Author
	details.PublishedAt = meta.PublishedTime

	// Smart detection
	details.DomainType = meta.DomainType
	details.DomainCategory = meta.DomainCategory
	details.Country = meta.Country
	details.Confidence = meta.Confidence

	// Academic signals
	details.AcademicScore = meta.AcademicScore
	details.HasDOI = meta.HasDOI
	details.HasArXiv = meta.HasArXiv
	details.DOI = meta.DOIPattern
	details.ArXivID = meta.ArXivID
	details.HasLaTeX = meta.HasLaTeX
	details.HasCitations = meta.HasCitations
	details.HasReferences = meta.HasReferences
	details.HasAbstract = meta.HasAbstract

	// Content metrics
	details.WordCount = meta.WordCount
	details.EstimatedTokens = int(math.Round(float64(meta.WordCount) / 2.5))
	details.ReadTimeMin = meta.EstimatedReadMin
	details.Language = meta.Language
	details.LanguageConfidence = meta.LanguageConfidence
	details.ContentType = meta.ContentType
	details.ExtractionMode = string(meta.ExtractionMode)
	details.SectionCount = meta.SectionCount
	details.BlockCount = meta.BlockCount

	// Visual metadata (boolean/count only)
	details.HasFavicon = meta.Favicon != ""
	// NOTE: Image counting currently limited to featured/main image from metadata.
	// Future enhancement: Parse and count <img> tags from content blocks in full-parse mode.
	details.ImageCount = 0
	if meta.Image != "" {
		details.ImageCount = 1 // At minimum, we have the main image
	}

	// HTTP metadata
	details.StatusCode = meta.StatusCode
	details.FinalURL = meta.FinalURL
	details.RedirectChain = meta.RedirectChain
	details.HTTPContentType = meta.HTTPContentType

	return details
}

// writeSummaryIndexToSession writes the summary index to a session directory (file, not stdout)
func WriteSummaryIndexToSession(results []Result, sessionDir string) error {
	var index []SummaryIndex

	for _, r := range results {
		if entry := BuildSummaryIndex(r); entry != nil {
			index = append(index, *entry)
		}
	}

	// Create output path
	outputPath := filepath.Join(sessionDir, "summary-index.yaml")

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal index to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, yamlBytes, 0600); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return nil
}

// writeSummaryDetailsToSession writes the full details to a session directory
func WriteSummaryDetailsToSession(results []Result, sessionDir string, database *db.DB) error {
	details := make([]SummaryDetails, 0, len(results))

	for _, r := range results {
		detail := BuildSummaryDetails(r)
		// Add URL ID from database
		if urlID, err := database.GetURLID(r.URL); err == nil {
			detail.URLID = urlID
		}
		details = append(details, detail)
	}

	// Create output path
	outputPath := filepath.Join(sessionDir, "summary-details.yaml")

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed to marshal details to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, yamlBytes, 0600); err != nil {
		return fmt.Errorf("failed to write details file: %w", err)
	}

	return nil
}

func ComputeConfidenceDist(page *models.Page) map[string]int {
	dist := map[string]int{"high": 0, "medium": 0, "low": 0}
	if page == nil {
		return dist
	}
	for _, block := range page.AllTextBlocks() {
		switch {
		case block.Confidence >= 0.7:
			dist["high"]++
		case block.Confidence >= 0.5:
			dist["medium"]++
		default:
			dist["low"]++
		}
	}
	return dist
}

func ComputeBlockTypeDist(page *models.Page) map[string]int {
	dist := make(map[string]int)
	if page == nil {
		return dist
	}
	for _, block := range page.AllTextBlocks() {
		dist[block.Type]++
	}
	return dist
}

// collectFailedURLs extracts failed URLs from results and creates FailedURL objects.
func collectFailedURLs(results []Result) []FailedURL {
	var failed []FailedURL

	for _, r := range results {
		if r.Error != nil {
			failedURL := FailedURL{
				URL:          r.URL,
				StatusCode:   0, // Default to 0 for network errors
				ErrorType:    r.ErrorType,
				ErrorMessage: r.Error.Error(),
			}

			// Try to get status code if available from page metadata
			if r.Page != nil && r.Page.Metadata.StatusCode > 0 {
				failedURL.StatusCode = r.Page.Metadata.StatusCode
			}

			// Classify error type if not set
			if failedURL.ErrorType == "" {
				errMsg := strings.ToLower(r.Error.Error())
				switch {
				case strings.Contains(errMsg, "timeout"):
					failedURL.ErrorType = "timeout"
				case strings.Contains(errMsg, "parse") || strings.Contains(errMsg, "unmarshal"):
					failedURL.ErrorType = "parse_error"
				case strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network"):
					failedURL.ErrorType = "network_error"
				case failedURL.StatusCode >= 400:
					failedURL.ErrorType = "http_error"
				default:
					failedURL.ErrorType = "unknown_error"
				}
			}

			failed = append(failed, failedURL)
		}
	}

	return failed
}

// writeFailedURLsToSession writes failed URLs to failed-urls.yaml in the session directory.
func WriteFailedURLsToSession(failed []FailedURL, sessionDir string) error {
	if len(failed) == 0 {
		return nil // No failures, skip writing file
	}

	failedURLs := FailedURLs{
		FailedURLs: failed,
	}

	outputPath := filepath.Join(sessionDir, "failed-urls.yaml")

	yamlBytes, err := yaml.Marshal(&failedURLs)
	if err != nil {
		return fmt.Errorf("failed to marshal failed URLs to YAML: %w", err)
	}

	if err := os.WriteFile(outputPath, yamlBytes, 0600); err != nil {
		return fmt.Errorf("failed to write failed URLs file: %w", err)
	}

	return nil
}

// sessionsAction lists all sessions in a table format
