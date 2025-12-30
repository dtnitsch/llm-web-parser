package manifest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/mapreduce"
	"github.com/dtnitsch/llm-web-parser/pkg/storage"
)

// FetchResult represents the result of fetching and parsing a single URL.
// This is passed from main.go to avoid circular dependencies.
type FetchResult struct {
	URL        string
	FilePath   string
	Page       *models.Page
	Error      error
	ErrorType  string
	WordCounts map[string]int
	FileSizeBytes int64 // Cached file size to avoid redundant os.Stat() calls
}

// GenerateSummary creates a summary manifest file with aggregated results.
// Ig accepts all fetch results, aggregate keywords, and a storage instance.
// Returns the path to the generated manifest file and any error.
func GenerateSummary(results []FetchResult, aggregateKeywords map[string]int, s *storage.Storage) (string, error) {
	manifest := SummaryManifest{
		GeneratedAt:       time.Now().Format(time.RFC3339),
		TotalURLs:         len(results),
		AggregateKeywords: mapreduce.TopKeywords(aggregateKeywords, 25),
	}

	// Process each result
	for _, result := range results {
		summary := URLSummary{
			URL: result.URL,
		}

		if result.Error != nil {
			// Error case
			manifest.Failed++
			summary.Status = "error"
			summary.ErrorType = result.ErrorType
			summary.ErrorMessage = result.Error.Error()
		} else {
			// Success case
			manifest.Successful++
			summary.Status = "success"
			summary.FilePath = result.FilePath

			// Extract page metadata
			if result.Page != nil {
				summary.WordCount = result.Page.Metadata.WordCount
				summary.EstimatedTokens = int(float64(summary.WordCount) / 2.5)
				summary.ExtractionQuality = result.Page.Metadata.ExtractionQuality
			}

			// Get file stats (size, mod time) using storage layer
			if result.FilePath != "" {
				fsb := result.FileSizeBytes
				if fsb == 0 {
					// Backup incase results didn't set correctly
					stats, err := s.GetFileStats(result.FilePath)
					if err == nil {
						summary.SizeBytes = stats.SizeBytes
					}
				}
				summary.SizeBytes = fsb
			}

			// Add top keywords for this URL
			if result.WordCounts != nil {
				summary.TopKeywords = mapreduce.TopKeywords(result.WordCounts, 25)
			}
		}

		manifest.Results = append(manifest.Results, summary)
	}

	// Save manifest to file
	manifestPath := fmt.Sprintf("results/summary-%s.json", time.Now().Format("2006-01-02"))
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshalling manifest: %w", err)
	}

	if err := s.SaveFile(manifestPath, manifestData); err != nil {
		return "", fmt.Errorf("error saving manifest: %w", err)
	}

	return manifestPath, nil
}
