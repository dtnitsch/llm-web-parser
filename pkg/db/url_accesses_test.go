package db

import (
	"testing"
)

func TestRecordAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/test")

	err := db.RecordAccess(urlID, 200, "", true)
	if err != nil {
		t.Fatalf("RecordAccess() failed: %v", err)
	}

	// Verify access was recorded
	var statusCode int
	var errorType string
	var success bool
	err = db.QueryRow(`
		SELECT status_code, error_type, success
		FROM url_accesses WHERE url_id = ?
	`, urlID).Scan(&statusCode, &errorType, &success)
	if err != nil {
		t.Fatalf("failed to query access: %v", err)
	}

	if statusCode != 200 {
		t.Errorf("status_code = %d, want 200", statusCode)
	}
	if errorType != "" {
		t.Errorf("error_type = %q, want empty", errorType)
	}
	if !success {
		t.Error("success = false, want true")
	}
}

func TestRecordAccess_Failed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/fail")

	err := db.RecordAccess(urlID, 0, "fetch_error", false)
	if err != nil {
		t.Fatalf("RecordAccess() failed: %v", err)
	}

	// Verify failed access
	var errorType string
	var success bool
	db.QueryRow("SELECT error_type, success FROM url_accesses WHERE url_id = ?", urlID).Scan(&errorType, &success)

	if errorType != "fetch_error" {
		t.Errorf("error_type = %q, want %q", errorType, "fetch_error")
	}
	if success {
		t.Error("success = true, want false")
	}
}

func TestGetLastAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/test")

	// Record multiple accesses
	db.RecordAccess(urlID, 200, "", true)
	db.RecordAccess(urlID, 404, "", false)
	db.RecordAccess(urlID, 200, "", true)

	// Get last access
	record, err := db.GetLastAccess(urlID)
	if err != nil {
		t.Fatalf("GetLastAccess() failed: %v", err)
	}

	if record == nil {
		t.Fatal("GetLastAccess() returned nil")
	}

	if record.StatusCode != 200 {
		t.Errorf("last access status_code = %d, want 200", record.StatusCode)
	}
	if !record.Success {
		t.Error("last access success = false, want true")
	}
}

func TestGetLastAccess_NoAccesses(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/new")

	record, err := db.GetLastAccess(urlID)
	if err != nil {
		t.Fatalf("GetLastAccess() failed: %v", err)
	}

	if record != nil {
		t.Error("GetLastAccess() should return nil for URL with no accesses")
	}
}

func TestRecordAccess_MultipleURLs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	url1, _ := db.InsertURL("https://example.com/page1")
	url2, _ := db.InsertURL("https://example.com/page2")

	db.RecordAccess(url1, 200, "", true)
	db.RecordAccess(url2, 404, "", false)

	// Check url1 has 1 access
	var count int
	db.QueryRow("SELECT COUNT(*) FROM url_accesses WHERE url_id = ?", url1).Scan(&count)
	if count != 1 {
		t.Errorf("url1 has %d accesses, want 1", count)
	}

	// Check url2 has 1 access
	db.QueryRow("SELECT COUNT(*) FROM url_accesses WHERE url_id = ?", url2).Scan(&count)
	if count != 1 {
		t.Errorf("url2 has %d accesses, want 1", count)
	}

	// Verify each URL's access is independent
	record1, _ := db.GetLastAccess(url1)
	if record1.StatusCode != 200 {
		t.Errorf("url1 status = %d, want 200", record1.StatusCode)
	}
	if !record1.Success {
		t.Error("url1 success = false, want true")
	}

	record2, _ := db.GetLastAccess(url2)
	if record2.StatusCode != 404 {
		t.Errorf("url2 status = %d, want 404", record2.StatusCode)
	}
	if record2.Success {
		t.Error("url2 success = true, want false")
	}
}
