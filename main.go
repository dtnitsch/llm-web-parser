package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/analytics"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	"github.com/dtnitsch/llm-web-parser/pkg/extractor"
	"github.com/dtnitsch/llm-web-parser/pkg/fetcher"
	"github.com/dtnitsch/llm-web-parser/pkg/manifest"
	"github.com/dtnitsch/llm-web-parser/pkg/mapreduce"
	"github.com/dtnitsch/llm-web-parser/pkg/parser"
	"github.com/dtnitsch/llm-web-parser/pkg/storage"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
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
	FileSizeBytes int64
}

// ResultOutput is the structured output for a single URL.
type ResultOutput struct {
	URL       string `json:"url"`
	FilePath  string `json:"file_path,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"error_type,omitempty"`
}

// ResultSummary holds detailed summary data for a single processed URL.
type ResultSummary struct {
	URL                 string            `json:"url"`
	FilePath            string            `json:"file_path,omitempty"`
	Status              string            `json:"status"`
	Error               string            `json:"error,omitempty"`
	FileSizeBytes       int64             `json:"file_size_bytes,omitempty"`
	EstimatedTokens     int               `json:"estimated_tokens,omitempty"`
	ContentType         string            `json:"content_type,omitempty"`
	ExtractionQuality   string            `json:"extraction_quality,omitempty"`
	ConfidenceDist      map[string]int    `json:"confidence_distribution,omitempty"`
	BlockTypeDist       map[string]int    `json:"block_type_distribution,omitempty"`
}

// FinalOutput is the structured output for the entire run.
type FinalOutput struct {
	Status  string      `json:"status"`
	Results interface{} `json:"results"`
	Stats   Stats       `json:"stats"`
}

// Stats provides summary statistics for the run.
type Stats struct {
	TotalURLs        int      `json:"total_urls"`
	Successful       int      `json:"successful"`
	Failed           int      `json:"failed"`
	TotalTimeSeconds float64  `json:"total_time_seconds"`
	TopKeywords      []string `json:"top_keywords,omitempty"`
}

func main() {
	// Will be overridden in commands based on --quiet flag
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	app := &cli.App{
		Name:  "llm-web-parser",
		Usage: "A CLI tool to fetch and parse web content for LLMs.",
		Commands: []*cli.Command{
			{
				Name:   "fetch",
				Usage:  "Fetch and parse URLs",
				Action: fetchAction,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "quiet",
						Usage: "Suppress log output",
					},
					&cli.StringFlag{
						Name:    "urls",
						Usage:   "Comma-separated list of URLs to process",
						Aliases: []string{"u"},
					},
					&cli.IntFlag{
						Name:    "workers",
						Usage:   "Number of concurrent workers",
						Aliases: []string{"w"},
						Value:   8,
					},
					&cli.StringFlag{
						Name:    "format",
						Usage:   "Output format (json or yaml)",
						Aliases: []string{"f"},
						Value:   "json",
					},
					&cli.StringFlag{
						Name:  "output-mode",
						Usage: "Output mode (summary, full, minimal)",
						Value: "summary",
					},
					&cli.StringFlag{
						Name:  "max-age",
						Usage: "Maximum age for raw HTML artifacts (e.g., '1h', '0s' to always fetch fresh)",
						Value: "1h",
					},
					&cli.BoolFlag{
						Name:  "force-fetch",
						Usage: "Force fetching all URLs, ignoring max-age and existing artifacts",
					},
					&cli.StringFlag{
						Name:    "config",
						Usage:   "Path to the configuration file",
						Aliases: []string{"c"},
						Value:   "config.yaml",
					},
					&cli.StringFlag{
						Name:  "output-dir",
						Usage: "Base directory for storing raw and parsed artifacts",
						Value: artifact_manager.DefaultBaseDir,
					},
				},
			},
			{
				Name:   "extract",
				Usage:  "Extract filtered content from existing JSON results",
				Action: extractAction, // Will be implemented next
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "from",
						Usage:   "Path or glob pattern to one or more parsed JSON files",
						Aliases: []string{"i"}, // for input
					},
					&cli.StringFlag{
						Name:    "strategy",
						Usage:   "Filtering strategy (e.g., 'conf:>=0.7,type:code|table')",
						Aliases: []string{"s"},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Error("application exited with an error", "error", err)
		os.Exit(1)
	}
}

func extractAction(c *cli.Context) error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	from := c.StringSlice("from")
	strategyStr := c.String("strategy")

	if len(from) == 0 {
		return fmt.Errorf("no input files provided with --from flag")
	}

	strategy, err := extractor.ParseStrategy(strategyStr)
	if err != nil {
		return fmt.Errorf("failed to parse strategy: %w", err)
	}

	var filePaths []string
	for _, pattern := range from {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			logger.Warn("error matching glob pattern, skipping", "pattern", pattern, "error", err)
			continue
		}
		filePaths = append(filePaths, matches...)
	}

	if len(filePaths) == 0 {
		return fmt.Errorf("no files found matching glob patterns")
	}

	logger.Info("extracting from files", "count", len(filePaths), "strategy", strategyStr)

	allFilteredPages := []*models.Page{}

	for _, path := range filePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("failed to read file, skipping", "path", path, "error", err)
			continue
		}

		var page models.Page
		if err := json.Unmarshal(data, &page); err != nil {
			logger.Warn("failed to unmarshal JSON, skipping", "path", path, "error", err)
			continue
		}

		filteredPage := extractor.FilterPage(&page, strategy)
		allFilteredPages = append(allFilteredPages, filteredPage)
	}

	outputData, err := json.MarshalIndent(allFilteredPages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal final filtered output: %w", err)
	}
	fmt.Println(string(outputData))

	return nil
}

func fetchAction(c *cli.Context) error {
	logLevel := slog.LevelInfo
	if c.Bool("quiet") {
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	startTime := time.Now()
	finalOutput := &FinalOutput{}

	var maxAge time.Duration
	var err error
	if c.Bool("force-fetch") {
		maxAge = 0
	} else {
		maxAge, err = time.ParseDuration(c.String("max-age"))
		if err != nil {
			logger.Error("invalid max-age duration", "error", err)
			os.Exit(2)
		}
	}

	manager, err := artifact_manager.NewManager(c.String("output-dir"), maxAge)
	if err != nil {
		logger.Error("failed to initialize artifact manager", "error", err)
		os.Exit(2)
	}

	config, err := models.LoadConfig(c.String("config"))
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error("failed to load config", "error", err)
			os.Exit(2)
		}
		config = &models.Config{}
	}

	if c.IsSet("urls") {
		config.URLs = strings.Split(c.String("urls"), ",")
	}
	if c.IsSet("workers") {
		config.WorkerCount = c.Int("workers")
	}

	if len(config.URLs) == 0 {
		cli.ShowAppHelpAndExit(c, 1)
		return fmt.Errorf("no URLs provided via --urls flag or config file")
	}

	allResults, finalWordCounts, runErr := run(logger, config, manager, c.Bool("force-fetch"))

	stats := Stats{
		TotalURLs:        len(config.URLs),
		TotalTimeSeconds: time.Since(startTime).Seconds(),
		TopKeywords:      mapreduce.TopKeywords(finalWordCounts, 25),
	}

	outputMode := strings.ToLower(c.String("output-mode"))
	switch outputMode {
	case "summary":
		summaryResults := []ResultSummary{}
		for _, r := range allResults {
			summary := buildSummary(r)
			summaryResults = append(summaryResults, summary)
			if r.Error != nil {
				stats.Failed++
			} else {
				stats.Successful++
			}
		}
		finalOutput.Results = summaryResults
	default:
		legacyResults := []ResultOutput{}
		for _, r := range allResults {
			legacy := ResultOutput{URL: r.URL, FilePath: r.FilePath}
			if r.Error != nil {
				stats.Failed++
				legacy.Status = "failed"
				legacy.Error = r.Error.Error()
				legacy.ErrorType = r.ErrorType
			} else {
				stats.Successful++
				legacy.Status = "success"
			}
			legacyResults = append(legacyResults, legacy)
		}
		finalOutput.Results = legacyResults
	}

	finalOutput.Stats = stats
	if runErr != nil {
		finalOutput.Status = "partial_failure"
	} else {
		finalOutput.Status = "success"
	}

	var outputData []byte
	var marshalErr error
	outputFormat := strings.ToLower(c.String("format"))
	if outputFormat == "yaml" {
		outputData, marshalErr = yaml.Marshal(finalOutput)
	} else {
		outputData, marshalErr = json.MarshalIndent(finalOutput, "", "  ")
	}

	if marshalErr != nil {
		logger.Error("failed to marshal final output", "error", marshalErr)
		os.Exit(2)
	}
	fmt.Println(string(outputData))

	if stats.Failed == stats.TotalURLs {
		os.Exit(2)
	}
	if stats.Failed > 0 {
		os.Exit(1)
	}

	return nil
}

func run(logger *slog.Logger, config *models.Config, manager *artifact_manager.Manager, forceFetch bool) ([]Result, map[string]int, error) {
	f := fetcher.NewFetcher()
	p := &parser.Parser{}
	a := &analytics.Analytics{}

	logger.Info("Starting concurrent fetch phase", "url_count", len(config.URLs), "workers", config.WorkerCount, "force_fetch", forceFetch, "max_age", manager.MaxAge())
	var wg sync.WaitGroup
	jobs := make(chan Job, len(config.URLs))
	results := make(chan Result, len(config.URLs))

	for w := 1; w <= config.WorkerCount; w++ {
		wg.Add(1)
		go worker(w, logger, manager, f, p, a, &wg, jobs, results, forceFetch)
	}

	for _, rawURL := range config.URLs {
		jobs <- Job{URL: rawURL}
	}
	close(jobs)

	wg.Wait()
	close(results)
	logger.Info("All fetch workers finished")

	var allResults []Result
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

	logger.Info("Generating legacy summary manifest")
	legacyStorage := &storage.Storage{}
	manifestResults := convertToManifestResults(allResults)
	manifestPath, err := manifest.GenerateSummary(manifestResults, finalWordCounts, legacyStorage)
	if err != nil {
		logger.Warn("Error generating legacy summary manifest", "error", err)
	} else {
		logger.Info("Legacy summary manifest saved", "path", manifestPath)
	}

	return allResults, finalWordCounts, runErr
}

func processHTML(id int, logger *slog.Logger, url string, rawHTML []byte, manager *artifact_manager.Manager, p *parser.Parser, a *analytics.Analytics, results chan<- Result) {
	result := Result{URL: url}

	page, parseErr := p.Parse(models.ParseRequest{
		URL:  url,
		HTML: string(rawHTML),
		Mode: models.ParseModeFull,
	})
	if parseErr != nil {
		logger.Error("Error parsing HTML", "worker_id", id, "url", url, "error", parseErr)
		result.Error = parseErr
		result.ErrorType = "parse_error"
		results <- result
		return
	}

	wordCounts := mapreduce.Map(page.ToPlainText(), a)
	result.WordCounts = wordCounts

	jsonData, marshalErr := json.MarshalIndent(page, "", "  ")
	if marshalErr != nil {
		logger.Error("Error marshalling JSON", "worker_id", id, "url", url, "error", marshalErr)
		result.Error = marshalErr
		result.ErrorType = "marshal_error"
		result.Page = page
		results <- result
		return
	}

	if setParsedErr := manager.SetParsedJSON(url, jsonData); setParsedErr != nil {
		logger.Warn("Failed to store parsed JSON artifact", "url", url, "error", setParsedErr)
	}
	result.FilePath, _ = manager.GetArtifactPath(artifact_manager.ParsedJSONDir, url, ".json")

	result.FileSizeBytes = int64(len(jsonData))
	result.Page = page
	results <- result
	logger.Info("Worker finished processing", "worker_id", id, "url", url)
}

func worker(id int, logger *slog.Logger, manager *artifact_manager.Manager, f *fetcher.Fetcher, p *parser.Parser, a *analytics.Analytics, wg *sync.WaitGroup, jobs <-chan Job, results chan<- Result, forceFetch bool) {
	defer wg.Done()
	for job := range jobs {
		logger.Info("Worker started job", "worker_id", id, "url", job.URL)

		var rawHTML []byte
		var err error
		var fresh bool

		if !forceFetch {
			rawHTML, fresh, err = manager.GetRawHTML(job.URL)
			if err != nil {
				logger.Warn("Error checking artifact storage, fetching fresh", "url", job.URL, "error", err)
			}
		}

		if fresh {
			logger.Info("Raw HTML found in storage, using it", "worker_id", id, "url", job.URL)
		} else {
			logger.Info("Raw HTML not found or stale, fetching from network", "worker_id", id, "url", job.URL)
			rawHTML, err = f.GetHtmlBytes(job.URL)
			if err != nil {
				result := Result{URL: job.URL}
				logger.Error("Error fetching HTML", "worker_id", id, "url", job.URL, "error", err)
				result.Error = err
				result.ErrorType = "fetch_error"
				results <- result
				continue
			}

			if err := manager.SetRawHTML(job.URL, rawHTML); err != nil {
				logger.Warn("Failed to store raw HTML artifact", "url", job.URL, "error", err)
			}
		}

		processHTML(id, logger, job.URL, rawHTML, manager, p, a, results)
	}
}

func buildSummary(r Result) ResultSummary {
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
		summary.ConfidenceDist = computeConfidenceDist(r.Page)
		summary.BlockTypeDist = computeBlockTypeDist(r.Page)
	}
	return summary
}

func computeConfidenceDist(page *models.Page) map[string]int {
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

func computeBlockTypeDist(page *models.Page) map[string]int {
	dist := make(map[string]int)
	if page == nil {
		return dist
	}
	for _, block := range page.AllTextBlocks() {
		dist[block.Type]++
	}
	return dist
}

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
