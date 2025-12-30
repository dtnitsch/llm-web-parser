package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/analytics"
	"github.com/dtnitsch/llm-web-parser/pkg/fetcher"
	"github.com/dtnitsch/llm-web-parser/pkg/mapreduce"
	"github.com/dtnitsch/llm-web-parser/pkg/parser"
	"github.com/dtnitsch/llm-web-parser/pkg/storage"
)

const numWorkers = 4

// Job defines a task for a worker to perform.
type Job struct {
	URL string
}

// Result holds the outcome of a processed job.
type Result struct {
	Page *models.Page
}

func main() {
	multiUrls()
}

// worker is a goroutine that processes jobs from the jobs channel
// and sends results to the results channel.
func worker(id int, f *fetcher.Fetcher, s *storage.Storage, p *parser.Parser, wg *sync.WaitGroup, jobs <-chan Job, results chan<- Result) {
	defer wg.Done()
	for job := range jobs {
		log.Printf("Worker %d started job for URL: %s", id, job.URL)
		fn := getSavePath(job.URL)
		var page *models.Page
		var err error

		// For structured JSON, we don't want to use the cache of flat text files.
		// A more advanced caching system would be needed. For now, we re-fetch.
		// if s.HasFile(fn) { ... }

		log.Printf("Worker %d: Fetching URL: %s", id, job.URL)
		html, err := f.GetHtml(job.URL)
		if err != nil {
			log.Printf("Worker %d: Error fetching HTML for %s: %s", id, job.URL, err)
			continue // Get next job
		}

		// Parse the HTML into a structured Page object
		page, err = p.Parse(models.ParseRequest{
			URL: job.URL,
			HTML: html.String(),
			Mode: models.ParseModeFull,
			//Mode: models.ParseModeCheap,
		})
		if err != nil {
			log.Printf("Worker %d: Error parsing HTML for %s: %s", id, job.URL, err)
			continue
		}

		// Marshal the Page object into indented JSON
		jsonData, err := json.MarshalIndent(page, "", "  ")
		if err != nil {
			log.Printf("Worker %d: Error marshalling JSON for %s: %s", id, job.URL, err)
			continue
		}

		// Save the JSON data
		if err := s.SaveFile(fn, jsonData); err != nil {
			log.Printf("Worker %d: Error saving file [%s]: %s", id, fn, err)
			continue // Get next job
		}

		results <- Result{Page: page}
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
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go worker(w, f, s, p, &wg, jobs, results)
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
	var allPages []*models.Page
	for result := range results {
		allPages = append(allPages, result.Page)
	}

	if len(allPages) == 0 {
		log.Println("No content was fetched or processed. Exiting.")
		return
	}

	// --- MapReduce Phase ---
	// The analytics still run on the flat text version of the content.
	fmt.Println("\n--- Starting MapReduce Phase ---")

	// 1. Map Stage
	intermediateResults := []map[string]int{}
	for _, page := range allPages {
		counts := mapreduce.Map(page.ToPlainText(), a)
		intermediateResults = append(intermediateResults, counts)
	}
	fmt.Printf("Map phase complete. Generated %d intermediate frequency maps.\n", len(intermediateResults))

	// 2. Reduce Stage
	finalWordCounts := mapreduce.Reduce(intermediateResults)
	fmt.Println("Reduce phase complete. Aggregated all word counts.")

	// 3. Present Results
	fmt.Println("\n--- Top 25 Words (Aggregated) ---")
	printSortedMap(finalWordCounts, 25)
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
	host := strings.ReplaceAll(parsedURL.Host, ".", "_")
	today := time.Now().Format("2006-01-02")
	return fmt.Sprintf("results/%s-%s.json", host, today)
}

// printSortedMap sorts a map by value (desc) and prints the top N results.
func printSortedMap(data map[string]int, topN int) {
	// Convert map to slice of structs for sorting
	type kv struct {
		Key   string
		Value int
	}
	var ss []kv
	for k, v := range data {
		ss = append(ss, kv{k, v})
	}

	// Sort slice by value in descending order
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})

	// Print the top N results
	limit := topN
	if len(ss) < topN {
		limit = len(ss)
	}
	if limit < 0 {
		limit = 0
	}

	for i := 0; i < limit; i++ {
		fmt.Printf("%d. %s: %d\n", i+1, ss[i].Key, ss[i].Value)
	}
}
