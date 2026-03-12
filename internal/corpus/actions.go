package corpus

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	internaldb "github.com/dtnitsch/llm-web-parser/internal/db"
	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	"github.com/dtnitsch/llm-web-parser/pkg/corpus"
	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// CorpusAction handles corpus API commands.
func CorpusAction(c *cli.Context) error {
	// Parse URL IDs from comma-separated string
	var urlIDs []int64
	if urlIDsStr := c.String("url-ids"); urlIDsStr != "" {
		parts := strings.Split(urlIDsStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid URL ID: %s", part)
			}
			urlIDs = append(urlIDs, id)
		}
	}

	// Determine session ID (for extract verb with no explicit session or url-ids)
	sessionID := c.Int("session")
	isActiveSession := false

	// If no session and no url-ids, default to active session for extract
	if sessionID == 0 && len(urlIDs) == 0 && c.Command.Name == "extract" {
		database, err := dbpkg.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Check for active session first
		activeSessionID := internaldb.GetActiveSession()
		if activeSessionID > 0 {
			// Verify it still exists
			_, err := database.GetSessionByID(activeSessionID)
			if err == nil {
				sessionID = int(activeSessionID)
				isActiveSession = true
			}
		}

		// Fall back to latest session if no active session
		if sessionID == 0 {
			sessions, err := database.ListSessions(1)
			if err != nil {
				return fmt.Errorf("failed to get latest session: %w", err)
			}
			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found. Run 'lwp fetch --urls \"...\"' first")
			}
			sessionID = int(sessions[0].SessionID)
		}
	}

	// Build constraints map for verb-specific parameters
	constraints := make(map[string]interface{})
	// Check --top first, fall back to --limit
	if c.IsSet("top") {
		constraints["top"] = c.Int("top")
	} else if c.IsSet("limit") {
		constraints["top"] = c.Int("limit")
	} else if top := c.Int("top"); top != 0 {
		// Use default value if neither flag was explicitly set
		constraints["top"] = top
	}

	// Build request from CLI flags
	req := models.Request{
		Verb:        c.Command.Name, // extract, query, etc.
		Session:     sessionID,
		View:        c.String("view"),
		Schema:      c.String("schema"),
		Filter:      c.String("filter"),
		Format:      c.String("format"),
		URLIDs:      urlIDs,
		Constraints: constraints,
	}

	// Handle the request
	resp := corpus.Handle(req)

	// Special handling for missing_filter error - print help directly
	if resp.Error != nil && resp.Error.Type == "missing_filter" {
		fmt.Print(resp.Error.Message)
		return nil
	}

	// Check for verbose flag - if set, output full YAML
	if c.Bool("verbose") {
		yamlBytes, err := yaml.Marshal(resp)
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Print(string(yamlBytes))
		return nil
	}

	// Compact output for extract verb
	if req.Verb == "extract" {
		return outputExtractCompact(&resp, sessionID, isActiveSession, c.Int("top"))
	}

	// Default: output full YAML for other verbs
	yamlBytes, err := yaml.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Print(string(yamlBytes))
	return nil
}

// SuggestAction handles corpus suggest commands.
func SuggestAction(c *cli.Context) error {
	sessionID := int64(c.Int("session"))
	if sessionID == 0 {
		return fmt.Errorf("session ID is required")
	}

	// Generate suggestions
	suggestions, err := corpus.SuggestFromSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to generate suggestions: %w", err)
	}

	fmt.Print(suggestions)
	return nil
}

// outputExtractCompact outputs extract results in compact text format
func outputExtractCompact(resp *models.Response, sessionID int, isActiveSession bool, topLimit int) error {
	// Marshal to YAML first, then unmarshal to map for easier access
	yamlBytes, err := yaml.Marshal(resp.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Get keywords
	keywordsRaw, ok := data["keywords"]
	if !ok {
		fmt.Println("No keywords found")
		return nil
	}

	keywords, ok := keywordsRaw.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected keywords format")
	}

	if len(keywords) == 0 {
		fmt.Println("No keywords found")
		return nil
	}

	// Show session header if we have a session
	if sessionID > 0 {
		if isActiveSession {
			fmt.Printf("Session: %d (active)\n", sessionID)
		} else {
			fmt.Printf("Session: %d\n", sessionID)
		}
	}

	// Print subheader
	fmt.Printf("Top keywords:\n\n")

	// Print keywords in compact format
	displayLimit := len(keywords)
	if topLimit > 0 && topLimit < displayLimit {
		displayLimit = topLimit
	}

	for i := 0; i < displayLimit && i < len(keywords); i++ {
		kw, ok := keywords[i].(map[string]interface{})
		if !ok {
			continue
		}

		word, _ := kw["word"].(string)
		count := 0
		if c, ok := kw["count"].(int); ok {
			count = c
		} else if c, ok := kw["count"].(float64); ok {
			count = int(c)
		}

		fmt.Printf("%s (%d)\n", word, count)
	}

	// Show tips
	fmt.Println()

	// If we're showing a limited set, suggest --top=0 for full list
	if topLimit > 0 {
		fmt.Printf("Tip: Use --top=0 for full list | --verbose for full details (confidence, coverage, hints)\n")
	} else {
		fmt.Printf("Tip: Use --verbose for full details (confidence, coverage, hints)\n")
	}

	return nil
}

// GrepAction handles corpus grep command - search across multiple URLs
func GrepAction(c *cli.Context) error {
	if c.NArg() == 0 {
		fmt.Println("Error: pattern required")
		fmt.Println()
		cli.ShowSubcommandHelp(c)
		return nil
	}

	pattern := c.Args().First()

	// Open database
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Get session ID (custom logic since pattern is positional arg, not session)
	var sessionID int64
	if c.IsSet("session") {
		sessionID = int64(c.Int("session"))
		if sessionID <= 0 {
			return fmt.Errorf("invalid session ID: %d (must be > 0)", sessionID)
		}
	} else {
		// Check for active session first
		activeSessionID := internaldb.GetActiveSession()
		if activeSessionID > 0 {
			// Verify it still exists
			_, err := database.GetSessionByID(activeSessionID)
			if err == nil {
				sessionID = activeSessionID
			}
		}

		// Fall back to latest session if no active session
		if sessionID == 0 {
			sessions, err := database.ListSessions(1)
			if err != nil {
				return fmt.Errorf("failed to get latest session: %w", err)
			}
			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found. Run 'lwp fetch --urls \"...\"' first")
			}
			sessionID = sessions[0].SessionID
		}
	}

	// Check if session is active
	activeSessionID := internaldb.GetActiveSession()
	isActive := (sessionID == activeSessionID)

	// Get URLs to search
	var urlIDs []int64
	if urlsArg := c.String("urls"); urlsArg != "" {
		// Parse comma-separated URL IDs or URLs
		parts := strings.Split(urlsArg, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			// Try parsing as ID first
			if id, err := strconv.ParseInt(part, 10, 64); err == nil {
				urlIDs = append(urlIDs, id)
			} else {
				// Try resolving as URL
				id, err := database.GetURLID(part)
				if err != nil {
					return fmt.Errorf("URL not found: %s", part)
				}
				urlIDs = append(urlIDs, id)
			}
		}
	} else {
		// Get all URLs from session
		urls, err := database.GetSessionURLsWithMetadata(sessionID)
		if err != nil {
			return fmt.Errorf("failed to get session URLs: %w", err)
		}
		for _, u := range urls {
			urlIDs = append(urlIDs, u.URLID)
		}
	}

	if len(urlIDs) == 0 {
		fmt.Printf("No URLs found in session %d\n", sessionID)
		return nil
	}

	// Initialize artifact manager
	manager, err := artifact_manager.NewManager(artifact_manager.DefaultBaseDir, 0)
	if err != nil {
		return fmt.Errorf("failed to initialize artifact manager: %w", err)
	}

	// Check if pattern contains | (grouped grep)
	isGrouped := strings.Contains(pattern, "|")
	var subPatterns []string
	if isGrouped {
		subPatterns = strings.Split(pattern, "|")
		for i, p := range subPatterns {
			subPatterns[i] = strings.TrimSpace(p)
		}
	} else {
		subPatterns = []string{pattern}
	}

	// Compile regexes
	regexes := make([]*regexp.Regexp, len(subPatterns))
	for i, p := range subPatterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return fmt.Errorf("invalid pattern '%s': %w", p, err)
		}
		regexes[i] = re
	}

	// Collect results
	results := []URLResult{}
	totalMatches := 0

	// Search each URL
	for _, urlID := range urlIDs {
		data, found, err := manager.GetParsedJSONByID(urlID)
		if err != nil {
			fmt.Fprintf(c.App.ErrWriter, "Warning: failed to read URL %d: %v\n", urlID, err)
			continue
		}
		if !found {
			continue
		}

		// Parse YAML to get page
		var page models.Page
		if err := yaml.Unmarshal(data, &page); err != nil {
			fmt.Fprintf(c.App.ErrWriter, "Warning: failed to parse URL %d: %v\n", urlID, err)
			continue
		}

		// Count matches per pattern
		matchesByPattern := make(map[string]int)
		urlTotal := 0

		for i, re := range regexes {
			count := countMatches(&page, re)
			matchesByPattern[subPatterns[i]] = count
			urlTotal += count
		}

		if urlTotal > 0 {
			result := URLResult{
				URLID:        urlID,
				URL:          page.URL,
				TotalMatches: urlTotal,
			}
			if isGrouped {
				result.MatchesByPattern = matchesByPattern
			}
			results = append(results, result)
			totalMatches += urlTotal
		}
	}

	// Sort results by total matches (descending)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].TotalMatches > results[i].TotalMatches {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Output based on format
	outputFormat := strings.ToLower(c.String("format"))

	if outputFormat == "json" {
		return outputJSON(sessionID, isActive, pattern, subPatterns, results, totalMatches, isGrouped)
	} else if outputFormat == "yaml" {
		return outputYAML(sessionID, isActive, pattern, subPatterns, results, totalMatches, isGrouped)
	} else if outputFormat == "csv" {
		return outputCSV(c.App.Writer, subPatterns, results, isGrouped)
	} else {
		// Default text format
		return outputText(sessionID, isActive, pattern, subPatterns, results, totalMatches, isGrouped)
	}
}

// countMatches counts pattern matches in a page
func countMatches(page *models.Page, re *regexp.Regexp) int {
	count := 0

	if len(page.FlatContent) > 0 {
		for _, block := range page.FlatContent {
			if re.MatchString(block.Text) {
				count++
			}
		}
	} else {
		var countInSection func(sections []models.Section)
		countInSection = func(sections []models.Section) {
			for _, section := range sections {
				for _, block := range section.Blocks {
					if re.MatchString(block.Text) {
						count++
					}
				}
				countInSection(section.Children)
			}
		}
		countInSection(page.Content)
	}

	return count
}

// URLResult holds grep results for a single URL
type URLResult struct {
	URLID           int64          `json:"url_id" yaml:"url_id"`
	URL             string         `json:"url" yaml:"url"`
	MatchesByPattern map[string]int `json:"matches_by_pattern,omitempty" yaml:"matches_by_pattern,omitempty"`
	TotalMatches    int            `json:"total_matches" yaml:"total_matches"`
}

// outputText outputs results in human-readable text format
func outputText(sessionID int64, isActive bool, pattern string, subPatterns []string, urlResults []URLResult, totalMatches int, isGrouped bool) error {

	// Header
	if isActive {
		fmt.Printf("Session: %d (active)\n", sessionID)
	} else {
		fmt.Printf("Session: %d\n", sessionID)
	}

	if isGrouped {
		fmt.Printf("Patterns: %s\n\n", strings.Join(subPatterns, ", "))
	} else {
		fmt.Printf("Pattern: %q\n\n", pattern)
	}

	// Results
	if len(urlResults) == 0 {
		fmt.Println("No matches found")
		return nil
	}

	for _, result := range urlResults {
		if isGrouped {
			// Show breakdown by pattern
			parts := make([]string, len(subPatterns))
			for i, p := range subPatterns {
				count := result.MatchesByPattern[p]
				parts[i] = fmt.Sprintf("%d %s", count, p)
			}
			fmt.Printf("#%-3d  %s  %s\n", result.URLID, strings.Join(parts, ", "), result.URL)
		} else {
			// Simple count
			matchWord := "matches"
			if result.TotalMatches == 1 {
				matchWord = "match"
			}
			fmt.Printf("#%-3d  %d %s  %s\n", result.URLID, result.TotalMatches, matchWord, result.URL)
		}
	}

	// Summary
	fmt.Println()
	if isGrouped {
		fmt.Printf("Total: %d matches across %d patterns, %d URLs\n", totalMatches, len(subPatterns), len(urlResults))
	} else {
		fmt.Printf("Total: %d matches across %d URLs\n", totalMatches, len(urlResults))
	}

	return nil
}

// outputJSON outputs results in JSON format
func outputJSON(sessionID int64, isActive bool, pattern string, subPatterns []string, results []URLResult, totalMatches int, isGrouped bool) error {
	output := map[string]interface{}{
		"session_id":    sessionID,
		"active":        isActive,
		"total_matches": totalMatches,
		"total_urls":    len(results),
		"urls":          results,
	}

	if isGrouped {
		output["patterns"] = subPatterns
	} else {
		output["pattern"] = pattern
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// outputYAML outputs results in YAML format
func outputYAML(sessionID int64, isActive bool, pattern string, subPatterns []string, results []URLResult, totalMatches int, isGrouped bool) error {
	output := map[string]interface{}{
		"session_id":    sessionID,
		"active":        isActive,
		"total_matches": totalMatches,
		"total_urls":    len(results),
		"urls":          results,
	}

	if isGrouped {
		output["patterns"] = subPatterns
	} else {
		output["pattern"] = pattern
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

// outputCSV outputs results in CSV format
func outputCSV(writer io.Writer, subPatterns []string, results []URLResult, isGrouped bool) error {
	w := csv.NewWriter(writer)
	defer w.Flush()

	if isGrouped {
		// Header: url_id, url, pattern1, pattern2, ..., total
		header := []string{"url_id", "url"}
		header = append(header, subPatterns...)
		header = append(header, "total")
		w.Write(header)

		// Rows
		for _, result := range results {
			row := []string{
				fmt.Sprintf("%d", result.URLID),
				result.URL,
			}
			for _, p := range subPatterns {
				row = append(row, fmt.Sprintf("%d", result.MatchesByPattern[p]))
			}
			row = append(row, fmt.Sprintf("%d", result.TotalMatches))
			w.Write(row)
		}
	} else {
		// Header: url_id, url, matches
		w.Write([]string{"url_id", "url", "matches"})

		// Rows
		for _, result := range results {
			w.Write([]string{
				fmt.Sprintf("%d", result.URLID),
				result.URL,
				fmt.Sprintf("%d", result.TotalMatches),
			})
		}
	}

	return nil
}
