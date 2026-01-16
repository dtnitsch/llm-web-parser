package corpus

import (
	"database/sql"
	"fmt"

	"github.com/dtnitsch/llm-web-parser/models"
	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
)

// QueryResult represents a single URL matching the query.
type QueryResult struct {
	URLID               int64   `json:"url_id"`
	OriginalURL         string  `json:"original_url"`
	Domain              string  `json:"domain"`
	ContentType         string  `json:"content_type,omitempty"`
	ContentSubtype      string  `json:"content_subtype,omitempty"`
	DetectionConfidence float64 `json:"detection_confidence,omitempty"`
	HasAbstract         bool    `json:"has_abstract,omitempty"`
	HasInfobox          bool    `json:"has_infobox,omitempty"`
	HasTOC              bool    `json:"has_toc,omitempty"`
	HasCodeExamples     bool    `json:"has_code_examples,omitempty"`
	SectionCount        int     `json:"section_count,omitempty"`
	CitationCount       int     `json:"citation_count,omitempty"`
	CodeBlockCount      int     `json:"code_block_count,omitempty"`
}

// QueryResponse is the data returned by QUERY verb.
type QueryResponse struct {
	Filter       string        `json:"filter"`
	MatchCount   int           `json:"match_count"`
	TotalCount   int           `json:"total_count"`
	Matches      []QueryResult `json:"matches"`
	WhereClause  string        `json:"where_clause,omitempty"` // For debugging
}

// ExecuteQuery runs a metadata query against the database.
func ExecuteQuery(db *dbpkg.DB, filter string, session int) (models.Response, error) {
	// Parse filter
	filterResult, err := ParseFilter(filter)
	if err != nil {
		return models.Response{
			Verb:       VerbQUERY,
			Data:       nil,
			Confidence: 0.0,
			Coverage:   0.0,
			Unknowns:   []string{},
			Error: &models.ErrorInfo{
				Type:             "filter_parse_error",
				Message:          fmt.Sprintf("Failed to parse filter: %v", err),
				SuggestedActions: []string{"Check filter syntax", "See docs/CORPUS-API.md for examples"},
			},
		}, nil
	}

	// Build query
	baseQuery := "SELECT url_id, original_url, domain, content_type, content_subtype, detection_confidence, has_abstract, has_infobox, has_toc, has_code_examples, section_count, citation_count, code_block_count FROM urls"

	var whereClause string
	var args []interface{}

	// Add session filter if specified
	if session > 0 {
		// Join with session_urls to filter by session
		baseQuery = `
			SELECT DISTINCT u.url_id, u.original_url, u.domain, u.content_type, u.content_subtype,
			       u.detection_confidence, u.has_abstract, u.has_infobox, u.has_toc, u.has_code_examples,
			       u.section_count, u.citation_count, u.code_block_count
			FROM urls u
			JOIN session_urls su ON u.url_id = su.url_id
			WHERE su.session_id = ?`
		args = append(args, session)

		if filterResult.WhereClause != "1=1" {
			whereClause = " AND (" + filterResult.WhereClause + ")"
			args = append(args, filterResult.Args...)
		}
	} else {
		whereClause = " WHERE " + filterResult.WhereClause
		args = filterResult.Args
	}

	query := baseQuery + whereClause

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		return models.Response{}, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Scan results
	var matches []QueryResult
	for rows.Next() {
		var m QueryResult
		var contentType, contentSubtype sql.NullString
		var detectionConfidence sql.NullFloat64

		err := rows.Scan(
			&m.URLID,
			&m.OriginalURL,
			&m.Domain,
			&contentType,
			&contentSubtype,
			&detectionConfidence,
			&m.HasAbstract,
			&m.HasInfobox,
			&m.HasTOC,
			&m.HasCodeExamples,
			&m.SectionCount,
			&m.CitationCount,
			&m.CodeBlockCount,
		)
		if err != nil {
			return models.Response{}, fmt.Errorf("row scan failed: %w", err)
		}

		if contentType.Valid {
			m.ContentType = contentType.String
		}
		if contentSubtype.Valid {
			m.ContentSubtype = contentSubtype.String
		}
		if detectionConfidence.Valid {
			m.DetectionConfidence = detectionConfidence.Float64
		}

		matches = append(matches, m)
	}

	// Get total count for coverage calculation
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM urls"
	if session > 0 {
		countQuery = "SELECT COUNT(DISTINCT url_id) FROM session_urls WHERE session_id = ?"
		err = db.QueryRow(countQuery, session).Scan(&totalCount)
	} else {
		err = db.QueryRow(countQuery).Scan(&totalCount)
	}
	if err != nil {
		totalCount = 0 // Non-fatal
	}

	// Calculate confidence and coverage
	confidence := calculateConfidence(filterResult.WhereClause)
	coverage := 0.0
	if totalCount > 0 {
		coverage = float64(len(matches)) / float64(totalCount)
	}

	// Build response
	responseData := QueryResponse{
		Filter:      filter,
		MatchCount:  len(matches),
		TotalCount:  totalCount,
		Matches:     matches,
		WhereClause: filterResult.WhereClause, // For debugging
	}

	return models.Response{
		Verb:       VerbQUERY,
		Data:       responseData,
		Confidence: confidence,
		Coverage:   coverage,
		Unknowns:   []string{},
	}, nil
}

// calculateConfidence estimates confidence based on filter complexity.
// Simple filters = high confidence, complex filters = lower confidence.
func calculateConfidence(whereClause string) float64 {
	// Simple heuristic:
	// - Exact matches (=) are high confidence (0.95)
	// - Range queries (>, <) are medium confidence (0.85)
	// - Complex queries (AND/OR) reduce confidence slightly

	confidence := 0.95

	// Reduce confidence for comparison operators
	if hasAny(whereClause, ">", "<", ">=", "<=") {
		confidence -= 0.05
	}

	// Reduce confidence for boolean logic
	andCount := countOccurrences(whereClause, " AND ")
	orCount := countOccurrences(whereClause, " OR ")
	confidence -= float64(andCount) * 0.03
	confidence -= float64(orCount) * 0.05

	if confidence < 0.6 {
		confidence = 0.6
	}

	return confidence
}

func hasAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
			i += len(substr) - 1
		}
	}
	return count
}
