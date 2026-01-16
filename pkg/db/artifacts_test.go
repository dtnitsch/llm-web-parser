package db

import (
	"testing"
)

func TestInsertArtifact(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert URL first
	urlID, err := db.InsertURL("https://example.com/test")
	if err != nil {
		t.Fatalf("InsertURL() failed: %v", err)
	}

	// Get artifact type ID
	typeID, err := db.GetArtifactTypeID("html_raw")
	if err != nil {
		t.Fatalf("GetArtifactTypeID() failed: %v", err)
	}

	// Insert artifact
	hash := "abc123def456"
	path := "/path/to/artifact.html"
	size := int64(1024)

	artifactID, err := db.InsertArtifact(urlID, typeID, hash, path, size)
	if err != nil {
		t.Fatalf("InsertArtifact() failed: %v", err)
	}

	if artifactID == 0 {
		t.Error("InsertArtifact() returned 0 ID")
	}

	// Verify artifact was inserted
	var gotHash, gotPath string
	var gotSize int64
	err = db.QueryRow(`
		SELECT content_hash, file_path, size_bytes
		FROM artifacts WHERE artifact_id = ?
	`, artifactID).Scan(&gotHash, &gotPath, &gotSize)
	if err != nil {
		t.Fatalf("failed to query artifact: %v", err)
	}

	if gotHash != hash {
		t.Errorf("content_hash = %q, want %q", gotHash, hash)
	}
	if gotPath != path {
		t.Errorf("file_path = %q, want %q", gotPath, path)
	}
	if gotSize != size {
		t.Errorf("size_bytes = %d, want %d", gotSize, size)
	}
}

func TestInsertArtifact_UpdatesExisting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/test")
	typeID, _ := db.GetArtifactTypeID("html_raw")

	// Insert first time
	artifactID1, err := db.InsertArtifact(urlID, typeID, "hash1", "/path1", 100)
	if err != nil {
		t.Fatalf("InsertArtifact() failed: %v", err)
	}

	// Insert again with same URL and type (should update)
	artifactID2, err := db.InsertArtifact(urlID, typeID, "hash2", "/path2", 200)
	if err != nil {
		t.Fatalf("InsertArtifact() update failed: %v", err)
	}

	if artifactID1 != artifactID2 {
		t.Errorf("got different artifact ID on update: %d vs %d", artifactID1, artifactID2)
	}

	// Verify updated values
	var gotHash string
	var gotSize int64
	db.QueryRow("SELECT content_hash, size_bytes FROM artifacts WHERE artifact_id = ?", artifactID1).Scan(&gotHash, &gotSize)

	if gotHash != "hash2" {
		t.Errorf("hash not updated: got %q, want %q", gotHash, "hash2")
	}
	if gotSize != 200 {
		t.Errorf("size not updated: got %d, want %d", gotSize, 200)
	}
}

func TestGetArtifactTypeID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name     string
		typeName string
		wantErr  bool
	}{
		{"html_raw", "html_raw", false},
		{"json_parsed", "json_parsed", false},
		{"keywords", "keywords", false},
		{"wordcount", "wordcount", false},
		{"links", "links", false},
		{"images", "images", false},
		{"metadata", "metadata", false},
		{"nonexistent", "nonexistent_type", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeID, err := db.GetArtifactTypeID(tt.typeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetArtifactTypeID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && typeID == 0 {
				t.Error("GetArtifactTypeID() returned 0 for valid type")
			}
		})
	}
}

func TestListArtifacts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert URL
	urlID, _ := db.InsertURL("https://example.com/page")

	// Insert multiple artifacts
	rawTypeID, _ := db.GetArtifactTypeID("html_raw")
	parsedTypeID, _ := db.GetArtifactTypeID("json_parsed")
	keywordsTypeID, _ := db.GetArtifactTypeID("keywords")

	db.InsertArtifact(urlID, rawTypeID, "hash1", "/raw/page.html", 1000)
	db.InsertArtifact(urlID, parsedTypeID, "hash2", "/parsed/page.json", 500)
	db.InsertArtifact(urlID, keywordsTypeID, "hash3", "/keywords/page.txt", 200)

	// List artifacts
	artifacts, err := db.ListArtifacts(urlID)
	if err != nil {
		t.Fatalf("ListArtifacts() failed: %v", err)
	}

	if len(artifacts) != 3 {
		t.Errorf("got %d artifacts, want 3", len(artifacts))
	}

	// Verify types are correct and sorted
	expectedTypes := []string{"html_raw", "json_parsed", "keywords"}
	for i, artifact := range artifacts {
		if artifact.TypeName != expectedTypes[i] {
			t.Errorf("artifact[%d].TypeName = %q, want %q", i, artifact.TypeName, expectedTypes[i])
		}
	}
}

func TestGetArtifactPath(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	urlID, _ := db.InsertURL("https://example.com/test")
	typeID, _ := db.GetArtifactTypeID("html_raw")
	wantPath := "/raw/test.html"

	db.InsertArtifact(urlID, typeID, "hash", wantPath, 100)

	gotPath, err := db.GetArtifactPath(urlID, "html_raw")
	if err != nil {
		t.Fatalf("GetArtifactPath() failed: %v", err)
	}

	if gotPath != wantPath {
		t.Errorf("GetArtifactPath() = %q, want %q", gotPath, wantPath)
	}

	// Test non-existent artifact
	_, err = db.GetArtifactPath(urlID, "nonexistent_type")
	if err == nil {
		t.Error("GetArtifactPath() should return error for non-existent type")
	}
}
