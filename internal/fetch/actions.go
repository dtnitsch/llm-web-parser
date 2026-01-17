package fetch

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/dtnitsch/llm-web-parser/internal/common"
	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	"github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/dtnitsch/llm-web-parser/pkg/extractor"
	"github.com/dtnitsch/llm-web-parser/pkg/mapreduce"
	"github.com/dtnitsch/llm-web-parser/pkg/session"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func FetchAction(c *cli.Context) error {
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

	// Open database for metadata storage
	database, err := db.Open()
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(2)
	}
	defer database.Close()

	// Initialize runtime config from CLI flags
	config := &models.FetchConfig{
		URLs:        []string{},
		WorkerCount: c.Int("workers"),
	}

	// Load URLs from session if --session is provided
	if c.IsSet("session") {
		if c.IsSet("urls") {
			fmt.Fprintln(os.Stderr, "Error: Cannot use both --urls and --session flags")
			fmt.Fprintln(os.Stderr, "Use --session to refetch URLs from a previous session, or --urls for new URLs")
			os.Exit(1)
		}

		sessionID := int64(c.Int("session"))
		failedOnly := c.Bool("failed-only")

		if failedOnly {
			// Get only failed URLs from session
			results, err := database.GetSessionResults(sessionID)
			if err != nil {
				logger.Error("failed to get session results", "error", err, "session_id", sessionID)
				os.Exit(2)
			}

			var failedURLs []string
			for _, r := range results {
				if r.Status == "failed" {
					failedURLs = append(failedURLs, r.URL)
				}
			}

			if len(failedURLs) == 0 {
				fmt.Printf("Session %d has no failed URLs to retry\n", sessionID)
				os.Exit(0)
			}

			config.URLs = failedURLs
			fmt.Fprintf(os.Stderr, "Retrying %d failed URLs from session %d\n", len(failedURLs), sessionID)
		} else {
			// Get all URLs from session
			urls, err := database.GetSessionURLsWithSanitization(sessionID)
			if err != nil {
				logger.Error("failed to get session URLs", "error", err, "session_id", sessionID)
				os.Exit(2)
			}

			if len(urls) == 0 {
				fmt.Printf("Session %d has no URLs\n", sessionID)
				os.Exit(0)
			}

			config.URLs = make([]string, len(urls))
			for i, u := range urls {
				config.URLs[i] = u.URL
			}

			fmt.Fprintf(os.Stderr, "Refetching %d URLs from session %d\n", len(config.URLs), sessionID)
		}
	}

	if c.IsSet("urls") {
		config.URLs = strings.Split(c.String("urls"), ",")
	}
	// WorkerCount is already set during config initialization from CLI flag

	if len(config.URLs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No URLs provided")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, `  llm-web-parser fetch --urls "https://example.com,https://example.org"`)
		fmt.Fprintln(os.Stderr, `  llm-web-parser fetch --session 5                         # Refetch all URLs from session 5`)
		fmt.Fprintln(os.Stderr, `  llm-web-parser fetch --session 5 --failed-only          # Retry only failed URLs`)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Need help? Run: llm-web-parser fetch --help")
		os.Exit(1)
	}

	// Keep original URLs for tracking sanitization
	originalURLs := make([]string, len(config.URLs))
	copy(originalURLs, config.URLs)

	// Sanitize and validate all URLs before processing (fail fast)
	sanitizedURLs, invalidURLs := common.SanitizeAndValidateURLs(config.URLs)
	if len(invalidURLs) > 0 {
		fmt.Fprintf(os.Stderr, "Error: %d URL(s) are malformed (even after cleanup):\n", len(invalidURLs))
		for _, badURL := range invalidURLs {
			fmt.Fprintf(os.Stderr, "  - %s\n", badURL)
		}
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Note: URLs are auto-cleaned (whitespace trimmed, trailing punctuation removed, markdown links extracted)")
		fmt.Fprintln(os.Stderr, "      Spaces in URLs must be pre-encoded as %20. Braces {} in domains are not allowed.")
		os.Exit(1)
	}

	// Replace with sanitized URLs
	config.URLs = sanitizedURLs

	// Parse features flag to determine ParseMode (needed for session lookup)
	parseMode := ParseFeaturesFlag(c.String("features"))
	parseModeStr := string(parseMode)
	logger.Info("Parse mode determined", "mode", parseMode, "features", c.String("features"))

	// Find or create session in database
	outputDir := c.String("output-dir")
	sessionMaxAge := maxAge
	if c.Bool("force-fetch") {
		sessionMaxAge = 0 // Force new session
	}
	sessionID, cacheHit, err := database.FindOrCreateSession(originalURLs, config.URLs, c.String("features"), parseModeStr, sessionMaxAge)
	if err != nil {
		logger.Error("failed to find or create session", "error", err)
		os.Exit(2)
	}
	logger.Info("Session", "session_id", sessionID, "cache_hit", cacheHit)

	// If cache hit, return early
	if cacheHit {
		logger.Info("Session cache hit - returning cached summaries", "session_id", sessionID)
		sessionTimestamp := time.Now() // For display purposes
		sessionDir := session.GetSessionDir(sessionID, sessionTimestamp)
		fmt.Printf("Session %d cache hit! Results at: %s\n", sessionID, sessionDir)
		return nil
	}

	// Parse filter flag if provided
	var filterStrategy *extractor.Strategy
	filterStr := c.String("filter")
	if filterStr != "" {
		filterStrategy, err = extractor.ParseStrategy(filterStr)
		if err != nil {
			logger.Error("invalid filter strategy", "error", err)
			os.Exit(2)
		}
		logger.Info("Filter strategy parsed", "filter", filterStr)
	}

	allResults, finalWordCounts, runErr := run(logger, config, manager, c.Bool("force-fetch"), parseMode, filterStrategy, database)

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
		sessionTimestamp := time.Now()
		if err := session.EnsureSessionDir(sessionID, sessionTimestamp); err != nil {
			return fmt.Errorf("failed to create session directory: %w", err)
		}

		// Generate FIELDS.yaml reference (only if it doesn't exist)
		if err := session.GenerateFieldsReference(outputDir); err != nil {
			logger.Warn("Failed to generate FIELDS.yaml reference", "error", err)
		}

		// Write summaries to session directory
		sessionDir := session.GetSessionDir(sessionID, sessionTimestamp)
		if err := WriteSummaryIndexToSession(allResults, sessionDir); err != nil {
			return fmt.Errorf("failed to write summary index: %w", err)
		}
		if err := WriteSummaryDetailsToSession(allResults, sessionDir, database); err != nil {
			return fmt.Errorf("failed to write summary details: %w", err)
		}

		// Collect and write failed URLs if any
		failedURLs := collectFailedURLs(allResults)
		if err := WriteFailedURLsToSession(failedURLs, sessionDir); err != nil {
			logger.Warn("Failed to write failed URLs file", "error", err)
		}

		// Update session stats in database
		if err := database.UpdateSessionStats(sessionID, successCount, failedCount); err != nil {
			logger.Warn("Failed to update session stats in DB", "error", err)
		}

		// Insert session results for each URL
		for _, result := range allResults {
			urlID, getErr := database.GetURLID(result.URL)
			if getErr != nil {
				logger.Warn("Failed to get URL ID for session result", "url", result.URL, "error", getErr)
				continue
			}

			status := "success"
			statusCode := 200
			errorType := ""
			errorMessage := ""
			if result.Error != nil {
				status = "failed"
				statusCode = 0
				errorType = result.ErrorType
				errorMessage = result.Error.Error()
			}

			estimatedTokens := 0
			if result.Page != nil && result.Page.Metadata.WordCount > 0 {
				estimatedTokens = result.Page.Metadata.WordCount / 2 // Rough estimate
			}

			if err := database.InsertSessionResult(sessionID, urlID, status, statusCode, errorType, errorMessage, result.FileSizeBytes, estimatedTokens); err != nil {
				logger.Warn("Failed to insert session result", "url", result.URL, "error", err)
			}
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
		if err := session.UpdateSessionIndex(sessionInfo); err != nil {
			logger.Warn("Failed to update sessions index", "error", err)
		}

		// Print simplified stats to stdout
		fmt.Printf("Session %d: %d/%d URLs successful\nResults: %s\n", sessionID, successCount, len(config.URLs), sessionDir)

		// Show quick start commands for corpus API
		if successCount > 0 {
			fmt.Printf("\n💡 Quick start:\n")
			fmt.Printf("  lwp corpus extract --session=%d        # See top keywords\n", sessionID)
			fmt.Printf("  lwp corpus query --session=%d --filter=\"has_code\"  # Filter results\n", sessionID)
			fmt.Printf("\nMore: lwp corpus suggest --session=%d\n", sessionID)
		}

		// Show URL IDs unless --quiet flag is set
		if !c.Bool("quiet") {
			urlsWithSanitization, err := database.GetSessionURLsWithSanitization(sessionID)
			if err == nil && len(urlsWithSanitization) > 0 {
				fmt.Printf("\nURL IDs:\n")
				for i, u := range urlsWithSanitization {
					fmt.Printf("  %d. [#%d] %s\n", i+1, u.URLID, u.URL)
				}
			}
		}

		// Show sanitization info if any URLs were cleaned
		sanitizedCount, err := database.CountSanitizedURLs(sessionID)
		if err == nil && sanitizedCount > 0 {
			fmt.Printf("\nNote: %d URL(s) were auto-cleaned\n", sanitizedCount)
			fmt.Printf("  To see what changed: lwp db urls %d --sanitized\n", sessionID)
		}

		// Show tip about using URL IDs and common commands
		fmt.Printf("\nCommands:\n")
		fmt.Printf("  lwp db get --file=details %d  # Full session YAML\n", sessionID)
		fmt.Printf("  lwp db urls %d                # List URL IDs\n", sessionID)
		fmt.Printf("  lwp db show <id>              # Get parsed content\n")
		fmt.Printf("  lwp db raw <id>               # Get raw HTML\n")

		return nil
	case "summary":
		summaryResults = []ResultSummary{}
		for _, r := range allResults {
			summary := BuildSummary(r)
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
				terseResults[i] = ToTerseResult(r)
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
			filteredResults[i] = common.FilterResultFields(r, summaryFields, isTerse)
		}

		// Build custom output structure
		customOutput := map[string]interface{}{
			"status":  finalOutput.Status,
			"results": filteredResults,
			"stats":   ToTerseStats(stats),
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
			terseResults[i] = ToTerseResult(r)
		}

		terseFinalOutput := FinalOutputTerse{
			Status:  finalOutput.Status,
			Results: terseResults,
			Stats:   ToTerseStats(stats),
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
