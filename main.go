package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/analytics"
	"github.com/dtnitsch/llm-web-parser/pkg/fetcher"
	"github.com/dtnitsch/llm-web-parser/pkg/manifest"
	"github.com/dtnitsch/llm-web-parser/pkg/mapreduce"
	"github.com/dtnitsch/llm-web-parser/pkg/parser"
	"github.com/dtnitsch/llm-web-parser/pkg/storage"
)

// Job defines a task for a worker to perform.
type Job struct {
	URL string
}

// Result holds the outcome of a processed job.
type Result struct {
	URL           string
	FilePath      string
	Page          *models.Page
	Error         error
	ErrorType     string
	WordCounts    map[string]int
	FileSizeBytes int64 // Cached file size to avoid redundant os.Stat() calls
}

func main() {
	multiUrls()
}

// worker is a goroutine that processes jobs from the jobs channel
// and sends results to the results channel.
func worker(id int, f *fetcher.Fetcher, s *storage.Storage, p *parser.Parser, a *analytics.Analytics, wg *sync.WaitGroup, jobs <-chan Job, results chan<- Result) {
	defer wg.Done()
	for job := range jobs {
		log.Printf("Worker %d started job for URL: %s", id, job.URL)
		fn := getSavePath(job.URL)
		result := Result{
			URL:      job.URL,
			FilePath: fn,
		}

		// For structured JSON, we don't want to use the cache of flat text files.
		// A more advanced caching system would be needed. For now, we re-fetch.
		// if s.HasFile(fn) { ... }

		log.Printf("Worker %d: Fetching URL: %s", id, job.URL)
		html, err := f.GetHtml(job.URL)
		if err != nil {
			log.Printf("Worker %d: Error fetching HTML for %s: %s", id, job.URL, err)
			result.Error = err
			result.ErrorType = "fetch_error"
			result.FilePath = ""
			results <- result
			continue // Get next job
		}

		// Parse the HTML into a structured Page object
		page, err := p.Parse(models.ParseRequest{
			URL:  job.URL,
			HTML: html.String(),
			Mode: models.ParseModeFull,
			//Mode: models.ParseModeCheap,
		})
		if err != nil {
			log.Printf("Worker %d: Error parsing HTML for %s: %s", id, job.URL, err)
			result.Error = err
			result.ErrorType = "parse_error"
			result.FilePath = ""
			results <- result
			continue
		}

		// Compute per-URL word counts
		wordCounts := mapreduce.Map(page.ToPlainText(), a)
		result.WordCounts = wordCounts

		// Marshal the Page object into indented JSON
		jsonData, err := json.MarshalIndent(page, "", "  ")
		if err != nil {
			log.Printf("Worker %d: Error marshalling JSON for %s: %s", id, job.URL, err)
			result.Error = err
			result.ErrorType = "marshal_error"
			result.FilePath = ""
			result.Page = page
			results <- result
			continue
		}

		// Save the JSON data
		if err := s.SaveFile(fn, jsonData); err != nil {
			log.Printf("Worker %d: Error saving file [%s]: %s", id, fn, err)
			result.Error = err
			result.ErrorType = "save_error"
			result.Page = page
			results <- result
			continue // Get next job
		}

		// Cache file size to avoid redundant os.Stat() calls during manifest generation
		result.FileSizeBytes = int64(len(jsonData))
		result.Page = page
		results <- result
		log.Printf("Worker %d finished job for URL: %s", id, job.URL)
	}
}

func multiUrls() {
	config, err := models.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize packages
	f := fetcher.NewFetcher()
	s := &storage.Storage{}
	p := &parser.Parser{}
	a := &analytics.Analytics{}

	// --- Concurrent Fetch Phase ---
	fmt.Println("--- Starting Concurrent Fetch Phase ---")
	var wg sync.WaitGroup
	jobs := make(chan Job, len(config.URLs))
	results := make(chan Result, len(config.URLs))

	// Start workers
	var workerCount = 4
	if config.WorkerCount > 0 {
		workerCount = config.WorkerCount
	}
	for w := 1; w <= workerCount; w++ {
		wg.Add(1)
		go worker(w, f, s, p, a, &wg, jobs, results)
	}

	// Send jobs to the workers
	for _, rawURL := range config.URLs {
		jobs <- Job{URL: rawURL}
	}
	close(jobs)

	// Wait for all workers to finish, then close the results channel
	wg.Wait()
	close(results)
	fmt.Println("--- All fetch workers finished ---")

	// Collect results
	var allResults []Result
	var allPages []*models.Page
	for result := range results {
		allResults = append(allResults, result)
		if result.Page != nil {
			allPages = append(allPages, result.Page)
		}
	}

	// --- MapReduce Phase ---
	// The analytics still run on the flat text version of the content.
	fmt.Println("\n--- Starting MapReduce Phase ---")

	// 1. Map Stage (aggregate across all URLs)
	intermediateResults := []map[string]int{}
	for _, result := range allResults {
		if result.WordCounts != nil {
			intermediateResults = append(intermediateResults, result.WordCounts)
		}
	}
	fmt.Printf("Map phase complete. Generated %d intermediate frequency maps.\n", len(intermediateResults))

	// 2. Reduce Stage
	finalWordCounts := mapreduce.Reduce(intermediateResults)
	fmt.Println("Reduce phase complete. Aggregated all word counts.")

	// 3. Present Results
	fmt.Println("\n--- Top 25 Words (Aggregated) ---")
	mapreduce.PrintTopKeywords(finalWordCounts, 25)

	// 4. Generate summary manifest
	fmt.Println("\n--- Generating Summary Manifest ---")
	manifestResults := convertToManifestResults(allResults)
	manifestPath, err := manifest.GenerateSummary(manifestResults, finalWordCounts, s)
	if err != nil {
		log.Printf("Error generating summary manifest: %s", err)
	} else {
		fmt.Printf("Summary manifest saved to: %s\n", manifestPath)
	}
}

// getSavePath generates a filesystem-friendly path from a URL.
func getSavePath(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// Fallback for invalid URLs
		safeString := strings.ReplaceAll(rawURL, "https://", "")
		safeString = strings.ReplaceAll(safeString, "http://", "")
		safeString = strings.ReplaceAll(safeString, "/", "_")
		return fmt.Sprintf("results/%s-%s.json", safeString, time.Now().Format("2006-01-02"))
	}

	// Sanitize host
	host := strings.ReplaceAll(parsedURL.Host, ".", "_")

	// Sanitize path to avoid collisions (e.g., github.com/cli/cli vs github.com/urfave/cli)
	path := strings.Trim(parsedURL.Path, "/")
	path = strings.ReplaceAll(path, "/", "-")
	path = strings.ReplaceAll(path, ".", "_")

	// Combine host + path
	var base string
	if path != "" {
		base = fmt.Sprintf("%s-%s", host, path)
	} else {
		base = host
	}

	today := time.Now().Format("2006-01-02")
	return fmt.Sprintf("results/%s-%s.json", base, today)
}

// convertToManifestResults converts main.go Result types to manifest.FetchResult.
// This adapter function prevents circular dependencies between packages.
func convertToManifestResults(results []Result) []manifest.FetchResult {
	manifestResults := make([]manifest.FetchResult, len(results))
	for i, r := range results {
		manifestResults[i] = manifest.FetchResult{
			URL:           r.URL,
			FilePath:      r.FilePath,
			Page:          r.Page,
			Error:         r.Error,
			ErrorType:     r.ErrorType,
			WordCounts:    r.WordCounts,
			FileSizeBytes: r.FileSizeBytes,
		}
	}
	return manifestResults
}
