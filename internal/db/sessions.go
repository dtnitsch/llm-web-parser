package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func SessionsAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	limit := c.Int("limit")
	verbose := c.Bool("verbose")

	sessions, err := database.ListSessions(limit)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	// Get active session
	activeSessionID := getActiveSession()

	if verbose {
		// Verbose mode: show aggregated metadata
		fmt.Printf("%-4s %-12s %-6s %-8s %-45s %-25s %-8s\n",
			"ID", "Date", "URLs", "Status", "Keywords", "Types", "Code")
		fmt.Println(strings.Repeat("-", 120))

		for _, s := range sessions {
			// Get aggregated metadata for this session
			meta := getSessionAggregatedMetadata(database, s.SessionID)

			// Format status
			status := fmt.Sprintf("%d/%d", s.SuccessCount, s.FailedCount)

			// Mark active session
			idStr := fmt.Sprintf("%d", s.SessionID)
			if s.SessionID == activeSessionID {
				idStr = fmt.Sprintf("%d*", s.SessionID)
			}

			fmt.Printf("%-4s %-12s %-6d %-8s %-45s %-25s %-8s\n",
				idStr,
				s.CreatedAt.Format("2006-01-02"),
				s.URLCount,
				status,
				meta.Keywords,
				meta.Types,
				meta.CodePercent,
			)
		}

		fmt.Printf("\nTotal: %d sessions", len(sessions))
		if activeSessionID > 0 {
			fmt.Printf(" (* = active session: %d)", activeSessionID)
		}
		fmt.Println()
	} else {
		// Compact mode: original format
		fmt.Printf("%-4s %-12s %-6s %-8s %-10s\n",
			"ID", "Date", "URLs", "Status", "Parse")
		fmt.Println(strings.Repeat("-", 50))

		for _, s := range sessions {
			status := fmt.Sprintf("%d/%d", s.SuccessCount, s.FailedCount)

			// Mark active session
			idStr := fmt.Sprintf("%d", s.SessionID)
			if s.SessionID == activeSessionID {
				idStr = fmt.Sprintf("%d*", s.SessionID)
			}

			fmt.Printf("%-4s %-12s %-6d %-8s %-10s\n",
				idStr,
				s.CreatedAt.Format("2006-01-02"),
				s.URLCount,
				status,
				s.ParseMode,
			)
		}

		fmt.Printf("\nTotal: %d sessions", len(sessions))
		if activeSessionID > 0 {
			fmt.Printf(" (* = active: %d)", activeSessionID)
		}
		fmt.Println()
		fmt.Printf("\nTip: Use --verbose to see keywords and content types\n")
	}

	return nil
}

// sessionAction shows details for a specific session
func SessionAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	sessionID, err := GetSessionIDOrLatest(c, database)
	if err != nil {
		return err
	}

	// Get session info
	session, err := database.GetSessionByID(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Get URLs for this session
	urls, err := database.GetSessionURLs(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session URLs: %w", err)
	}

	// Get results for this session
	results, err := database.GetSessionResults(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session results: %w", err)
	}

	// Print session details
	fmt.Printf("Session %d\n", session.SessionID)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Created:     %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Directory:   %s\n", session.SessionDir)
	fmt.Printf("URLs:        %d total (%d success, %d failed)\n",
		session.URLCount, session.SuccessCount, session.FailedCount)
	fmt.Printf("Features:    %s\n", session.Features)
	fmt.Printf("Parse Mode:  %s\n", session.ParseMode)

	// Print URLs
	fmt.Printf("\nURLs (%d):\n", len(urls))
	fmt.Println(strings.Repeat("-", 60))
	for i, u := range urls {
		fmt.Printf("%2d. %s\n", i+1, u.OriginalURL)
		canonicalURL := u.CanonicalURL.String
		if !u.CanonicalURL.Valid {
			canonicalURL = "(none)"
		}
		fmt.Printf("    Domain: %s, Canonical: %s\n", u.Domain, canonicalURL)
	}

	// Print results if available
	if len(results) > 0 {
		fmt.Printf("\nResults (%d):\n", len(results))
		fmt.Println(strings.Repeat("-", 60))
		for i, r := range results {
			fmt.Printf("%2d. [%s] %s\n", i+1, r.Status, r.URL)
			if r.Status == "failed" {
				fmt.Printf("    Error: [%s] %s\n", r.ErrorType, r.ErrorMessage)
			} else {
				fmt.Printf("    Status: %d | Size: %d bytes | Tokens: ~%d\n",
					r.StatusCode, r.FileSizeBytes, r.EstimatedTokens)
			}
		}
	}

	fmt.Printf("\nTip: Use 'llm-web-parser db get %d' to see summary YAML\n", sessionID)

	return nil
}

// getSessionAction retrieves and prints session content files
func GetSessionAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	sessionID, err := GetSessionIDOrLatest(c, database)
	if err != nil {
		return err
	}

	// Get session to find directory
	session, err := database.GetSessionByID(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Determine which file to read
	fileType := strings.ToLower(c.String("file"))
	var fileName string
	switch fileType {
	case "index":
		fileName = "summary-index.yaml"
	case "details":
		fileName = "summary-details.yaml"
	case "failed":
		fileName = "failed-urls.yaml"
	default:
		return fmt.Errorf("unknown file type: %s (use: index, details, or failed)", fileType)
	}

	// Build full path (session_dir is relative to output dir)
	outputDir := artifact_manager.DefaultBaseDir
	filePath := filepath.Join(outputDir, session.SessionDir, fileName)

	// Read and print file
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s\nSession directory: %s", fileName, session.SessionDir)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Print session reminder as YAML comment
	fmt.Printf("# Session: %d\n", sessionID)
	fmt.Print(string(data))

	return nil
}

// querySessionsAction queries sessions with filters
func QuerySessionsAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	todayOnly := c.Bool("today")
	failedOnly := c.Bool("failed")
	urlPattern := c.String("url")

	sessions, err := database.QuerySessions(todayOnly, failedOnly, urlPattern)
	if err != nil {
		return fmt.Errorf("failed to query sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found matching filters")
		if todayOnly {
			fmt.Println("  - Filter: today only")
		}
		if failedOnly {
			fmt.Println("  - Filter: with failures")
		}
		if urlPattern != "" {
			fmt.Printf("  - Filter: URL pattern '%s'\n", urlPattern)
		}
		return nil
	}

	// Print table header
	fmt.Printf("%-6s %-20s %-8s %-8s %-8s %-15s %-30s\n",
		"ID", "Created", "URLs", "Success", "Failed", "Parse Mode", "Session Dir")
	fmt.Println(strings.Repeat("-", 120))

	// Print each session
	for _, s := range sessions {
		fmt.Printf("%-6d %-20s %-8d %-8d %-8d %-15s %-30s\n",
			s.SessionID,
			s.CreatedAt.Format("2006-01-02 15:04:05"),
			s.URLCount,
			s.SuccessCount,
			s.FailedCount,
			s.ParseMode,
			s.SessionDir,
		)
	}

	fmt.Printf("\nFound: %d sessions\n", len(sessions))

	return nil
}

// SessionAggregatedMetadata holds aggregated metadata for a session
type SessionAggregatedMetadata struct {
	Keywords    string // Top 5 keywords across all URLs
	Types       string // Top 2-3 content types with counts
	CodePercent string // Percentage with code examples
}

// getSessionAggregatedMetadata aggregates metadata across all URLs in a session
func getSessionAggregatedMetadata(database *dbpkg.DB, sessionID int64) SessionAggregatedMetadata {
	meta := SessionAggregatedMetadata{
		Keywords:    "",
		Types:       "",
		CodePercent: "0%",
	}

	// Get all URLs with metadata for this session
	urls, err := database.GetSessionURLsWithMetadata(sessionID)
	if err != nil || len(urls) == 0 {
		return meta
	}

	// Aggregate keywords (combine meta and top keywords from all URLs)
	keywordCounts := make(map[string]int)
	for _, u := range urls {
		// Add meta keywords (weight: 2x since they're author-supplied)
		for _, kw := range u.MetaKeywords {
			keywordCounts[kw] += 2
		}
		// Add top keywords (weight: 1x)
		for _, kw := range u.TopKeywords {
			keywordCounts[kw]++
		}
	}

	// Get top 5 keywords by count
	type kwCount struct {
		word  string
		count int
	}
	var sortedKW []kwCount
	for word, count := range keywordCounts {
		sortedKW = append(sortedKW, kwCount{word, count})
	}
	sort.Slice(sortedKW, func(i, j int) bool {
		return sortedKW[i].count > sortedKW[j].count
	})

	keywords := []string{}
	for i := 0; i < len(sortedKW) && i < 5; i++ {
		keywords = append(keywords, sortedKW[i].word)
	}
	meta.Keywords = strings.Join(keywords, ", ")

	// Aggregate content types
	typeCounts := make(map[string]int)
	for _, u := range urls {
		if u.ContentType != "" && u.ContentType != "unknown" {
			typeCounts[u.ContentType]++
		}
	}

	// Sort types by count DESC
	type typeCount struct {
		name  string
		count int
	}
	var sortedTypes []typeCount
	for name, count := range typeCounts {
		sortedTypes = append(sortedTypes, typeCount{name, count})
	}
	sort.Slice(sortedTypes, func(i, j int) bool {
		return sortedTypes[i].count > sortedTypes[j].count
	})

	// Format top 3 types
	typeStrs := []string{}
	for i := 0; i < len(sortedTypes) && i < 3; i++ {
		if sortedTypes[i].count > 1 {
			typeStrs = append(typeStrs, fmt.Sprintf("%s (%d)", sortedTypes[i].name, sortedTypes[i].count))
		} else {
			typeStrs = append(typeStrs, sortedTypes[i].name)
		}
	}
	meta.Types = strings.Join(typeStrs, ", ")

	// Calculate code percentage
	codeCount := 0
	for _, u := range urls {
		if u.HasCodeExamples {
			codeCount++
		}
	}
	if len(urls) > 0 {
		percent := (codeCount * 100) / len(urls)
		meta.CodePercent = fmt.Sprintf("%d%%", percent)
	}

	return meta
}

// Config represents the .lwp/config file
type Config struct {
	ActiveSession int64 `yaml:"active_session"`
}

// getActiveSession returns the active session ID from .lwp/config
func getActiveSession() int64 {
	configPath := ".lwp/config"
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0 // No active session
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return 0
	}

	return config.ActiveSession
}

// setActiveSession writes the active session ID to .lwp/config
func setActiveSession(sessionID int64) error {
	// Ensure .lwp directory exists
	if err := os.MkdirAll(".lwp", 0755); err != nil {
		return fmt.Errorf("failed to create .lwp directory: %w", err)
	}

	config := Config{
		ActiveSession: sessionID,
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := ".lwp/config"
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// SetActiveSession is exported for use by fetch command
func SetActiveSession(sessionID int64) error {
	return setActiveSession(sessionID)
}

// GetActiveSession is exported for use by corpus command
func GetActiveSession() int64 {
	return getActiveSession()
}

// UseAction sets the active session
func UseAction(c *cli.Context) error {
	// Check for --clear flag first (before checking args)
	if c.Bool("clear") {
		if err := setActiveSession(0); err != nil {
			return err
		}
		fmt.Println("Active session cleared")
		return nil
	}

	if c.NArg() == 0 {
		// Show current active session
		activeSessionID := getActiveSession()
		if activeSessionID == 0 {
			fmt.Println("No active session set")
			fmt.Println("\nTips:")
			fmt.Println("  llm-web-parser db sessions                # List all sessions")
			fmt.Println("  llm-web-parser db sessions --verbose      # See keywords and content types")
			fmt.Println("  llm-web-parser db use latest              # Set latest session as active")
			fmt.Println("  llm-web-parser db use 12                  # Set session 12 as active")
			return nil
		}

		database, err := dbpkg.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		session, err := database.GetSessionByID(activeSessionID)
		if err != nil {
			fmt.Printf("Active session %d not found (may have been deleted)\n", activeSessionID)
			fmt.Println("\nTip: Clear with: llm-web-parser db use --clear")
			return nil
		}

		fmt.Printf("Active session: %d\n", activeSessionID)
		fmt.Printf("Created: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("URLs: %d (%d success, %d failed)\n",
			session.URLCount, session.SuccessCount, session.FailedCount)

		fmt.Println("\nTips:")
		fmt.Println("  llm-web-parser db sessions                # List all sessions")
		fmt.Println("  llm-web-parser db sessions --verbose      # See keywords and content types")
		fmt.Println("  llm-web-parser db use latest              # Switch to latest session")
		fmt.Println("  llm-web-parser db use 7                   # Switch to session 7")
		return nil
	}

	// Open database
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Check for "latest" keyword
	var sessionID int64
	if c.Args().First() == "latest" {
		// Get latest session (highest ID)
		sessions, err := database.ListSessions(1)
		if err != nil {
			return fmt.Errorf("failed to get latest session: %w", err)
		}
		if len(sessions) == 0 {
			return fmt.Errorf("no sessions found")
		}
		sessionID = sessions[0].SessionID
	} else {
		// Parse session ID argument
		_, err := fmt.Sscanf(c.Args().First(), "%d", &sessionID)
		if err != nil {
			return fmt.Errorf("invalid session ID: %s (use number or 'latest')", c.Args().First())
		}

		if sessionID <= 0 {
			return fmt.Errorf("invalid session ID: %d (must be > 0)", sessionID)
		}
	}

	// Verify session exists
	session, err := database.GetSessionByID(sessionID)
	if err != nil {
		return fmt.Errorf("session %d not found", sessionID)
	}

	// Set active session
	if err := setActiveSession(sessionID); err != nil {
		return err
	}

	fmt.Printf("Active session set to %d\n", sessionID)
	fmt.Printf("Created: %s (%d URLs, %d success, %d failed)\n",
		session.CreatedAt.Format("2006-01-02"),
		session.URLCount, session.SuccessCount, session.FailedCount)
	fmt.Println("\nNow you can use commands without --session flag:")
	fmt.Printf("  llm-web-parser db urls\n")
	fmt.Printf("  llm-web-parser db show <url_id>\n")

	return nil
}
