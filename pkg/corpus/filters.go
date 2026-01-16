package corpus

import (
	"fmt"
	"strconv"
	"strings"
)

// FilterResult represents parsed filter components for SQL generation.
type FilterResult struct {
	WhereClause string
	Args        []interface{}
}

// ParseFilter parses a filter expression into SQL WHERE clause.
// Supported syntax:
//   - Simple: "has_code", "content_type=academic"
//   - Comparison: "citations>50", "section_count>=10"
//   - Boolean: "has_code AND citations>50", "content_type=academic OR has_abstract"
//
// Returns SQL WHERE clause and args for prepared statement.
func ParseFilter(filter string) (*FilterResult, error) {
	if filter == "" {
		return &FilterResult{WhereClause: "1=1", Args: []interface{}{}}, nil
	}

	// Split by AND/OR (simple tokenization)
	// For v1.0, we'll use simple string replacement
	// v2.0 can add proper expression parser

	filter = strings.TrimSpace(filter)

	// Handle AND/OR by splitting and building clause
	var whereParts []string
	var args []interface{}

	// Simple approach: split by AND/OR, parse each part
	if strings.Contains(strings.ToUpper(filter), " AND ") {
		parts := splitByKeyword(filter, "AND")
		for _, part := range parts {
			clause, partArgs, err := parseSimpleFilter(strings.TrimSpace(part))
			if err != nil {
				return nil, err
			}
			whereParts = append(whereParts, clause)
			args = append(args, partArgs...)
		}
		return &FilterResult{
			WhereClause: strings.Join(whereParts, " AND "),
			Args:        args,
		}, nil
	}

	if strings.Contains(strings.ToUpper(filter), " OR ") {
		parts := splitByKeyword(filter, "OR")
		for _, part := range parts {
			clause, partArgs, err := parseSimpleFilter(strings.TrimSpace(part))
			if err != nil {
				return nil, err
			}
			whereParts = append(whereParts, "("+clause+")")
			args = append(args, partArgs...)
		}
		return &FilterResult{
			WhereClause: strings.Join(whereParts, " OR "),
			Args:        args,
		}, nil
	}

	// Single filter
	clause, args, err := parseSimpleFilter(filter)
	if err != nil {
		return nil, err
	}

	return &FilterResult{
		WhereClause: clause,
		Args:        args,
	}, nil
}

// parseSimpleFilter parses a single filter expression.
// Examples: "has_code", "citations>50", "content_type=academic"
func parseSimpleFilter(filter string) (string, []interface{}, error) {
	filter = strings.TrimSpace(filter)

	// Normalize field aliases
	filter = normalizeFieldName(filter)

	// Keyword filtering (special case for top_keywords JSON field)
	if strings.HasPrefix(filter, "keyword:") {
		keyword := strings.TrimPrefix(filter, "keyword:")
		keyword = strings.TrimSpace(keyword)

		// Generate SQL LIKE query to match keyword in JSON array
		// JSON format: ["error:97","type:163","value:112",...]
		// We need to match: "<keyword>:" within the JSON string
		whereClause := "top_keywords LIKE ?"
		args := []interface{}{fmt.Sprintf("%%\"%s:%%", keyword)}
		return whereClause, args, nil
	}

	// Boolean field (just field name)
	if !strings.ContainsAny(filter, "=<>!") {
		if !isValidField(filter) {
			return "", nil, fmt.Errorf("invalid field: %s", filter)
		}
		return filter + " = 1", []interface{}{}, nil
	}

	// Comparison operators
	for _, op := range []string{">=", "<=", "!=", "=", ">", "<"} {
		if strings.Contains(filter, op) {
			parts := strings.SplitN(filter, op, 2)
			if len(parts) != 2 {
				continue
			}

			field := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if !isValidField(field) {
				return "", nil, fmt.Errorf("invalid field: %s", field)
			}

			// Parse value (number or string)
			var arg interface{}
			if num, err := strconv.Atoi(value); err == nil {
				arg = num
			} else if floatNum, err := strconv.ParseFloat(value, 64); err == nil {
				arg = floatNum
			} else {
				// String value - remove quotes if present
				value = strings.Trim(value, "\"'")
				arg = value
			}

			return field + " " + op + " ?", []interface{}{arg}, nil
		}
	}

	return "", nil, fmt.Errorf("invalid filter syntax: %s", filter)
}

// splitByKeyword splits a string by AND/OR keywords (case-insensitive).
func splitByKeyword(s, keyword string) []string {
	// Simple split - can be improved with proper tokenization
	upper := strings.ToUpper(s)
	pattern := " " + keyword + " "

	var parts []string
	remaining := s
	upperRemaining := upper

	for {
		idx := strings.Index(upperRemaining, pattern)
		if idx == -1 {
			parts = append(parts, remaining)
			break
		}

		parts = append(parts, remaining[:idx])
		remaining = remaining[idx+len(pattern):]
		upperRemaining = upperRemaining[idx+len(pattern):]
	}

	return parts
}

// isValidField checks if a field name is queryable.
var validFields = map[string]bool{
	"content_type":         true,
	"content_subtype":      true,
	"detection_confidence": true,
	"has_abstract":         true,
	"has_infobox":          true,
	"has_toc":              true,
	"has_code":             true,
	"has_code_examples":    true,
	"section_count":        true,
	"citation_count":       true,
	"code_block_count":     true,
	"domain":               true,
	"scheme":               true,
}

func isValidField(field string) bool {
	return validFields[field]
}

// normalizeFieldName normalizes field aliases to database column names.
func normalizeFieldName(filter string) string {
	// has_code → has_code_examples
	if strings.HasPrefix(filter, "has_code ") || strings.HasPrefix(filter, "has_code=") ||
		strings.HasPrefix(filter, "has_code>") || strings.HasPrefix(filter, "has_code<") ||
		strings.HasPrefix(filter, "has_code!") || filter == "has_code" {
		return strings.Replace(filter, "has_code", "has_code_examples", 1)
	}
	return filter
}
