package corpus

import (
	"fmt"
	"strings"

	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
)

// SessionStats holds statistics about a session for generating suggestions.
type SessionStats struct {
	TotalURLs       int
	ContentTypes    map[string]int // content_type -> count
	HasAbstract     int
	HasCodeExamples int
	HasInfobox      int
	HasTOC          int
}

// SuggestFromSession generates query suggestions based on session contents.
func SuggestFromSession(sessionID int64) (string, error) {
	db, err := openDB()
	if err != nil {
		return "", fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get session stats
	stats, err := getSessionStats(db, sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session stats: %w", err)
	}

	// Generate suggestions
	return formatSuggestions(sessionID, stats), nil
}

// getSessionStats queries database for session statistics.
func getSessionStats(db *dbpkg.DB, sessionID int64) (*SessionStats, error) {
	stats := &SessionStats{
		ContentTypes: make(map[string]int),
	}

	// Get total URLs in session
	err := db.QueryRow(`
		SELECT COUNT(DISTINCT url_id)
		FROM session_urls
		WHERE session_id = ?
	`, sessionID).Scan(&stats.TotalURLs)
	if err != nil {
		return nil, fmt.Errorf("failed to get total URLs: %w", err)
	}

	// Get content type distribution
	rows, err := db.Query(`
		SELECT u.content_type, COUNT(*)
		FROM urls u
		JOIN session_urls su ON u.url_id = su.url_id
		WHERE su.session_id = ? AND u.content_type IS NOT NULL
		GROUP BY u.content_type
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get content types: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contentType string
		var count int
		if err := rows.Scan(&contentType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan content type: %w", err)
		}
		stats.ContentTypes[contentType] = count
	}

	// Get feature flags
	err = db.QueryRow(`
		SELECT
			COUNT(CASE WHEN u.has_abstract = 1 THEN 1 END),
			COUNT(CASE WHEN u.has_code_examples = 1 THEN 1 END),
			COUNT(CASE WHEN u.has_infobox = 1 THEN 1 END),
			COUNT(CASE WHEN u.has_toc = 1 THEN 1 END)
		FROM urls u
		JOIN session_urls su ON u.url_id = su.url_id
		WHERE su.session_id = ?
	`, sessionID).Scan(
		&stats.HasAbstract,
		&stats.HasCodeExamples,
		&stats.HasInfobox,
		&stats.HasTOC,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get feature flags: %w", err)
	}

	return stats, nil
}

// formatSuggestions generates human-readable suggestions.
func formatSuggestions(sessionID int64, stats *SessionStats) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("\n📊 Session %d Analysis:\n", sessionID))
	sb.WriteString(fmt.Sprintf("  %d URLs parsed\n", stats.TotalURLs))

	// Content type breakdown
	if len(stats.ContentTypes) > 0 {
		for contentType, count := range stats.ContentTypes {
			pct := float64(count) / float64(stats.TotalURLs) * 100
			sb.WriteString(fmt.Sprintf("  %d %s (%.0f%%)\n", count, contentType, pct))
		}
	}

	// Feature flags
	if stats.HasCodeExamples > 0 {
		sb.WriteString(fmt.Sprintf("  %d with code examples\n", stats.HasCodeExamples))
	}
	if stats.HasAbstract > 0 {
		sb.WriteString(fmt.Sprintf("  %d with abstracts\n", stats.HasAbstract))
	}
	if stats.HasTOC > 0 {
		sb.WriteString(fmt.Sprintf("  %d with table of contents\n", stats.HasTOC))
	}

	// Suggested queries
	sb.WriteString("\n💡 Suggested queries:\n")

	suggestions := generateSuggestions(sessionID, stats)
	for _, suggestion := range suggestions {
		sb.WriteString(fmt.Sprintf("  %s\n", suggestion))
	}

	sb.WriteString("\nAdvanced: lwp corpus --help\n")

	return sb.String()
}

// generateSuggestions creates query suggestions based on stats.
func generateSuggestions(sessionID int64, stats *SessionStats) []string {
	var suggestions []string

	// Always suggest basic queries if there's data
	if stats.TotalURLs > 0 {
		// Add EXTRACT suggestion first (most useful starting point)
		suggestions = append(suggestions,
			fmt.Sprintf("lwp corpus extract --session=%d", sessionID))

		// Get top keyword and suggest filtering by it
		topKeyword := getTopKeywordForSession(sessionID)
		if topKeyword != "" {
			suggestions = append(suggestions,
				fmt.Sprintf("lwp corpus query --session=%d --filter=\"keyword:%s\"  # Find %s content", sessionID, topKeyword, topKeyword))
		}

		// Content type queries
		for contentType := range stats.ContentTypes {
			suggestions = append(suggestions,
				fmt.Sprintf("lwp corpus query --session=%d --filter=\"content_type=%s\"", sessionID, contentType))
		}

		// Feature-based queries
		if stats.HasCodeExamples > 0 {
			suggestions = append(suggestions,
				fmt.Sprintf("lwp corpus query --session=%d --filter=\"has_code\"", sessionID))
		}

		if stats.HasAbstract > 0 {
			suggestions = append(suggestions,
				fmt.Sprintf("lwp corpus query --session=%d --filter=\"has_abstract\"", sessionID))
		}

		// Combined keyword + feature query
		if topKeyword != "" && stats.HasCodeExamples > 0 {
			suggestions = append(suggestions,
				fmt.Sprintf("lwp corpus query --session=%d --filter=\"keyword:%s AND has_code\"", sessionID, topKeyword))
		}
	}

	// Limit to top 6 suggestions
	if len(suggestions) > 6 {
		suggestions = suggestions[:6]
	}

	return suggestions
}

// getTopKeywordForSession retrieves the most common keyword from the session.
func getTopKeywordForSession(sessionID int64) string {
	db, err := openDB()
	if err != nil {
		return ""
	}
	defer db.Close()

	// Get first URL's top_keywords from session
	var topKeywords string
	err = db.QueryRow(`
		SELECT u.top_keywords
		FROM urls u
		JOIN session_urls su ON u.url_id = su.url_id
		WHERE su.session_id = ? AND u.top_keywords IS NOT NULL
		LIMIT 1
	`, sessionID).Scan(&topKeywords)

	if err != nil || topKeywords == "" {
		return ""
	}

	// Parse JSON array: ["error:97","type:163",...]
	// Extract first keyword (highest count should be first, but they're sorted by name in storage)
	// Simple parsing: find first "word:count" pattern
	if len(topKeywords) < 5 {
		return ""
	}

	// Remove leading [ and quotes
	topKeywords = strings.TrimPrefix(topKeywords, "[\"")

	// Find first colon to get the keyword
	colonIdx := strings.Index(topKeywords, ":")
	if colonIdx == -1 {
		return ""
	}

	keyword := topKeywords[:colonIdx]
	return keyword
}
