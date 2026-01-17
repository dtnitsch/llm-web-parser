package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// InsertURL parses and inserts a URL, returning the url_id.
// If the URL already exists, returns the existing url_id.
func (db *DB) InsertURL(rawURL string) (int64, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check if URL already exists
	var existingID int64
	err = db.QueryRow("SELECT url_id FROM urls WHERE original_url = ?", rawURL).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to check existing URL: %w", err)
	}

	// Extract canonical URL (scheme + host + path, no query/fragment)
	canonicalURL := fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host, parsed.Path)

	// Insert URL
	result, err := db.Exec(`
		INSERT INTO urls (original_url, canonical_url, scheme, domain, path, fragment)
		VALUES (?, ?, ?, ?, ?, ?)
	`, rawURL, canonicalURL, parsed.Scheme, parsed.Host, parsed.Path, parsed.Fragment)
	if err != nil {
		return 0, fmt.Errorf("failed to insert URL: %w", err)
	}

	urlID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get URL ID: %w", err)
	}

	// Insert query params if present
	if parsed.RawQuery != "" {
		params, err := url.ParseQuery(parsed.RawQuery)
		if err == nil {
			for key, values := range params {
				for _, value := range values {
					_, err = db.Exec(`
						INSERT INTO url_query_params (url_id, key, value)
						VALUES (?, ?, ?)
					`, urlID, key, value)
					if err != nil {
						return 0, fmt.Errorf("failed to insert query param: %w", err)
					}
				}
			}
		}
	}

	return urlID, nil
}

// RecordAccess records a fetch attempt in url_accesses.
func (db *DB) RecordAccess(urlID int64, statusCode int, errorType string, success bool) error {
	_, err := db.Exec(`
		INSERT INTO url_accesses (url_id, status_code, error_type, success)
		VALUES (?, ?, ?, ?)
	`, urlID, statusCode, errorType, success)
	if err != nil {
		return fmt.Errorf("failed to record access: %w", err)
	}
	return nil
}

// InsertArtifact inserts or updates an artifact, returning the artifact_id.
func (db *DB) InsertArtifact(urlID int64, typeID int64, contentHash, filePath string, sizeBytes int64) (int64, error) {
	// Check if artifact already exists for this URL and type
	var existingID int64
	err := db.QueryRow("SELECT artifact_id FROM artifacts WHERE url_id = ? AND type_id = ?", urlID, typeID).Scan(&existingID)
	if err == nil {
		// Update existing artifact
		_, err = db.Exec(`
			UPDATE artifacts
			SET content_hash = ?, file_path = ?, size_bytes = ?
			WHERE artifact_id = ?
		`, contentHash, filePath, sizeBytes, existingID)
		if err != nil {
			return 0, fmt.Errorf("failed to update artifact: %w", err)
		}
		return existingID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to check existing artifact: %w", err)
	}

	// Insert new artifact
	result, err := db.Exec(`
		INSERT INTO artifacts (url_id, type_id, content_hash, file_path, size_bytes)
		VALUES (?, ?, ?, ?, ?)
	`, urlID, typeID, contentHash, filePath, sizeBytes)
	if err != nil {
		return 0, fmt.Errorf("failed to insert artifact: %w", err)
	}

	artifactID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get artifact ID: %w", err)
	}

	return artifactID, nil
}

// SetURLMetadata sets a metadata key-value pair for a URL (upsert).
func (db *DB) SetURLMetadata(urlID int64, namespace, key, value string) error {
	_, err := db.Exec(`
		INSERT INTO url_metadata (url_id, namespace, key, value)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(url_id, namespace, key) DO UPDATE SET value = excluded.value
	`, urlID, namespace, key, value)
	if err != nil {
		return fmt.Errorf("failed to set URL metadata: %w", err)
	}
	return nil
}

// SetArtifactMetadata sets a metadata key-value pair for an artifact (upsert).
func (db *DB) SetArtifactMetadata(artifactID int64, key, value string) error {
	_, err := db.Exec(`
		INSERT INTO artifact_metadata (artifact_id, key, value)
		VALUES (?, ?, ?)
		ON CONFLICT(artifact_id, key) DO UPDATE SET value = excluded.value
	`, artifactID, key, value)
	if err != nil {
		return fmt.Errorf("failed to set artifact metadata: %w", err)
	}
	return nil
}

// GetArtifactTypeID returns the type_id for a given type_name.
func (db *DB) GetArtifactTypeID(typeName string) (int64, error) {
	var typeID int64
	err := db.QueryRow("SELECT type_id FROM artifact_types WHERE type_name = ?", typeName).Scan(&typeID)
	if err != nil {
		return 0, fmt.Errorf("failed to get artifact type ID for %s: %w", typeName, err)
	}
	return typeID, nil
}

// GetLastAccess returns the most recent access record for a URL.
func (db *DB) GetLastAccess(urlID int64) (*AccessRecord, error) {
	var record AccessRecord
	err := db.QueryRow(`
		SELECT access_id, accessed_at, status_code, error_type, success
		FROM url_accesses
		WHERE url_id = ?
		ORDER BY accessed_at DESC
		LIMIT 1
	`, urlID).Scan(&record.AccessID, &record.AccessedAt, &record.StatusCode, &record.ErrorType, &record.Success)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last access: %w", err)
	}
	return &record, nil
}

// AccessRecord represents a URL access attempt.
type AccessRecord struct {
	AccessID   int64
	AccessedAt time.Time
	StatusCode int
	ErrorType  string
	Success    bool
}

// GetArtifactPath returns the file path for a specific artifact.
func (db *DB) GetArtifactPath(urlID int64, typeName string) (string, error) {
	var filePath string
	err := db.QueryRow(`
		SELECT a.file_path
		FROM artifacts a
		JOIN artifact_types t ON a.type_id = t.type_id
		WHERE a.url_id = ? AND t.type_name = ?
	`, urlID, typeName).Scan(&filePath)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("artifact not found for URL and type %s", typeName)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get artifact path: %w", err)
	}
	return filePath, nil
}

// GetURLID returns the url_id for a given original URL.
func (db *DB) GetURLID(originalURL string) (int64, error) {
	var urlID int64
	err := db.QueryRow("SELECT url_id FROM urls WHERE original_url = ?", originalURL).Scan(&urlID)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("URL not found: %s", originalURL)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get URL ID: %w", err)
	}
	return urlID, nil
}

// ListArtifacts returns all artifacts for a given URL.
func (db *DB) ListArtifacts(urlID int64) ([]ArtifactInfo, error) {
	rows, err := db.Query(`
		SELECT a.artifact_id, t.type_name, a.content_hash, a.file_path, a.size_bytes, a.created_at
		FROM artifacts a
		JOIN artifact_types t ON a.type_id = t.type_id
		WHERE a.url_id = ?
		ORDER BY t.type_name
	`, urlID)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []ArtifactInfo
	for rows.Next() {
		var artifact ArtifactInfo
		err := rows.Scan(&artifact.ArtifactID, &artifact.TypeName, &artifact.ContentHash, &artifact.FilePath, &artifact.SizeBytes, &artifact.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan artifact: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

// ArtifactInfo represents artifact metadata.
type ArtifactInfo struct {
	ArtifactID  int64
	TypeName    string
	ContentHash string
	FilePath    string
	SizeBytes   int64
	CreatedAt   time.Time
}

// QueryURLs returns URLs matching metadata criteria.
// Example: db.QueryURLs("domain", "has_doi", "true")
func (db *DB) QueryURLs(namespace, key, value string) ([]URLInfo, error) {
	query := `
		SELECT u.url_id, u.original_url, u.canonical_url, u.domain
		FROM urls u
		JOIN url_metadata m ON u.url_id = m.url_id
		WHERE m.namespace = ? AND m.key = ? AND m.value = ?
		ORDER BY u.created_at DESC
	`

	rows, err := db.Query(query, namespace, key, value)
	if err != nil {
		return nil, fmt.Errorf("failed to query URLs: %w", err)
	}
	defer rows.Close()

	var urls []URLInfo
	for rows.Next() {
		var info URLInfo
		err := rows.Scan(&info.URLID, &info.OriginalURL, &info.CanonicalURL, &info.Domain)
		if err != nil {
			return nil, fmt.Errorf("failed to scan URL: %w", err)
		}
		urls = append(urls, info)
	}

	return urls, nil
}

// URLInfo represents basic URL information.
type URLInfo struct {
	URLID        int64
	OriginalURL  string
	CanonicalURL sql.NullString
	Domain       string
}

// ContentTypeInfo represents content type classification and features.
type ContentTypeInfo struct {
	ContentType         sql.NullString
	ContentSubtype      sql.NullString
	DetectionConfidence sql.NullFloat64
	HasAbstract         bool
	HasInfobox          bool
	HasTOC              bool
	HasCodeExamples     bool
	SectionCount        int
	CitationCount       int
	CodeBlockCount      int
	TopKeywords         sql.NullString // JSON object
}

// UpdateURLContentType updates content type classification for a URL.
func (db *DB) UpdateURLContentType(urlID int64, info ContentTypeInfo) error {
	_, err := db.Exec(`
		UPDATE urls SET
			content_type = ?,
			content_subtype = ?,
			detection_confidence = ?,
			has_abstract = ?,
			has_infobox = ?,
			has_toc = ?,
			has_code_examples = ?,
			section_count = ?,
			citation_count = ?,
			code_block_count = ?,
			top_keywords = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE url_id = ?
	`, info.ContentType, info.ContentSubtype, info.DetectionConfidence,
		info.HasAbstract, info.HasInfobox, info.HasTOC, info.HasCodeExamples,
		info.SectionCount, info.CitationCount, info.CodeBlockCount,
		info.TopKeywords, urlID)
	if err != nil {
		return fmt.Errorf("failed to update content type: %w", err)
	}
	return nil
}

// GetURLContentInfo retrieves content type information for a URL.
func (db *DB) GetURLContentInfo(urlID int64) (*ContentTypeInfo, error) {
	var info ContentTypeInfo
	err := db.QueryRow(`
		SELECT content_type, content_subtype, detection_confidence,
			has_abstract, has_infobox, has_toc, has_code_examples,
			section_count, citation_count, code_block_count, top_keywords
		FROM urls
		WHERE url_id = ?
	`, urlID).Scan(
		&info.ContentType, &info.ContentSubtype, &info.DetectionConfidence,
		&info.HasAbstract, &info.HasInfobox, &info.HasTOC, &info.HasCodeExamples,
		&info.SectionCount, &info.CitationCount, &info.CodeBlockCount,
		&info.TopKeywords,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("URL not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get content info: %w", err)
	}
	return &info, nil
}

// GetURLsByContentType queries URLs by content type with optional filters.
func (db *DB) GetURLsByContentType(contentType string, hasAbstract, hasCode *bool) ([]URLInfo, error) {
	query := `SELECT url_id, original_url, canonical_url, domain FROM urls WHERE 1=1`
	args := []interface{}{}

	if contentType != "" {
		query += " AND content_type = ?"
		args = append(args, contentType)
	}
	if hasAbstract != nil {
		query += " AND has_abstract = ?"
		args = append(args, *hasAbstract)
	}
	if hasCode != nil {
		query += " AND has_code_examples = ?"
		args = append(args, *hasCode)
	}
	query += " ORDER BY detection_confidence DESC, created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query URLs by content type: %w", err)
	}
	defer rows.Close()

	var urls []URLInfo
	for rows.Next() {
		var info URLInfo
		err := rows.Scan(&info.URLID, &info.OriginalURL, &info.CanonicalURL, &info.Domain)
		if err != nil {
			return nil, fmt.Errorf("failed to scan URL: %w", err)
		}
		urls = append(urls, info)
	}

	return urls, nil
}

// NewNullString creates a sql.NullString from a string value.
func NewNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// NewNullFloat64 creates a sql.NullFloat64 from a float64 value.
func NewNullFloat64(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}
