package db

import (
	"testing"
)

func TestSetURLMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/test")

	err := db.SetURLMetadata(urlID, "domain", "has_doi", "true")
	if err != nil {
		t.Fatalf("SetURLMetadata() failed: %v", err)
	}

	// Verify metadata was set
	var value string
	err = db.QueryRow(`
		SELECT value FROM url_metadata
		WHERE url_id = ? AND namespace = ? AND key = ?
	`, urlID, "domain", "has_doi").Scan(&value)
	if err != nil {
		t.Fatalf("failed to query metadata: %v", err)
	}

	if value != "true" {
		t.Errorf("value = %q, want %q", value, "true")
	}
}

func TestSetURLMetadata_Upsert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/test")

	// Set initial value
	db.SetURLMetadata(urlID, "domain", "type", "academic")

	// Update value
	err := db.SetURLMetadata(urlID, "domain", "type", "documentation")
	if err != nil {
		t.Fatalf("SetURLMetadata() update failed: %v", err)
	}

	// Verify updated value
	var value string
	db.QueryRow("SELECT value FROM url_metadata WHERE url_id = ? AND namespace = ? AND key = ?",
		urlID, "domain", "type").Scan(&value)

	if value != "documentation" {
		t.Errorf("value = %q, want %q", value, "documentation")
	}

	// Verify only one row exists
	var count int
	db.QueryRow("SELECT COUNT(*) FROM url_metadata WHERE url_id = ? AND namespace = ? AND key = ?",
		urlID, "domain", "type").Scan(&count)
	if count != 1 {
		t.Errorf("metadata row count = %d, want 1", count)
	}
}

func TestSetURLMetadata_MultipleNamespaces(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://arxiv.org/abs/2103.00020")

	// Set metadata in different namespaces
	db.SetURLMetadata(urlID, "domain", "type", "academic")
	db.SetURLMetadata(urlID, "domain", "has_doi", "true")
	db.SetURLMetadata(urlID, "academic", "doi", "10.1234/example")
	db.SetURLMetadata(urlID, "academic", "citation_count", "42")

	// Query all metadata for this URL
	rows, err := db.Query(`
		SELECT namespace, key, value FROM url_metadata
		WHERE url_id = ? ORDER BY namespace, key
	`, urlID)
	if err != nil {
		t.Fatalf("failed to query metadata: %v", err)
	}
	defer rows.Close()

	expected := []struct {
		namespace string
		key       string
		value     string
	}{
		{"academic", "citation_count", "42"},
		{"academic", "doi", "10.1234/example"},
		{"domain", "has_doi", "true"},
		{"domain", "type", "academic"},
	}

	i := 0
	for rows.Next() {
		var namespace, key, value string
		if err := rows.Scan(&namespace, &key, &value); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		if i >= len(expected) {
			t.Errorf("unexpected extra row: %s.%s = %s", namespace, key, value)
			continue
		}
		exp := expected[i]
		if namespace != exp.namespace || key != exp.key || value != exp.value {
			t.Errorf("row %d = (%s, %s, %s), want (%s, %s, %s)",
				i, namespace, key, value, exp.namespace, exp.key, exp.value)
		}
		i++
	}

	if i != len(expected) {
		t.Errorf("got %d rows, want %d", i, len(expected))
	}
}

func TestQueryURLs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert multiple URLs with metadata
	url1, _ := db.InsertURL("https://arxiv.org/abs/1")
	url2, _ := db.InsertURL("https://example.com/docs")
	url3, _ := db.InsertURL("https://arxiv.org/abs/2")

	db.SetURLMetadata(url1, "domain", "type", "academic")
	db.SetURLMetadata(url2, "domain", "type", "documentation")
	db.SetURLMetadata(url3, "domain", "type", "academic")

	// Query for academic URLs
	urls, err := db.QueryURLs("domain", "type", "academic")
	if err != nil {
		t.Fatalf("QueryURLs() failed: %v", err)
	}

	if len(urls) != 2 {
		t.Errorf("got %d URLs, want 2", len(urls))
	}

	// Verify domains
	for _, u := range urls {
		if u.Domain != "arxiv.org" {
			t.Errorf("domain = %q, want %q", u.Domain, "arxiv.org")
		}
	}
}

func TestSetArtifactMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/test")
	typeID, _ := db.GetArtifactTypeID("json_parsed")
	artifactID, _ := db.InsertArtifact(urlID, typeID, "hash", "/path", 100)

	err := db.SetArtifactMetadata(artifactID, "parser_name", "html-semantic-parser")
	if err != nil {
		t.Fatalf("SetArtifactMetadata() failed: %v", err)
	}

	// Verify metadata
	var value string
	db.QueryRow("SELECT value FROM artifact_metadata WHERE artifact_id = ? AND key = ?",
		artifactID, "parser_name").Scan(&value)

	if value != "html-semantic-parser" {
		t.Errorf("value = %q, want %q", value, "html-semantic-parser")
	}
}

func TestSetArtifactMetadata_Multiple(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/test")
	typeID, _ := db.GetArtifactTypeID("json_parsed")
	artifactID, _ := db.InsertArtifact(urlID, typeID, "hash", "/path", 100)

	// Set multiple metadata keys
	metadata := map[string]string{
		"parser_name":         "html-semantic-parser",
		"parser_version":      "3.4.1",
		"language":            "en",
		"academic_confidence": "8",
		"section_count":       "12",
	}

	for key, value := range metadata {
		if err := db.SetArtifactMetadata(artifactID, key, value); err != nil {
			t.Fatalf("SetArtifactMetadata(%s) failed: %v", key, err)
		}
	}

	// Query all metadata
	rows, err := db.Query("SELECT key, value FROM artifact_metadata WHERE artifact_id = ? ORDER BY key", artifactID)
	if err != nil {
		t.Fatalf("failed to query metadata: %v", err)
	}
	defer rows.Close()

	got := make(map[string]string)
	for rows.Next() {
		var key, value string
		rows.Scan(&key, &value)
		got[key] = value
	}

	if len(got) != len(metadata) {
		t.Errorf("got %d metadata entries, want %d", len(got), len(metadata))
	}

	for k, want := range metadata {
		if got[k] != want {
			t.Errorf("metadata[%s] = %q, want %q", k, got[k], want)
		}
	}
}
