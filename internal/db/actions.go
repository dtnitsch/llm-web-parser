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
	fmt.Printf("\nTip: Use 'lwp db session <id>' to see details\n")

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

	fmt.Printf("\nTip: Use 'lwp db get %d' to see summary YAML\n", sessionID)

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

// urlsAction shows URLs for a session with sanitization tracking and metadata
func UrlsAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	sessionID, err := GetSessionIDOrLatest(c, database)
	if err != nil {
		return err
	}

	// Get URLs with full metadata
	urls, err := database.GetSessionURLsWithMetadata(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session URLs: %w", err)
	}

	if len(urls) == 0 {
		fmt.Printf("No URLs found for session %d\n", sessionID)
		return nil
	}

	sanitizedOnly := c.Bool("sanitized")

	if sanitizedOnly {
		// Show only sanitized URLs (existing behavior)
		sanitizedURLs := []dbpkg.URLWithMetadata{}
		for _, u := range urls {
			if u.WasSanitized {
				sanitizedURLs = append(sanitizedURLs, u)
			}
		}

		if len(sanitizedURLs) == 0 {
			fmt.Printf("Session: %d\n\n", sessionID)
			fmt.Printf("No URLs were auto-cleaned in this session\n")
			return nil
		}

		fmt.Printf("Session: %d\n\n", sessionID)
		fmt.Printf("Auto-cleaned URLs:\n\n")
		for i, u := range sanitizedURLs {
			fmt.Printf("%2d. [#%d] Original: %s\n", i+1, u.URLID, u.OriginalURL)
			fmt.Printf("          Cleaned:  %s\n\n", u.URL)
		}
	} else {
		// Show all URLs with rich metadata
		fmt.Printf("Session: %d\n\n", sessionID)
		for i, u := range urls {
			// Line 1: URL with sanitization indicator
			if u.WasSanitized {
				fmt.Printf("%2d. [#%d] %s (cleaned)\n", i+1, u.URLID, u.URL)
			} else {
				fmt.Printf("%2d. [#%d] %s\n", i+1, u.URLID, u.URL)
			}

			// Line 2: Metadata - content type, code flag, confidence
			codeFlag := "no_code"
			if u.HasCodeExamples {
				codeFlag = "has_code"
			}
			fmt.Printf("    %s | %s | conf:%.1f\n",
				u.ContentType, codeFlag, u.DetectionConfidence)

			// Line 3: Keywords (if available)
			if len(u.TopKeywords) > 0 {
				fmt.Printf("    Keywords: %s\n", strings.Join(u.TopKeywords, ", "))
			}

			fmt.Println() // Blank line between URLs
		}
	}

	return nil
}

// showAction shows parsed JSON for a URL by ID or URL
func ShowAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("URL ID or URL required\nUsage: lwp db show <url_id_or_url>\nExample: lwp db show 123 OR lwp db show 6,7,8 OR lwp db show https://example.com")
	}

	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	manager, err := artifact_manager.NewManager(artifact_manager.DefaultBaseDir, 0)
	if err != nil {
		return fmt.Errorf("failed to initialize artifact manager: %w", err)
	}

	arg := c.Args().First()

	// Check if argument contains comma (batch mode)
	if strings.Contains(arg, ",") {
		ids := strings.Split(arg, ",")
		results := make([]string, 0, len(ids))

		for _, id := range ids {
			id = strings.TrimSpace(id)
			url, err := ResolveURLFromIDOrURL(id, database)
			if err != nil {
				return fmt.Errorf("failed to resolve ID %s: %w", id, err)
			}

			filePath, err := manager.GetArtifactPath(artifact_manager.ParsedJSONDir, url, ".json")
			if err != nil {
				return fmt.Errorf("failed to get artifact path for %s: %w", url, err)
			}

			data, err := os.ReadFile(filepath.Clean(filePath))
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("parsed JSON not found for URL: %s\n\nThis URL may not have been fetched yet. Try:\n  lwp fetch --urls \"%s\"", url, url)
				}
				return fmt.Errorf("failed to read file for %s: %w", url, err)
			}

			results = append(results, string(data))
		}

		// Print results as JSON array
		fmt.Print("[\n")
		for i, result := range results {
			fmt.Print(result)
			if i < len(results)-1 {
				fmt.Print(",\n")
			}
		}
		fmt.Print("\n]\n")
		return nil
	}

	// Single URL/ID mode
	url, err := ResolveURLFromIDOrURL(arg, database)
	if err != nil {
		return err
	}

	filePath, err := manager.GetArtifactPath(artifact_manager.ParsedJSONDir, url, ".json")
	if err != nil {
		return fmt.Errorf("failed to get artifact path: %w", err)
	}

	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("parsed JSON not found for URL: %s\n\nThis URL may not have been fetched yet. Try:\n  lwp fetch --urls \"%s\"", url, url)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

// rawAction shows raw HTML for a URL by ID or URL
func RawAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("URL ID or URL required\nUsage: lwp db raw <url_id_or_url>\nExample: lwp db raw 123 OR lwp db raw 6,7,8 OR lwp db raw https://example.com")
	}

	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	manager, err := artifact_manager.NewManager(artifact_manager.DefaultBaseDir, 0)
	if err != nil {
		return fmt.Errorf("failed to initialize artifact manager: %w", err)
	}

	arg := c.Args().First()

	// Check if argument contains comma (batch mode)
	if strings.Contains(arg, ",") {
		ids := strings.Split(arg, ",")

		for i, id := range ids {
			id = strings.TrimSpace(id)
			url, err := ResolveURLFromIDOrURL(id, database)
			if err != nil {
				return fmt.Errorf("failed to resolve ID %s: %w", id, err)
			}

			filePath, err := manager.GetArtifactPath(artifact_manager.RawHTMLDir, url, ".html")
			if err != nil {
				return fmt.Errorf("failed to get artifact path for %s: %w", url, err)
			}

			data, err := os.ReadFile(filepath.Clean(filePath))
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("raw HTML not found for URL: %s\n\nThis URL may not have been fetched yet. Try:\n  lwp fetch --urls \"%s\"", url, url)
				}
				return fmt.Errorf("failed to read file for %s: %w", url, err)
			}

			if i > 0 {
				fmt.Print("\n<!-- ===== Next URL ===== -->\n\n")
			}
			fmt.Print(string(data))
		}
		return nil
	}

	// Single URL/ID mode
	url, err := ResolveURLFromIDOrURL(arg, database)
	if err != nil {
		return err
	}

	filePath, err := manager.GetArtifactPath(artifact_manager.RawHTMLDir, url, ".html")
	if err != nil {
		return fmt.Errorf("failed to get artifact path: %w", err)
	}

	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("raw HTML not found for URL: %s\n\nThis URL may not have been fetched yet. Try:\n  lwp fetch --urls \"%s\"", url, url)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

func FindURLAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("URL required\nUsage: lwp db find-url <url>\nExample: lwp db find-url https://example.com")
	}

	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	url := c.Args().First()
	urlID, err := database.GetURLID(url)
	if err != nil {
		return fmt.Errorf("URL not found in database: %s\nNote: Only fetched URLs are tracked", url)
	}

	fmt.Printf("[#%d] %s\n", urlID, url)
	return nil
}
