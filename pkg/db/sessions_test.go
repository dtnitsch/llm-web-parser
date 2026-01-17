package db

import (
	"testing"
	"time"
)

func TestFindOrCreateSession_NewSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urls := []string{"https://example.com", "https://example.org"}
	originalURLs := urls // Same as sanitized for this test
	features := "full-parse"
	parseMode := "full"
	maxAge := 1 * time.Hour

	sessionID, cacheHit, err := db.FindOrCreateSession(originalURLs, urls, features, parseMode, maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() error = %v", err)
	}

	if cacheHit {
		t.Error("FindOrCreateSession() cacheHit = true, want false for new session")
	}

	if sessionID == 0 {
		t.Error("FindOrCreateSession() returned 0 session ID")
	}

	// Verify session was created
	session, err := db.GetSessionByID(sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID() error = %v", err)
	}

	if session.URLCount != 2 {
		t.Errorf("session.URLCount = %d, want 2", session.URLCount)
	}
	if session.Features != features {
		t.Errorf("session.Features = %q, want %q", session.Features, features)
	}
	if session.ParseMode != parseMode {
		t.Errorf("session.ParseMode = %q, want %q", session.ParseMode, parseMode)
	}
}

func TestFindOrCreateSession_CacheHit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urls := []string{"https://example.com", "https://example.org"}
	maxAge := 1 * time.Hour

	// Create first session
	sessionID1, cacheHit1, err := db.FindOrCreateSession(urls, urls, "full-parse", "full", maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() first call error = %v", err)
	}
	if cacheHit1 {
		t.Error("First call should not be cache hit")
	}

	// Second call with same URLs should hit cache
	sessionID2, cacheHit2, err := db.FindOrCreateSession(urls, urls, "full-parse", "full", maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() second call error = %v", err)
	}

	if !cacheHit2 {
		t.Error("FindOrCreateSession() second call cacheHit = false, want true")
	}

	if sessionID1 != sessionID2 {
		t.Errorf("session IDs don't match: %d vs %d", sessionID1, sessionID2)
	}
}

func TestFindOrCreateSession_DifferentURLs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urls1 := []string{"https://example.com"}
	urls2 := []string{"https://example.org"}
	maxAge := 1 * time.Hour

	sessionID1, _, err := db.FindOrCreateSession(urls1, urls1, "", "", maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() first call error = %v", err)
	}

	sessionID2, cacheHit, err := db.FindOrCreateSession(urls2, urls2, "", "", maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() second call error = %v", err)
	}

	if cacheHit {
		t.Error("Different URLs should not hit cache")
	}

	if sessionID1 == sessionID2 {
		t.Error("Different URL sets should create different sessions")
	}
}

func TestFindOrCreateSession_URLOrderIndependent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urls1 := []string{"https://example.com", "https://example.org"}
	urls2 := []string{"https://example.org", "https://example.com"} // Reversed order
	maxAge := 1 * time.Hour

	sessionID1, _, err := db.FindOrCreateSession(urls1, urls1, "", "", maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() first call error = %v", err)
	}

	sessionID2, cacheHit, err := db.FindOrCreateSession(urls2, urls2, "", "", maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() second call error = %v", err)
	}

	if !cacheHit {
		t.Error("Same URLs in different order should hit cache")
	}

	if sessionID1 != sessionID2 {
		t.Errorf("Same URL set should match same session: %d vs %d", sessionID1, sessionID2)
	}
}

func TestFindOrCreateSession_MaxAgeExpiry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urls := []string{"https://example.com"}
	maxAge := 100 * time.Millisecond

	// Create first session
	sessionID1, _, err := db.FindOrCreateSession(urls, urls, "", "", maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() first call error = %v", err)
	}

	// Wait for maxAge to expire
	time.Sleep(150 * time.Millisecond)

	// Second call should create new session (expired)
	sessionID2, cacheHit, err := db.FindOrCreateSession(urls, urls, "", "", maxAge)
	if err != nil {
		t.Fatalf("FindOrCreateSession() second call error = %v", err)
	}

	if cacheHit {
		t.Error("Expired session should not be cache hit")
	}

	if sessionID1 == sessionID2 {
		t.Error("Expired session should create new session ID")
	}
}

func TestInsertSessionResult(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create session and URL
	urlID, _ := db.InsertURL("https://example.com")
	sessionID, _, _ := db.FindOrCreateSession([]string{"https://example.com"}, []string{"https://example.com"}, "", "", 1*time.Hour)

	// Insert result
	err := db.InsertSessionResult(sessionID, urlID, "success", 200, "", "", 1024, 256)
	if err != nil {
		t.Fatalf("InsertSessionResult() error = %v", err)
	}

	// Verify result was inserted
	var status string
	var statusCode int
	var fileSizeBytes, estimatedTokens int64
	err = db.QueryRow(`
		SELECT status, status_code, file_size_bytes, estimated_tokens
		FROM session_results
		WHERE session_id = ? AND url_id = ?
	`, sessionID, urlID).Scan(&status, &statusCode, &fileSizeBytes, &estimatedTokens)
	if err != nil {
		t.Fatalf("failed to query session result: %v", err)
	}

	if status != "success" {
		t.Errorf("status = %q, want %q", status, "success")
	}
	if statusCode != 200 {
		t.Errorf("status_code = %d, want 200", statusCode)
	}
	if fileSizeBytes != 1024 {
		t.Errorf("file_size_bytes = %d, want 1024", fileSizeBytes)
	}
	if estimatedTokens != 256 {
		t.Errorf("estimated_tokens = %d, want 256", estimatedTokens)
	}
}

func TestUpdateSessionStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create session
	sessionID, _, _ := db.FindOrCreateSession([]string{"https://example.com"}, []string{"https://example.com"}, "", "", 1*time.Hour)

	// Update stats
	err := db.UpdateSessionStats(sessionID, 8, 2)
	if err != nil {
		t.Fatalf("UpdateSessionStats() error = %v", err)
	}

	// Verify stats were updated
	session, err := db.GetSessionByID(sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID() error = %v", err)
	}

	if session.SuccessCount != 8 {
		t.Errorf("success_count = %d, want 8", session.SuccessCount)
	}
	if session.FailedCount != 2 {
		t.Errorf("failed_count = %d, want 2", session.FailedCount)
	}
}

func TestGetSessionURLs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urls := []string{"https://example.com", "https://example.org", "https://example.net"}
	sessionID, _, _ := db.FindOrCreateSession(urls, urls, "", "", 1*time.Hour)

	// Get session URLs
	sessionURLs, err := db.GetSessionURLs(sessionID)
	if err != nil {
		t.Fatalf("GetSessionURLs() error = %v", err)
	}

	if len(sessionURLs) != 3 {
		t.Errorf("got %d URLs, want 3", len(sessionURLs))
	}

	// Verify URLs are correct (order may differ due to sorting)
	urlSet := make(map[string]bool)
	for _, u := range sessionURLs {
		urlSet[u.OriginalURL] = true
	}

	for _, expected := range urls {
		if !urlSet[expected] {
			t.Errorf("URL %q not found in session URLs", expected)
		}
	}
}

func TestSessionDir_Naming(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urls := []string{"https://example.com"}
	sessionID, _, err := db.FindOrCreateSession(urls, urls, "", "", 1*time.Hour)
	if err != nil {
		t.Fatalf("FindOrCreateSession() error = %v", err)
	}

	session, err := db.GetSessionByID(sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID() error = %v", err)
	}

	// Verify session_dir format: sessions/YYYY-MM-DD-{id}
	dateStr := time.Now().Format("2006-01-02")
	expectedPrefix := "sessions/" + dateStr
	if !contains(session.SessionDir, expectedPrefix) {
		t.Errorf("session_dir = %q, want to contain %q", session.SessionDir, expectedPrefix)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
