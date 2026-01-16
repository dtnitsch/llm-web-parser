package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Session represents a fetch session
type Session struct {
	SessionID    int64
	CreatedAt    time.Time
	URLCount     int
	SuccessCount int
	FailedCount  int
	Features     string
	ParseMode    string
	SessionDir   string
}

// FindOrCreateSession checks if a session exists for this URL set.
// Returns (session_id, cache_hit, error).
// If cache_hit is true, the session already exists and is fresh.
// originalURLs are the URLs before sanitization, urls are after sanitization.
func (db *DB) FindOrCreateSession(originalURLs, urls []string, features, parseMode string, maxAge time.Duration) (int64, bool, error) {
	// Sort URLs for consistency (use sanitized URLs for sorting/matching)
	sortedURLs := make([]string, len(urls))
	sortedOriginals := make([]string, len(originalURLs))
	copy(sortedURLs, urls)
	copy(sortedOriginals, originalURLs)

	// Sort both arrays the same way
	type urlPair struct {
		original  string
		sanitized string
	}
	pairs := make([]urlPair, len(urls))
	for i := range urls {
		pairs[i] = urlPair{original: sortedOriginals[i], sanitized: sortedURLs[i]}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].sanitized < pairs[j].sanitized
	})
	for i := range pairs {
		sortedURLs[i] = pairs[i].sanitized
		sortedOriginals[i] = pairs[i].original
	}

	// Get or insert URL IDs
	urlIDs := make([]int64, len(sortedURLs))
	for i, rawURL := range sortedURLs {
		urlID, err := db.InsertURL(rawURL)
		if err != nil {
			return 0, false, fmt.Errorf("failed to insert URL %s: %w", rawURL, err)
		}
		urlIDs[i] = urlID
	}

	// Find matching session
	sessionID, createdAt, found, err := db.findSessionByURLs(urlIDs)
	if err != nil {
		return 0, false, err
	}

	if found {
		// Check freshness
		if maxAge > 0 {
			age := time.Since(createdAt)
			if age <= maxAge {
				return sessionID, true, nil // Cache hit!
			}
			// Session exists but is stale, create new one
		} else {
			// maxAge == 0 means no expiry
			return sessionID, true, nil
		}
	}

	// Create new session
	sessionID, err = db.createSession(len(urls), features, parseMode)
	if err != nil {
		return 0, false, err
	}

	// Link URLs to session with sanitization tracking
	for i, urlID := range urlIDs {
		if err := db.InsertSessionURL(sessionID, urlID, sortedOriginals[i], sortedURLs[i]); err != nil {
			return 0, false, err
		}
	}

	return sessionID, false, nil
}

// findSessionByURLs finds a session that matches this exact URL set
func (db *DB) findSessionByURLs(urlIDs []int64) (sessionID int64, createdAt time.Time, found bool, err error) {
	// Build placeholders for IN clause
	placeholders := make([]string, len(urlIDs))
	args := make([]interface{}, len(urlIDs)+1)
	for i, id := range urlIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	args[len(urlIDs)] = len(urlIDs)

	// Query for session with exact URL match
	query := fmt.Sprintf(`
		SELECT s.session_id, s.created_at
		FROM sessions s
		JOIN session_urls su ON s.session_id = su.session_id
		WHERE su.url_id IN (%s)
		GROUP BY s.session_id
		HAVING COUNT(DISTINCT su.url_id) = ?
		ORDER BY s.created_at DESC
		LIMIT 1
	`, strings.Join(placeholders, ","))

	err = db.QueryRow(query, args...).Scan(&sessionID, &createdAt)
	if err == sql.ErrNoRows {
		return 0, time.Time{}, false, nil
	}
	if err != nil {
		return 0, time.Time{}, false, fmt.Errorf("failed to find session: %w", err)
	}

	// Verify the session has exactly the right URLs (not a superset)
	var urlCount int
	err = db.QueryRow("SELECT url_count FROM sessions WHERE session_id = ?", sessionID).Scan(&urlCount)
	if err != nil {
		return 0, time.Time{}, false, fmt.Errorf("failed to verify session url_count: %w", err)
	}

	if urlCount != len(urlIDs) {
		// Session has different number of URLs, not an exact match
		return 0, time.Time{}, false, nil
	}

	return sessionID, createdAt, true, nil
}

// createSession creates a new session record
func (db *DB) createSession(urlCount int, features, parseMode string) (int64, error) {
	// Generate session directory name (will be updated later with actual timestamp)
	timestamp := time.Now()
	dateStr := timestamp.Format("2006-01-02")

	// Insert with placeholder session_dir, will update after we get the ID
	result, err := db.Exec(`
		INSERT INTO sessions (url_count, features, parse_mode, session_dir)
		VALUES (?, ?, ?, ?)
	`, urlCount, features, parseMode, "temp")
	if err != nil {
		return 0, fmt.Errorf("failed to create session: %w", err)
	}

	sessionID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get session ID: %w", err)
	}

	// Update session_dir with actual session ID
	sessionDir := fmt.Sprintf("sessions/%s-%d", dateStr, sessionID)
	_, err = db.Exec("UPDATE sessions SET session_dir = ? WHERE session_id = ?", sessionDir, sessionID)
	if err != nil {
		return 0, fmt.Errorf("failed to update session_dir: %w", err)
	}

	return sessionID, nil
}

// InsertSessionURL links a URL to a session, tracking if it was sanitized
func (db *DB) InsertSessionURL(sessionID, urlID int64, originalURL, sanitizedURL string) error {
	wasSanitized := originalURL != sanitizedURL
	var origURLToStore interface{}
	if wasSanitized {
		origURLToStore = originalURL
	} else {
		origURLToStore = nil
	}

	_, err := db.Exec(`
		INSERT INTO session_urls (session_id, url_id, was_sanitized, original_url)
		VALUES (?, ?, ?, ?)
	`, sessionID, urlID, wasSanitized, origURLToStore)
	if err != nil {
		return fmt.Errorf("failed to insert session_url: %w", err)
	}
	return nil
}

// InsertSessionResult records a result for a URL in a session
func (db *DB) InsertSessionResult(sessionID, urlID int64, status string, statusCode int, errorType, errorMessage string, fileSizeBytes int64, estimatedTokens int) error {
	_, err := db.Exec(`
		INSERT INTO session_results (session_id, url_id, status, status_code, error_type, error_message, file_size_bytes, estimated_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, sessionID, urlID, status, statusCode, errorType, errorMessage, fileSizeBytes, estimatedTokens)
	if err != nil {
		return fmt.Errorf("failed to insert session result: %w", err)
	}
	return nil
}

// UpdateSessionStats updates the success and failed counts for a session
func (db *DB) UpdateSessionStats(sessionID int64, successCount, failedCount int) error {
	_, err := db.Exec(`
		UPDATE sessions
		SET success_count = ?, failed_count = ?
		WHERE session_id = ?
	`, successCount, failedCount, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session stats: %w", err)
	}
	return nil
}

// GetSessionByID retrieves a session by its ID
func (db *DB) GetSessionByID(sessionID int64) (*Session, error) {
	var session Session
	err := db.QueryRow(`
		SELECT session_id, created_at, url_count, success_count, failed_count, features, parse_mode, session_dir
		FROM sessions
		WHERE session_id = ?
	`, sessionID).Scan(
		&session.SessionID,
		&session.CreatedAt,
		&session.URLCount,
		&session.SuccessCount,
		&session.FailedCount,
		&session.Features,
		&session.ParseMode,
		&session.SessionDir,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session %d not found", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &session, nil
}

// GetSessionURLs retrieves all URLs for a session
func (db *DB) GetSessionURLs(sessionID int64) ([]URLInfo, error) {
	rows, err := db.Query(`
		SELECT u.url_id, u.original_url, u.canonical_url, u.domain
		FROM urls u
		JOIN session_urls su ON u.url_id = su.url_id
		WHERE su.session_id = ?
		ORDER BY su.id
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session URLs: %w", err)
	}
	defer rows.Close()

	var urls []URLInfo
	for rows.Next() {
		var info URLInfo
		if err := rows.Scan(&info.URLID, &info.OriginalURL, &info.CanonicalURL, &info.Domain); err != nil {
			return nil, fmt.Errorf("failed to scan URL: %w", err)
		}
		urls = append(urls, info)
	}

	return urls, nil
}

// SessionResult represents a result within a session
type SessionResult struct {
	URL             string
	Status          string
	StatusCode      int
	ErrorType       string
	ErrorMessage    string
	FileSizeBytes   int64
	EstimatedTokens int
}

// GetSessionResults retrieves all results for a session
func (db *DB) GetSessionResults(sessionID int64) ([]SessionResult, error) {
	rows, err := db.Query(`
		SELECT u.original_url, sr.status, sr.status_code, sr.error_type, sr.error_message,
		       sr.file_size_bytes, sr.estimated_tokens
		FROM session_results sr
		JOIN urls u ON sr.url_id = u.url_id
		WHERE sr.session_id = ?
		ORDER BY sr.result_id
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session results: %w", err)
	}
	defer rows.Close()

	var results []SessionResult
	for rows.Next() {
		var r SessionResult
		var errorType, errorMessage sql.NullString
		if err := rows.Scan(&r.URL, &r.Status, &r.StatusCode, &errorType, &errorMessage,
			&r.FileSizeBytes, &r.EstimatedTokens); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		if errorType.Valid {
			r.ErrorType = errorType.String
		}
		if errorMessage.Valid {
			r.ErrorMessage = errorMessage.String
		}
		results = append(results, r)
	}

	return results, nil
}

// ListSessions retrieves all sessions ordered by most recent first
func (db *DB) ListSessions(limit int) ([]Session, error) {
	query := `
		SELECT session_id, created_at, url_count, success_count, failed_count,
		       features, parse_mode, session_dir
		FROM sessions
		ORDER BY created_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.SessionID, &s.CreatedAt, &s.URLCount, &s.SuccessCount,
			&s.FailedCount, &s.Features, &s.ParseMode, &s.SessionDir); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// QuerySessions filters sessions based on criteria
func (db *DB) QuerySessions(todayOnly bool, failedOnly bool, urlPattern string) ([]Session, error) {
	query := `
		SELECT DISTINCT s.session_id, s.created_at, s.url_count, s.success_count,
		       s.failed_count, s.features, s.parse_mode, s.session_dir
		FROM sessions s
	`

	var conditions []string
	var args []interface{}

	if todayOnly {
		conditions = append(conditions, "DATE(s.created_at) = DATE('now')")
	}

	if failedOnly {
		conditions = append(conditions, "s.failed_count > 0")
	}

	if urlPattern != "" {
		query += `
		JOIN session_urls su ON s.session_id = su.session_id
		JOIN urls u ON su.url_id = u.url_id
		`
		conditions = append(conditions, "u.original_url LIKE ?")
		args = append(args, "%"+urlPattern+"%")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY s.created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.SessionID, &s.CreatedAt, &s.URLCount, &s.SuccessCount,
			&s.FailedCount, &s.Features, &s.ParseMode, &s.SessionDir); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// URLWithSanitization represents a URL with its sanitization status
type URLWithSanitization struct {
	URLID        int64
	URL          string
	WasSanitized bool
	OriginalURL  string
}

// URLWithMetadata represents a URL with full metadata for triage
type URLWithMetadata struct {
	// Basic info
	URLID        int64
	URL          string
	Domain       string
	WasSanitized bool
	OriginalURL  string

	// Content classification
	ContentType         string  // docs, blog, landing, academic, etc.
	ContentSubtype      string  // api-docs, reference, etc.
	DetectionConfidence float64 // 0-10

	// Feature flags
	HasCodeExamples bool
	HasAbstract     bool
	HasTOC          bool

	// Structure counts
	SectionCount   int
	CitationCount  int
	CodeBlockCount int

	// Keywords (top 5 for display)
	TopKeywords []string
}

// GetSessionURLsWithSanitization retrieves URLs for a session with sanitization info
func (db *DB) GetSessionURLsWithSanitization(sessionID int64) ([]URLWithSanitization, error) {
	rows, err := db.Query(`
		SELECT u.url_id, u.original_url, su.was_sanitized, su.original_url
		FROM urls u
		JOIN session_urls su ON u.url_id = su.url_id
		WHERE su.session_id = ?
		ORDER BY su.id
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session URLs: %w", err)
	}
	defer rows.Close()

	var urls []URLWithSanitization
	for rows.Next() {
		var info URLWithSanitization
		var origURL sql.NullString
		if err := rows.Scan(&info.URLID, &info.URL, &info.WasSanitized, &origURL); err != nil {
			return nil, fmt.Errorf("failed to scan URL: %w", err)
		}
		if origURL.Valid {
			info.OriginalURL = origURL.String
		}
		urls = append(urls, info)
	}

	return urls, nil
}

// GetSessionURLsWithMetadata retrieves URLs with full metadata for triage
func (db *DB) GetSessionURLsWithMetadata(sessionID int64) ([]URLWithMetadata, error) {
	query := `
		SELECT
			u.url_id,
			u.original_url,
			u.domain,
			su.was_sanitized,
			su.original_url,
			COALESCE(u.content_type, 'unknown'),
			COALESCE(u.content_subtype, ''),
			COALESCE(u.detection_confidence, 0.0),
			COALESCE(u.has_code_examples, 0),
			COALESCE(u.has_abstract, 0),
			COALESCE(u.has_toc, 0),
			COALESCE(u.section_count, 0),
			COALESCE(u.citation_count, 0),
			COALESCE(u.code_block_count, 0),
			COALESCE(u.top_keywords, '[]')
		FROM urls u
		JOIN session_urls su ON u.url_id = su.url_id
		WHERE su.session_id = ?
		ORDER BY su.id
	`

	rows, err := db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session URLs with metadata: %w", err)
	}
	defer rows.Close()

	var urls []URLWithMetadata
	for rows.Next() {
		var u URLWithMetadata
		var topKeywordsJSON string
		var sanitizedOriginal sql.NullString

		err := rows.Scan(
			&u.URLID,
			&u.URL,
			&u.Domain,
			&u.WasSanitized,
			&sanitizedOriginal,
			&u.ContentType,
			&u.ContentSubtype,
			&u.DetectionConfidence,
			&u.HasCodeExamples,
			&u.HasAbstract,
			&u.HasTOC,
			&u.SectionCount,
			&u.CitationCount,
			&u.CodeBlockCount,
			&topKeywordsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan URL metadata: %w", err)
		}

		if u.WasSanitized && sanitizedOriginal.Valid {
			u.OriginalURL = sanitizedOriginal.String
		}

		// Parse top_keywords JSON: ["error:97", "type:163", ...]
		u.TopKeywords = parseTopKeywordsForDisplay(topKeywordsJSON, 5)

		urls = append(urls, u)
	}

	return urls, nil
}

// parseTopKeywordsForDisplay extracts top N keyword names from JSON array
func parseTopKeywordsForDisplay(jsonStr string, limit int) []string {
	// JSON format: ["error:97","type:163","value:112",...]
	// Extract just the keyword names (before the colon)

	if jsonStr == "" || jsonStr == "[]" {
		return []string{}
	}

	// Simple parsing: split by comma, extract keyword before colon
	keywords := []string{}

	// Remove brackets and quotes
	jsonStr = strings.Trim(jsonStr, "[]")
	if jsonStr == "" {
		return []string{}
	}

	parts := strings.Split(jsonStr, ",")
	for i, part := range parts {
		if i >= limit {
			break
		}

		// Remove quotes: "error:97" -> error:97
		part = strings.Trim(part, "\"")

		// Extract keyword before colon
		colonIdx := strings.Index(part, ":")
		if colonIdx > 0 {
			keyword := part[:colonIdx]
			keywords = append(keywords, keyword)
		}
	}

	return keywords
}

// CountSanitizedURLs counts how many URLs were sanitized in a session
func (db *DB) CountSanitizedURLs(sessionID int64) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM session_urls
		WHERE session_id = ? AND was_sanitized = TRUE
	`, sessionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sanitized URLs: %w", err)
	}
	return count, nil
}

// GetURLByID retrieves a URL by its ID
func (db *DB) GetURLByID(urlID int64) (string, error) {
	var url string
	err := db.QueryRow(`
		SELECT original_url FROM urls WHERE url_id = ?
	`, urlID).Scan(&url)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("URL ID %d not found", urlID)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get URL: %w", err)
	}
	return url, nil
}
