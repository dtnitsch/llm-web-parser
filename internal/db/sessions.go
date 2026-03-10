package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/urfave/cli/v2"
)

func SessionsAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	limit := c.Int("limit")
	sessions, err := database.ListSessions(limit)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
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

	fmt.Printf("\nTotal: %d sessions\n", len(sessions))
	fmt.Printf("\nTip: Use 'llm-web-parser db session <id>' to see details\n")

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
