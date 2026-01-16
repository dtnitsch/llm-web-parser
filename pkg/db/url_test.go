package db

import (
	"testing"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	// Use in-memory database for tests
	database := &DB{path: ":memory:"}
	var err error
	database.DB, err = openDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	if err := database.InitSchema(); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	return database
}

func TestInsertURL(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "simple HTTPS URL",
			url:     "https://example.com",
			wantErr: false,
		},
		{
			name:    "URL with path",
			url:     "https://example.com/path/to/page",
			wantErr: false,
		},
		{
			name:    "URL with query params",
			url:     "https://example.com/search?q=test&lang=en",
			wantErr: false,
		},
		{
			name:    "URL with fragment",
			url:     "https://example.com/page#section",
			wantErr: false,
		},
		{
			name:    "duplicate URL returns same ID",
			url:     "https://example.com",
			wantErr: false,
		},
	}

	var firstID int64
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urlID, err := db.InsertURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("InsertURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if urlID == 0 && !tt.wantErr {
				t.Error("InsertURL() returned 0 ID")
			}

			// First and last test use same URL, should get same ID
			if i == 0 {
				firstID = urlID
			}
			if i == len(tests)-1 && urlID != firstID {
				t.Errorf("Duplicate URL got different ID: got %d, want %d", urlID, firstID)
			}
		})
	}
}

func TestInsertURL_ParsesComponents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testURL := "https://doc.rust-lang.org/book/ch01-01-installation.html?version=1.0#intro"
	urlID, err := db.InsertURL(testURL)
	if err != nil {
		t.Fatalf("InsertURL() failed: %v", err)
	}

	// Query the URL components
	var scheme, domain, path, fragment string
	err = db.QueryRow(`
		SELECT scheme, domain, path, fragment
		FROM urls WHERE url_id = ?
	`, urlID).Scan(&scheme, &domain, &path, &fragment)
	if err != nil {
		t.Fatalf("failed to query URL: %v", err)
	}

	if scheme != "https" {
		t.Errorf("scheme = %q, want %q", scheme, "https")
	}
	if domain != "doc.rust-lang.org" {
		t.Errorf("domain = %q, want %q", domain, "doc.rust-lang.org")
	}
	if path != "/book/ch01-01-installation.html" {
		t.Errorf("path = %q, want %q", path, "/book/ch01-01-installation.html")
	}
	if fragment != "intro" {
		t.Errorf("fragment = %q, want %q", fragment, "intro")
	}
}

func TestInsertURL_QueryParams(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testURL := "https://example.com/search?q=golang&lang=en&limit=10"
	urlID, err := db.InsertURL(testURL)
	if err != nil {
		t.Fatalf("InsertURL() failed: %v", err)
	}

	// Query the params
	rows, err := db.Query("SELECT key, value FROM url_query_params WHERE url_id = ? ORDER BY key", urlID)
	if err != nil {
		t.Fatalf("failed to query params: %v", err)
	}
	defer rows.Close()

	expected := map[string]string{
		"lang":  "en",
		"limit": "10",
		"q":     "golang",
	}

	got := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		got[key] = value
	}

	if len(got) != len(expected) {
		t.Errorf("param count = %d, want %d", len(got), len(expected))
	}

	for k, v := range expected {
		if got[k] != v {
			t.Errorf("param %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestGetURLID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testURL := "https://example.com/test"
	wantID, err := db.InsertURL(testURL)
	if err != nil {
		t.Fatalf("InsertURL() failed: %v", err)
	}

	gotID, err := db.GetURLID(testURL)
	if err != nil {
		t.Fatalf("GetURLID() error = %v", err)
	}

	if gotID != wantID {
		t.Errorf("GetURLID() = %d, want %d", gotID, wantID)
	}

	// Test non-existent URL
	_, err = db.GetURLID("https://nonexistent.com")
	if err == nil {
		t.Error("GetURLID() with non-existent URL should return error")
	}
}
