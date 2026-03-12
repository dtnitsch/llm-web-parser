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

	verbose := c.Bool("verbose")

	if verbose {
		// Verbose mode: Show all URLs with rich metadata (3-line format)
		fmt.Printf("Session: %d\n\n", sessionID)
		for i, u := range urls {
			// Line 1: URL ID and URL
			fmt.Printf("%2d. [#%d] %s\n", i+1, u.URLID, u.URL)

			// Line 2: Metadata - content type, code flag, confidence
			codeFlag := "no_code"
			if u.HasCodeExamples {
				codeFlag = "has_code"
			}
			fmt.Printf("    %s | %s | conf:%.1f\n",
				u.ContentType, codeFlag, u.DetectionConfidence)

			// Line 3: Keywords (prefer meta keywords, fallback to extracted)
			if len(u.MetaKeywords) > 0 {
				fmt.Printf("    Meta keywords: %s (from site)\n", strings.Join(u.MetaKeywords, ", "))
			} else if len(u.TopKeywords) > 0 {
				fmt.Printf("    Keywords: %s (extracted)\n", strings.Join(u.TopKeywords, ", "))
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
			// Prefer meta keywords (author intent), fallback to extracted keywords
			var keywords []string
			if len(u.MetaKeywords) > 0 {
				keywords = u.MetaKeywords
			} else {
				keywords = u.TopKeywords
			}

			if len(keywords) > 0 {
				fmt.Printf(" #%-3d  %-10s  %s  →  %s\n",
					u.URLID,
					typeLabel,
					u.URL,
					strings.Join(keywords, ", "))
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
