package analyze

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dtnitsch/llm-web-parser/internal/fetch"
	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/analytics"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	"github.com/dtnitsch/llm-web-parser/pkg/extractor"
	"github.com/dtnitsch/llm-web-parser/pkg/mapreduce"
	"github.com/dtnitsch/llm-web-parser/pkg/parser"
	"github.com/urfave/cli/v2"
)

func AnalyzeAction(c *cli.Context) error {
	fmt.Fprintln(os.Stderr, "WARNING: 'analyze' is deprecated. Use 'fetch' instead (auto-detects cache).")
	fmt.Fprintln(os.Stderr, "Example: llm-web-parser fetch --urls \"...\" --features full-parse")

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
	parseMode := fetch.ParseFeaturesFlag(c.String("features"))
	logger.Info("Analyzing cached URLs", "count", len(urls), "parse_mode", parseMode)

	// Initialize parser
	p := &parser.Parser{}
	a := &analytics.Analytics{}

	results := make([]fetch.Result, 0, len(urls))
	for _, url := range urls {
		url = strings.TrimSpace(url)
		logger.Info("Analyzing URL from cache", "url", url)

		// Load cached HTML
		rawHTML, isFresh, err := manager.GetRawHTML(url)
		if err != nil {
			logger.Error("Failed to load cached HTML", "url", url, "error", err)
			results = append(results, fetch.Result{
				URL:       url,
				Error:     fmt.Errorf("cache error: %w", err),
				ErrorType: "cache_error",
			})
			continue
		}
		if !isFresh || rawHTML == nil {
			logger.Error("Cached HTML not found or stale", "url", url)
			results = append(results, fetch.Result{
				URL:       url,
				Error:     fmt.Errorf("cache miss: HTML not found or stale"),
				ErrorType: "cache_miss",
			})
			continue
		}

		// Parse with specified mode
		result := fetch.Result{URL: url}
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
	finalOutput := &fetch.FinalOutput{
		Status: "success",
	}

	summaryResults := make([]fetch.ResultSummary, 0, len(results))
	for _, r := range results {
		summary := fetch.BuildSummary(r)
		summaryResults = append(summaryResults, summary)
	}
	finalOutput.Results = summaryResults

	// Build stats
	stats := fetch.Stats{
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

func ExtractAction(c *cli.Context) error {
	fmt.Fprintln(os.Stderr, "WARNING: 'extract' is deprecated. Use 'fetch --filter' instead.")
	fmt.Fprintln(os.Stderr, "Example: llm-web-parser fetch --urls \"...\" --filter \"conf:>=0.7\"")

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
