// Package main provides the llm-web-parser CLI tool for extracting and analyzing web content.
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
	"github.com/dtnitsch/llm-web-parser/pkg/mapreduce"
	"github.com/dtnitsch/llm-web-parser/pkg/parser"
	"github.com/dtnitsch/llm-web-parser/pkg/session"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// Job defines a task for a worker to perform.
type Job struct {
	URL       string
	ParseMode models.ParseMode
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

// ResultSummaryTerse is the token-optimized v2 format with abbreviated field names.
type ResultSummaryTerse struct {
	URL               string         `json:"u"`
	FilePath          string         `json:"p,omitempty"`
	Status            int            `json:"s"`                // 0=success, 1=failed
	Error             string         `json:"e,omitempty"`
	FileSizeBytes     int64          `json:"sz,omitempty"`
	EstimatedTokens   int            `json:"tk,omitempty"`
	ContentType       string         `json:"ct,omitempty"`     // l=landing, a=article, d=docs, u=unknown
	ExtractionQuality int            `json:"q,omitempty"`      // 1=ok, 0=low, -1=degraded
	ConfidenceDist    [3]int         `json:"cd,omitempty"`     // [high, medium, low] fixed order
	BlockTypeDist     map[string]int `json:"bd,omitempty"`
}

// StatsTerse is the token-optimized v2 stats format.
type StatsTerse struct {
	Total   int      `json:"t"`
	Success int      `json:"ok"`
	Failed  int      `json:"f"`
	Time    float64  `json:"ts"`
	Keywords []string `json:"kw,omitempty"`
}

// FinalOutputTerse is the v2 terse output wrapper.
type FinalOutputTerse struct {
	Status  string               `json:"s"`
	Results []ResultSummaryTerse `json:"r"`
	Stats   StatsTerse           `json:"st"`
}

// SummaryIndex is the ultra-minimal, scannable index format (~150 bytes/URL).
// Only includes successful fetches (200, 301).
type SummaryIndex struct {
	URL    string  `yaml:"url"`
	Cat    string  `yaml:"cat"`              // domain_category
	Conf   float64 `yaml:"conf"`             // confidence 0-10
	Title  string  `yaml:"title,omitempty"`
	Desc   string  `yaml:"desc,omitempty"`   // excerpt
	Tokens int     `yaml:"tokens,omitempty"` // estimated_tokens
}

// SummaryDetails contains full enriched metadata for decision making (~400 bytes/URL).
// Includes all URLs (successful and failed).
type SummaryDetails struct {
	URL        string  `yaml:"url"`
	FilePath   string  `yaml:"file_path,omitempty"`
	Status     string  `yaml:"status"` // success, failed
	StatusCode int     `yaml:"status_code,omitempty"`
	Error      string  `yaml:"error,omitempty"`

	// Basic metadata
	Title       string `yaml:"title,omitempty"`
	Excerpt     string `yaml:"excerpt,omitempty"`
	SiteName    string `yaml:"site_name,omitempty"`
	Author      string `yaml:"author,omitempty"`
	PublishedAt string `yaml:"published_at,omitempty"`

	// Smart detection
	DomainType     string  `yaml:"domain_type,omitempty"`
	DomainCategory string  `yaml:"domain_category,omitempty"`
	Country        string  `yaml:"country,omitempty"`
	Confidence     float64 `yaml:"confidence,omitempty"`

	// Academic signals
	AcademicScore  float64 `yaml:"academic_score,omitempty"`
	HasDOI         bool    `yaml:"has_doi,omitempty"`
	HasArXiv       bool    `yaml:"has_arxiv,omitempty"`
	DOI            string  `yaml:"doi,omitempty"`
	ArXivID        string  `yaml:"arxiv_id,omitempty"`
	HasLaTeX       bool    `yaml:"has_latex,omitempty"`
	HasCitations   bool    `yaml:"has_citations,omitempty"`
	HasReferences  bool    `yaml:"has_references,omitempty"`
	HasAbstract    bool    `yaml:"has_abstract,omitempty"`

	// Content metrics
	WordCount          int     `yaml:"word_count,omitempty"`
	EstimatedTokens    int     `yaml:"estimated_tokens,omitempty"`
	ReadTimeMin        float64 `yaml:"read_time_min,omitempty"`
	Language           string  `yaml:"language,omitempty"`
	LanguageConfidence float64 `yaml:"language_confidence,omitempty"`
	ContentType        string  `yaml:"content_type,omitempty"`
	ExtractionMode     string  `yaml:"extraction_mode,omitempty"`
	SectionCount       int     `yaml:"section_count,omitempty"`
	BlockCount         int     `yaml:"block_count,omitempty"`

	// Visual metadata (boolean/count only, not URLs)
	HasFavicon bool `yaml:"has_favicon,omitempty"`
	ImageCount int  `yaml:"image_count,omitempty"`

	// HTTP metadata
	FinalURL      string   `yaml:"final_url,omitempty"`
	RedirectChain []string `yaml:"redirect_chain,omitempty"`
	HTTPContentType string `yaml:"http_content_type,omitempty"`
}

// toTerseStatus converts status string to int (0=success, 1=failed).
func toTerseStatus(status string) int {
	if status == "success" {
		return 0
	}
	return 1
}

// toTerseContentType converts content_type to single char (l=landing, a=article, d=docs, u=unknown).
func toTerseContentType(ct string) string {
	switch ct {
	case "landing":
		return "l"
	case "article":
		return "a"
	case "documentation":
		return "d"
	default:
		return "u"
	}
}

// toTerseQuality converts extraction_quality to int (1=ok, 0=low, -1=degraded).
func toTerseQuality(q string) int {
	switch q {
	case "ok":
		return 1
	case "low":
		return 0
	case "degraded":
		return -1
	default:
		return 0
	}
}

// toTerseResult converts ResultSummary to ResultSummaryTerse.
func toTerseResult(r ResultSummary) ResultSummaryTerse {
	return ResultSummaryTerse{
		URL:               r.URL,
		FilePath:          r.FilePath,
		Status:            toTerseStatus(r.Status),
		Error:             r.Error,
		FileSizeBytes:     r.FileSizeBytes,
		EstimatedTokens:   r.EstimatedTokens,
		ContentType:       toTerseContentType(r.ContentType),
		ExtractionQuality: toTerseQuality(r.ExtractionQuality),
		ConfidenceDist:    [3]int{r.ConfidenceDist["high"], r.ConfidenceDist["medium"], r.ConfidenceDist["low"]},
		BlockTypeDist:     r.BlockTypeDist,
	}
}

// toTerseStats converts Stats to StatsTerse.
func toTerseStats(s Stats) StatsTerse {
	return StatsTerse{
		Total:    s.TotalURLs,
		Success:  s.Successful,
		Failed:   s.Failed,
		Time:     s.TotalTimeSeconds,
		Keywords: s.TopKeywords,
	}
}

// fieldNameMap maps verbose field names to terse equivalents.
var fieldNameMap = map[string]string{
	"url":                     "u",
	"file_path":               "p",
	"status":                  "s",
	"error":                   "e",
	"file_size_bytes":         "sz",
	"estimated_tokens":        "tk",
	"content_type":            "ct",
	"extraction_quality":      "q",
	"confidence_distribution": "cd",
	"block_type_distribution": "bd",
}

// filterResultFields filters a result struct to include only specified fields.
// Works with both ResultSummary (v1) and ResultSummaryTerse (v2).
func filterResultFields(result interface{}, fieldsStr string, isTerse bool) map[string]interface{} {
	if fieldsStr == "" {
		// No filtering, convert to map and return all fields
		return structToMap(result)
	}

	requestedFields := strings.Split(fieldsStr, ",")
	for i := range requestedFields {
		requestedFields[i] = strings.TrimSpace(requestedFields[i])
	}

	// Build set of fields to include (translate verbose->terse if needed)
	includeFields := make(map[string]bool)
	for _, field := range requestedFields {
		if isTerse {
			// If terse mode, check if user provided verbose name and translate
			if terseField, ok := fieldNameMap[field]; ok {
				includeFields[terseField] = true
			} else {
				// User already provided terse name
				includeFields[field] = true
			}
		} else {
			includeFields[field] = true
		}
	}

	// Convert struct to map
	fullMap := structToMap(result)

	// Filter map
	filtered := make(map[string]interface{})
	for key, value := range fullMap {
		if includeFields[key] {
			filtered[key] = value
		}
	}

	return filtered
}

// structToMap converts a struct to map[string]interface{} using JSON marshaling.
func structToMap(obj interface{}) map[string]interface{} {
	data, _ := json.Marshal(obj)
	var result map[string]interface{}
	_ = json.Unmarshal(data, &result)
	return result
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
						Usage: "Suppress log output (default: true, use --quiet=false for verbose logs)",
						Value: true,
					},
					&cli.StringFlag{
						Name:  "features",
						Usage: "Comma-separated list of features to enable (full-parse, wordcount). Default: minimal mode (metadata only)",
						Value: "",
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
						Usage:   "Output format (json or yaml). Default: yaml (more token-efficient)",
						Aliases: []string{"f"},
						Value:   "yaml",
					},
					&cli.StringFlag{
						Name:  "output-mode",
						Usage: "Output mode (tier2, summary, full, minimal). Default: tier2 (index to stdout + details file)",
						Value: "tier2",
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
					&cli.StringFlag{
						Name:  "summary-version",
						Usage: "Summary output format version (v1=verbose, v2=terse)",
						Value: "v1",
					},
					&cli.StringFlag{
						Name:  "summary-fields",
						Usage: "Comma-separated list of fields to include in summary (e.g., 'url,tokens,quality'). Empty = all fields.",
						Value: "",
					},
				},
			},
			{
				Name:   "extract",
				Usage:  "Extract filtered content from existing JSON results",
				Action: extractAction,
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
			{
				Name:   "analyze",
				Usage:  "Parse cached HTML on-demand with specified features",
				Action: analyzeAction,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "urls",
						Usage:   "Comma-separated list of URLs to re-analyze from cache",
						Aliases: []string{"u"},
					},
					&cli.StringFlag{
						Name:  "features",
						Usage: "Comma-separated features (full-parse, wordcount)",
						Value: "full-parse",
					},
					&cli.StringFlag{
						Name:  "output-dir",
						Usage: "Base directory for cached artifacts",
						Value: artifact_manager.DefaultBaseDir,
					},
					&cli.StringFlag{
						Name:  "max-age",
						Usage: "Maximum age for cached HTML (e.g., '24h'). Use '0s' to require fresh cache.",
						Value: "24h",
					},
					&cli.BoolFlag{
						Name:  "quiet",
						Usage: "Suppress log output (default: true, use --quiet=false for verbose logs)",
						Value: true,
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

func analyzeAction(c *cli.Context) error {
	logLevel := slog.LevelInfo
	if c.Bool("quiet") {
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	// Parse max-age
	maxAge, err := time.ParseDuration(c.String("max-age"))
	if err != nil {
		return fmt.Errorf("invalid max-age duration: %w", err)
	}

	// Initialize artifact manager
	manager, err := artifact_manager.NewManager(c.String("output-dir"), maxAge)
	if err != nil {
		return fmt.Errorf("failed to initialize artifact manager: %w", err)
	}

	// Get URLs
	urlsStr := c.String("urls")
	if urlsStr == "" {
		return fmt.Errorf("no URLs provided via --urls flag")
	}
	urls := strings.Split(urlsStr, ",")

	// Parse features flag
	parseMode := parseFeaturesFlag(c.String("features"))
	logger.Info("Analyzing cached URLs", "count", len(urls), "parse_mode", parseMode)

	// Initialize parser
	p := &parser.Parser{}
	a := &analytics.Analytics{}

	results := make([]Result, 0, len(urls))
	for _, url := range urls {
		url = strings.TrimSpace(url)
		logger.Info("Analyzing URL from cache", "url", url)

		// Load cached HTML
		rawHTML, isFresh, err := manager.GetRawHTML(url)
		if err != nil {
			logger.Error("Failed to load cached HTML", "url", url, "error", err)
			results = append(results, Result{
				URL:       url,
				Error:     fmt.Errorf("cache error: %w", err),
				ErrorType: "cache_error",
			})
			continue
		}
		if !isFresh || rawHTML == nil {
			logger.Error("Cached HTML not found or stale", "url", url)
			results = append(results, Result{
				URL:       url,
				Error:     fmt.Errorf("cache miss: HTML not found or stale"),
				ErrorType: "cache_miss",
			})
			continue
		}

		// Parse with specified mode
		result := Result{URL: url}
		page, parseErr := p.Parse(models.ParseRequest{
			URL:  url,
			HTML: string(rawHTML),
			Mode: parseMode,
		})

		if parseErr != nil {
			logger.Error("Failed to parse HTML", "url", url, "error", parseErr)
			result.Error = parseErr
			result.ErrorType = "parse_error"
			results = append(results, result)
			continue
		}

		result.Page = page

		// Compute metadata if not already done
		if !page.Metadata.Computed {
			page.ComputeMetadata()
		}

		// Extract word counts for analytics
		if parseMode != models.ParseModeMinimal {
			wordCounts := mapreduce.Map(page.ToPlainText(), a)
			result.WordCounts = wordCounts
		}

		// Save parsed result
		jsonData, err := json.Marshal(page)
		if err != nil {
			logger.Warn("Failed to marshal parsed result", "url", url, "error", err)
		} else {
			if saveErr := manager.SetParsedJSON(url, jsonData); saveErr != nil {
				logger.Warn("Failed to save parsed result", "url", url, "error", saveErr)
			} else {
				filePath, _ := manager.GetArtifactPath("parsed", url, ".json")
				result.FilePath = filePath
				// Get file size
				if info, err := os.Stat(filePath); err == nil {
					result.FileSizeBytes = info.Size()
				}
			}
		}

		results = append(results, result)
		logger.Info("Successfully analyzed URL", "url", url, "file_path", result.FilePath)
	}

	// Output results
	finalOutput := &FinalOutput{
		Status: "success",
	}

	summaryResults := make([]ResultSummary, 0, len(results))
	for _, r := range results {
		summary := buildSummary(r)
		summaryResults = append(summaryResults, summary)
	}
	finalOutput.Results = summaryResults

	// Build stats
	stats := Stats{
		TotalURLs: len(urls),
	}
	for _, r := range results {
		if r.Error != nil {
			stats.Failed++
		} else {
			stats.Successful++
		}
	}
	finalOutput.Stats = stats

	// Output JSON
	outputData, err := json.MarshalIndent(finalOutput, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(outputData))
	return nil
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
		data, err := os.ReadFile(filepath.Clean(path))
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

	// Generate session ID from URLs
	sessionID := session.GenerateSessionID(config.URLs)
	outputDir := c.String("output-dir")
	logger.Info("Session ID generated", "session_id", sessionID)

	// Check if session exists and is fresh (unless force-fetch is set)
	if !c.Bool("force-fetch") && session.IsSessionFresh(outputDir, sessionID, maxAge) {
		logger.Info("Session cache hit - returning cached summaries", "session_id", sessionID)
		// Print concise output and return
		sessionDir := session.GetSessionDir(outputDir, sessionID)
		fmt.Printf("Session cache hit! Results from: %s\n", sessionDir)
		return nil
	}

	// Parse features flag to determine ParseMode
	parseMode := parseFeaturesFlag(c.String("features"))
	logger.Info("Parse mode determined", "mode", parseMode, "features", c.String("features"))

	allResults, finalWordCounts, runErr := run(logger, config, manager, c.Bool("force-fetch"), parseMode)

	stats := Stats{
		TotalURLs:        len(config.URLs),
		TotalTimeSeconds: time.Since(startTime).Seconds(),
		TopKeywords:      mapreduce.TopKeywords(finalWordCounts, 25),
	}

	var summaryResults []ResultSummary
	outputMode := strings.ToLower(c.String("output-mode"))
	switch outputMode {
	case "tier2":
		// Two-tier summary system: write to session directory, print concise stats
		// Count success/failed
		var successCount, failedCount int
		for _, r := range allResults {
			if r.Error != nil {
				failedCount++
			} else {
				successCount++
			}
		}

		// Create session directory
		if err := session.EnsureSessionDir(outputDir, sessionID); err != nil {
			return fmt.Errorf("failed to create session directory: %w", err)
		}

		// Generate FIELDS.yaml reference (only if it doesn't exist)
		if err := session.GenerateFieldsReference(outputDir); err != nil {
			logger.Warn("Failed to generate FIELDS.yaml reference", "error", err)
		}

		// Write summaries to session directory
		sessionDir := session.GetSessionDir(outputDir, sessionID)
		if err := writeSummaryIndexToSession(allResults, sessionDir); err != nil {
			return fmt.Errorf("failed to write summary index: %w", err)
		}
		if err := writeSummaryDetailsToSession(allResults, sessionDir); err != nil {
			return fmt.Errorf("failed to write summary details: %w", err)
		}

		// Update sessions index
		sessionInfo := session.Info{
			SessionID:   sessionID,
			Created:     time.Now(),
			URLCount:    len(config.URLs),
			Success:     successCount,
			Failed:      failedCount,
			Features:    session.FormatFeatures(c.String("features")),
			URLsPreview: session.GetURLsPreview(config.URLs, 3),
		}
		if err := session.UpdateSessionIndex(outputDir, sessionInfo); err != nil {
			logger.Warn("Failed to update sessions index", "error", err)
		}

		// Print concise stats to stdout
		fmt.Printf("Parsed %d URLs - %d success, %d failed. Summary files in %s. Features enabled: %s\n",
			len(config.URLs), successCount, failedCount, sessionDir, c.String("features"))

		return nil
	case "summary":
		summaryResults = []ResultSummary{}
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
	summaryVersion := strings.ToLower(c.String("summary-version"))
	summaryFields := c.String("summary-fields")

	// Apply field filtering if requested
	if summaryFields != "" && outputMode == "summary" {
		isTerse := summaryVersion == "v2"

		// Convert results to terse if v2
		var resultsToFilter []interface{}
		if isTerse {
			terseResults := make([]ResultSummaryTerse, len(summaryResults))
			for i, r := range summaryResults {
				terseResults[i] = toTerseResult(r)
			}
			for i := range terseResults {
				resultsToFilter = append(resultsToFilter, terseResults[i])
			}
		} else {
			for i := range summaryResults {
				resultsToFilter = append(resultsToFilter, summaryResults[i])
			}
		}

		// Filter each result
		filteredResults := make([]map[string]interface{}, len(resultsToFilter))
		for i, r := range resultsToFilter {
			filteredResults[i] = filterResultFields(r, summaryFields, isTerse)
		}

		// Build custom output structure
		customOutput := map[string]interface{}{
			"status":  finalOutput.Status,
			"results": filteredResults,
			"stats":   toTerseStats(stats),
		}

		if outputFormat == "yaml" {
			outputData, marshalErr = yaml.Marshal(customOutput)
		} else {
			outputData, marshalErr = json.MarshalIndent(customOutput, "", "  ")
		}
	} else if summaryVersion == "v2" && outputMode == "summary" {
		// Use terse format without field filtering
		terseResults := make([]ResultSummaryTerse, len(summaryResults))
		for i, r := range summaryResults {
			terseResults[i] = toTerseResult(r)
		}

		terseFinalOutput := FinalOutputTerse{
			Status:  finalOutput.Status,
			Results: terseResults,
			Stats:   toTerseStats(stats),
		}

		if outputFormat == "yaml" {
			outputData, marshalErr = yaml.Marshal(terseFinalOutput)
		} else {
			outputData, marshalErr = json.MarshalIndent(terseFinalOutput, "", "  ")
		}
	} else {
		// Use regular format (v1) without field filtering
		if outputFormat == "yaml" {
			outputData, marshalErr = yaml.Marshal(finalOutput)
		} else {
			outputData, marshalErr = json.MarshalIndent(finalOutput, "", "  ")
		}
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

func run(logger *slog.Logger, config *models.Config, manager *artifact_manager.Manager, forceFetch bool, parseMode models.ParseMode) ([]Result, map[string]int, error) {
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

func processHTML(id int, logger *slog.Logger, url string, rawHTML []byte, manager *artifact_manager.Manager, p *parser.Parser, a *analytics.Analytics, results chan<- Result, parseMode models.ParseMode) {
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

		processHTML(id, logger, job.URL, rawHTML, manager, p, a, results, job.ParseMode)
	}
}

// parseFeaturesFlag converts features string to ParseMode
func parseFeaturesFlag(features string) models.ParseMode {
	if features == "" {
		return models.ParseModeMinimal // Default: minimal (metadata only)
	}

	// Parse comma-separated features
	featureList := strings.Split(features, ",")
	for _, f := range featureList {
		f = strings.TrimSpace(strings.ToLower(f))
		switch f {
		case "full-parse":
			return models.ParseModeFull
		case "wordcount":
			// wordcount requires at least cheap parsing
			return models.ParseModeCheap
		}
	}

	// If no recognized features, default to minimal
	return models.ParseModeMinimal
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

// buildSummaryIndex creates minimal index entry (only for successful fetches)
func buildSummaryIndex(r Result) *SummaryIndex {
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
func buildSummaryDetails(r Result) SummaryDetails {
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
	// TODO: Count images from content blocks when in full-parse mode
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
func writeSummaryIndexToSession(results []Result, sessionDir string) error {
	var index []SummaryIndex

	for _, r := range results {
		if entry := buildSummaryIndex(r); entry != nil {
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
func writeSummaryDetailsToSession(results []Result, sessionDir string) error {
	details := make([]SummaryDetails, 0, len(results))

	for _, r := range results {
		details = append(details, buildSummaryDetails(r))
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

