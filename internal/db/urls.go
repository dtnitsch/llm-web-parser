package db

import (
	"fmt"
	"strings"

	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/urfave/cli/v2"
)

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
	verbose := c.Bool("verbose")

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
	} else if verbose {
		// Verbose mode: Show all URLs with rich metadata (3-line format)
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
	} else {
		// Default: Compact single-line format with column alignment
		fmt.Printf("Session: %d\n", sessionID)
		for _, u := range urls {
			// Build type label (10 chars wide for alignment)
			var typeLabel string
			if u.ContentType != "" && u.ContentType != "unknown" {
				typeLabel = u.ContentType
				if u.HasCodeExamples {
					typeLabel += "/code"
				}
			} else if u.HasCodeExamples {
				typeLabel = "code"
			}

			// Format: " #55  docs/code  https://...  →  keywords"
			//         " #56            https://...  →  keywords"
			if len(u.TopKeywords) > 0 {
				fmt.Printf(" #%-3d  %-10s  %s  →  %s\n",
					u.URLID,
					typeLabel,
					u.URL,
					strings.Join(u.TopKeywords, ", "))
			} else {
				fmt.Printf(" #%-3d  %-10s  %s\n",
					u.URLID,
					typeLabel,
					u.URL)
			}
		}
	}

	return nil
}
