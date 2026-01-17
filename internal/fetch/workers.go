package fetch

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/dtnitsch/llm-web-parser/internal/common"
	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/analytics"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	"github.com/dtnitsch/llm-web-parser/pkg/corpus"
	"github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/dtnitsch/llm-web-parser/pkg/extractor"
	"github.com/dtnitsch/llm-web-parser/pkg/extractors"
	"github.com/dtnitsch/llm-web-parser/pkg/fetcher"
	"github.com/dtnitsch/llm-web-parser/pkg/mapreduce"
	"github.com/dtnitsch/llm-web-parser/pkg/parser"
	"gopkg.in/yaml.v3"
)

// formatKeywordsAsJSON formats word counts as JSON array for database storage.
// Uses existing mapreduce.TopKeywords() to get top N keywords.
func formatKeywordsAsJSON(counts map[string]int, limit int) string {
	keywords := mapreduce.TopKeywords(counts, limit)
	jsonBytes, err := json.Marshal(keywords)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

// formatWordCountsSorted formats word counts as sorted plain text.
// Format: "word:count\n" sorted by count descending for easy parsing.
func formatWordCountsSorted(counts map[string]int) string {
	type kv struct {
		word  string
		count int
	}

	sorted := make([]kv, 0, len(counts))
	for w, c := range counts {
		sorted = append(sorted, kv{word: w, count: c})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	var sb strings.Builder
	for _, item := range sorted {
		fmt.Fprintf(&sb, "%s:%d\n", item.word, item.count)
	}
	return sb.String()
}

func run(logger *slog.Logger, config *models.FetchConfig, manager *artifact_manager.Manager, forceFetch bool, parseMode models.ParseMode, filterStrategy *extractor.Strategy, database *db.DB) ([]Result, map[string]int, error) {
	f := fetcher.NewFetcher()
	p := &parser.Parser{}
	a := &analytics.Analytics{}

	logger.Info("Starting concurrent fetch phase", "url_count", len(config.URLs), "workers", config.WorkerCount, "force_fetch", forceFetch, "max_age", manager.MaxAge())
	var wg sync.WaitGroup
	jobs := make(chan Job, len(config.URLs))
	results := make(chan Result, len(config.URLs))

	for w := 1; w <= config.WorkerCount; w++ {
		wg.Add(1)
		go worker(w, logger, manager, f, p, a, &wg, jobs, results, forceFetch, filterStrategy, database)
	}

	for _, rawURL := range config.URLs {
		jobs <- Job{URL: rawURL, ParseMode: parseMode}
	}
	close(jobs)

	wg.Wait()
	close(results)
	logger.Info("All fetch workers finished")

	allResults := make([]Result, 0, len(config.URLs))
	var runErr error
	for result := range results {
		allResults = append(allResults, result)
		if result.Error != nil {
			runErr = fmt.Errorf("one or more jobs failed")
		}
		if result.Page != nil && !result.Page.Metadata.Computed {
			result.Page.ComputeMetadata()
		}
	}

	logger.Info("Starting MapReduce phase")
	intermediateResults := []map[string]int{}
	for _, result := range allResults {
		if result.WordCounts != nil {
			intermediateResults = append(intermediateResults, result.WordCounts)
		}
	}
	finalWordCounts := mapreduce.Reduce(intermediateResults)

	return allResults, finalWordCounts, runErr
}

func processHTML(id int, logger *slog.Logger, url string, rawHTML []byte, manager *artifact_manager.Manager, p *parser.Parser, a *analytics.Analytics, results chan<- Result, parseMode models.ParseMode, filterStrategy *extractor.Strategy, database *db.DB, urlID int64) {
	result := Result{URL: url}

	page, parseErr := p.Parse(models.ParseRequest{
		URL:  url,
		HTML: string(rawHTML),
		Mode: parseMode,
	})
	if parseErr != nil {
		logger.Error("Error parsing HTML", "worker_id", id, "url", url, "error", parseErr)
		result.Error = parseErr
		result.ErrorType = "parse_error"
		results <- result
		return
	}

	// Apply filter if provided
	if filterStrategy != nil && (filterStrategy.MinConfidence > 0 || len(filterStrategy.BlockTypes) > 0) {
		page = extractor.FilterPage(page, filterStrategy)
	}

	wordCounts := mapreduce.Map(page.ToPlainText(), a)
	result.WordCounts = wordCounts

	// Marshal to YAML for generic.yaml
	yamlData, marshalErr := yaml.Marshal(page)
	if marshalErr != nil {
		logger.Error("Error marshalling YAML", "worker_id", id, "url", url, "error", marshalErr)
		result.Error = marshalErr
		result.ErrorType = "marshal_error"
		result.Page = page
		results <- result
		return
	}

	// Store parsed YAML using URL-centric storage
	if database != nil && urlID > 0 {
		if setParsedErr := manager.SetParsedYAMLByID(urlID, yamlData); setParsedErr != nil {
			logger.Warn("Failed to store parsed YAML artifact", "url", url, "error", setParsedErr)
		}

		// Write full wordcount as sorted text file
		wordcountPath := filepath.Join(artifact_manager.GetURLDir(artifact_manager.DefaultBaseDir, urlID), "wordcount.txt")
		sortedWordcounts := formatWordCountsSorted(result.WordCounts)
		if err := os.WriteFile(wordcountPath, []byte(sortedWordcounts), 0644); err != nil {
			logger.Warn("Failed to write wordcount.txt", "url", url, "error", err)
		}

		// Insert parsed YAML artifact into database
		parsedTypeID, err := database.GetArtifactTypeID("yaml_parsed")
		if err != nil {
			logger.Warn("Failed to get yaml_parsed type ID", "url", url, "error", err)
		} else {
			hash := common.ContentHash(yamlData)
			parsedPath := artifact_manager.GetURLArtifactPath("", urlID, "generic.yaml")
			result.FilePath = parsedPath
			_, err = database.InsertArtifact(urlID, parsedTypeID, hash, parsedPath, int64(len(yamlData)))
			if err != nil {
				logger.Warn("Failed to insert parsed artifact to DB", "url", url, "error", err)
			}
		}

		// Update content type metadata in database
		contentInfo := db.ContentTypeInfo{
			ContentType:         db.NewNullString(page.Metadata.ContentType),
			ContentSubtype:      db.NewNullString(page.Metadata.ContentSubtype),
			DetectionConfidence: db.NewNullFloat64(page.Metadata.Confidence),
			HasAbstract:         page.Metadata.HasAbstract,
			HasInfobox:          page.Metadata.HasInfobox,
			HasTOC:              page.Metadata.HasTOC,
			HasCodeExamples:     page.Metadata.HasCodeExamples,
			SectionCount:        page.Metadata.SectionCount,
			CitationCount:       page.Metadata.CitationCount,
			CodeBlockCount:      page.Metadata.CodeBlockCount,
			TopKeywords:         db.NewNullString(formatKeywordsAsJSON(result.WordCounts, 25)),
		}
		if err := database.UpdateURLContentType(urlID, contentInfo); err != nil {
			logger.Warn("Failed to update content type metadata", "url", url, "error", err)
		}

		// Write metadata.yaml file for corpus queries
		if err := corpus.WriteMetadataFile(database, urlID, artifact_manager.DefaultBaseDir); err != nil {
			logger.Warn("Failed to write metadata file", "url", url, "error", err)
		}

		// Run specialized extractors based on content type
		runSpecializedExtractors(logger, page, urlID, manager)
	}

	result.FileSizeBytes = int64(len(yamlData))
	result.Page = page
	results <- result
	logger.Info("Worker finished processing", "worker_id", id, "url", url)
}

func worker(id int, logger *slog.Logger, manager *artifact_manager.Manager, f *fetcher.Fetcher, p *parser.Parser, a *analytics.Analytics, wg *sync.WaitGroup, jobs <-chan Job, results chan<- Result, forceFetch bool, filterStrategy *extractor.Strategy, database *db.DB) {
	defer wg.Done()
	for job := range jobs {
		logger.Info("Worker started job", "worker_id", id, "url", job.URL)

		var rawHTML []byte
		var err error
		var fresh bool
		var urlID int64
		var statusCode int

		// Insert or get URL ID from database
		if database != nil {
			urlID, err = database.InsertURL(job.URL)
			if err != nil {
				logger.Warn("Failed to insert URL to DB", "url", job.URL, "error", err)
			}
		}

		if !forceFetch {
			rawHTML, fresh, err = manager.GetRawHTML(job.URL)
			if err != nil {
				logger.Warn("Error checking artifact storage, fetching fresh", "url", job.URL, "error", err)
			}
		}

		if fresh {
			logger.Info("Raw HTML found in storage, using it", "worker_id", id, "url", job.URL)
			statusCode = 200 // Assume success from cache
		} else {
			logger.Info("Raw HTML not found or stale, fetching from network", "worker_id", id, "url", job.URL)
			rawHTML, err = f.GetHtmlBytes(job.URL)
			if err != nil {
				result := Result{URL: job.URL}
				logger.Error("Error fetching HTML", "worker_id", id, "url", job.URL, "error", err)
				result.Error = err
				result.ErrorType = "fetch_error"

				// Record failed access in database
				if database != nil && urlID > 0 {
					if dbErr := database.RecordAccess(urlID, 0, "fetch_error", false); dbErr != nil {
						logger.Warn("Failed to record failed access to DB", "url", job.URL, "error", dbErr)
					}
				}

				results <- result
				continue
			}
			statusCode = 200 // Successful fetch

			// Store raw HTML using URL-centric storage
			if database != nil && urlID > 0 {
				if err := manager.SetRawHTMLByID(urlID, rawHTML); err != nil {
					logger.Warn("Failed to store raw HTML artifact", "url", job.URL, "error", err)
				}

				// Insert raw HTML artifact into database
				rawTypeID, err := database.GetArtifactTypeID("html_raw")
				if err != nil {
					logger.Warn("Failed to get html_raw type ID", "url", job.URL, "error", err)
				} else {
					hash := common.ContentHash(rawHTML)
					rawPath := artifact_manager.GetURLArtifactPath("", urlID, "raw.html")
					_, err = database.InsertArtifact(urlID, rawTypeID, hash, rawPath, int64(len(rawHTML)))
					if err != nil {
						logger.Warn("Failed to insert raw artifact to DB", "url", job.URL, "error", err)
					}
				}
			}
		}

		// Record successful access in database
		if database != nil && urlID > 0 {
			if dbErr := database.RecordAccess(urlID, statusCode, "", true); dbErr != nil {
				logger.Warn("Failed to record access to DB", "url", job.URL, "error", dbErr)
			}
		}

		processHTML(id, logger, job.URL, rawHTML, manager, p, a, results, job.ParseMode, filterStrategy, database, urlID)
	}
}

// parseFeaturesFlag converts features string to ParseMode

// runSpecializedExtractors runs content-type-specific extractors and saves results.
func runSpecializedExtractors(logger *slog.Logger, page *models.Page, urlID int64, manager *artifact_manager.Manager) {
	if page == nil || page.Metadata.ContentType == "" {
		return
	}

	// Only run for full parse mode (skip minimal mode)
	if page.Metadata.ExtractionMode != "full" {
		return
	}

	contentType := page.Metadata.ContentType

	switch contentType {
	case "academic":
		extractAcademicContent(logger, page, urlID, manager)
	case "docs":
		extractDocsContent(logger, page, urlID, manager)
	case "wiki":
		extractWikiContent(logger, page, urlID, manager)
	}
}

// extractAcademicContent runs academic extractor and saves results.
func extractAcademicContent(logger *slog.Logger, page *models.Page, urlID int64, manager *artifact_manager.Manager) {
	extraction := extractors.ExtractAcademic(page)
	if extraction == nil {
		return
	}

	// Save to lwp-results/{url_id}/academic.yaml
	yamlData, err := yaml.Marshal(extraction)
	if err != nil {
		logger.Warn("Failed to marshal academic extraction", "url_id", urlID, "error", err)
		return
	}

	if err := manager.EnsureURLDir(urlID); err != nil {
		logger.Warn("Failed to ensure URL directory", "url_id", urlID, "error", err)
		return
	}

	filePath := artifact_manager.GetURLArtifactPath("", urlID, "academic.yaml")
	if err := os.WriteFile(filePath, yamlData, 0600); err != nil {
		logger.Warn("Failed to write academic extraction", "url_id", urlID, "error", err)
	} else {
		logger.Info("Saved academic extraction", "url_id", urlID, "file", filePath)
	}
}

// extractDocsContent runs docs extractor and saves results.
func extractDocsContent(logger *slog.Logger, page *models.Page, urlID int64, manager *artifact_manager.Manager) {
	extraction := extractors.ExtractDocs(page)
	if extraction == nil {
		return
	}

	// Save to lwp-results/{url_id}/docs.yaml
	yamlData, err := yaml.Marshal(extraction)
	if err != nil {
		logger.Warn("Failed to marshal docs extraction", "url_id", urlID, "error", err)
		return
	}

	if err := manager.EnsureURLDir(urlID); err != nil {
		logger.Warn("Failed to ensure URL directory", "url_id", urlID, "error", err)
		return
	}

	filePath := artifact_manager.GetURLArtifactPath("", urlID, "docs.yaml")
	if err := os.WriteFile(filePath, yamlData, 0600); err != nil {
		logger.Warn("Failed to write docs extraction", "url_id", urlID, "error", err)
	} else {
		logger.Info("Saved docs extraction", "url_id", urlID, "file", filePath)
	}
}

// extractWikiContent runs wiki extractor and saves results.
func extractWikiContent(logger *slog.Logger, page *models.Page, urlID int64, manager *artifact_manager.Manager) {
	extraction := extractors.ExtractWiki(page)
	if extraction == nil {
		return
	}

	// Save to lwp-results/{url_id}/wiki.yaml
	yamlData, err := yaml.Marshal(extraction)
	if err != nil {
		logger.Warn("Failed to marshal wiki extraction", "url_id", urlID, "error", err)
		return
	}

	if err := manager.EnsureURLDir(urlID); err != nil {
		logger.Warn("Failed to ensure URL directory", "url_id", urlID, "error", err)
		return
	}

	filePath := artifact_manager.GetURLArtifactPath("", urlID, "wiki.yaml")
	if err := os.WriteFile(filePath, yamlData, 0600); err != nil {
		logger.Warn("Failed to write wiki extraction", "url_id", urlID, "error", err)
	} else {
		logger.Info("Saved wiki extraction", "url_id", urlID, "file", filePath)
	}
}
