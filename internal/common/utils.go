package common

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// fieldNameMap maps verbose field names to terse equivalents.
var fieldNameMap = map[string]string{
	"url":                     "u",
	"file_path":               "p",
	"status":                  "s",
	"error":                   "e",
	"file_size_bytes":         "sz",
	"estimated_tokens":        "tk",
	"content_type":            "ct",
	"extraction_quality":      "q",
	"confidence_distribution": "cd",
	"block_type_distribution": "bd",
}

func FilterResultFields(result interface{}, fieldsStr string, isTerse bool) map[string]interface{} {
	if fieldsStr == "" {
		// No filtering, convert to map and return all fields
		return structToMap(result)
	}

	requestedFields := strings.Split(fieldsStr, ",")
	for i := range requestedFields {
		requestedFields[i] = strings.TrimSpace(requestedFields[i])
	}

	// Build set of fields to include (translate verbose->terse if needed)
	includeFields := make(map[string]bool)
	for _, field := range requestedFields {
		if isTerse {
			// If terse mode, check if user provided verbose name and translate
			if terseField, ok := fieldNameMap[field]; ok {
				includeFields[terseField] = true
			} else {
				// User already provided terse name
				includeFields[field] = true
			}
		} else {
			includeFields[field] = true
		}
	}

	// Convert struct to map
	fullMap := structToMap(result)

	// Filter map
	filtered := make(map[string]interface{})
	for key, value := range fullMap {
		if includeFields[key] {
			filtered[key] = value
		}
	}

	return filtered
}

// structToMap converts a struct to map[string]interface{} using JSON marshaling.
func structToMap(obj interface{}) map[string]interface{} {
	data, _ := json.Marshal(obj)
	var result map[string]interface{}
	_ = json.Unmarshal(data, &result)
	return result
}

// contentHash computes SHA256 hash of content and returns hex string.
func ContentHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// sanitizeURL performs basic cleanup on URLs to handle common copy-paste issues.
// Removes whitespace, trailing punctuation, markdown artifacts, and encodes spaces.
func SanitizeURL(rawURL string) string {
	// Trim all whitespace from edges
	cleaned := strings.TrimSpace(rawURL)

	// Extract URL from markdown link format: [text](url) -> url
	// Example: "[click here](https://example.com)" -> "https://example.com"
	markdownLinkPattern := regexp.MustCompile(`^\[.*?\]\((https?://[^\)]+)\)$`)
	if matches := markdownLinkPattern.FindStringSubmatch(cleaned); len(matches) > 1 {
		cleaned = matches[1]
	}

	// Remove common trailing punctuation from copy-paste errors
	// Example: "https://example.com," -> "https://example.com"
	trailingChars := []string{",", ".", ")", "}", "]", "\"", "'", ">", ";"}
	for _, char := range trailingChars {
		cleaned = strings.TrimSuffix(cleaned, char)
	}

	// Remove leading markdown/formatting artifacts
	// Example: "(https://example.com)" -> "https://example.com"
	leadingChars := []string{"(", "[", "<", "\"", "'"}
	for _, char := range leadingChars {
		cleaned = strings.TrimPrefix(cleaned, char)
	}

	// Trim again after removing punctuation (in case there was whitespace before punctuation)
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// sanitizeAndValidateURLs sanitizes all URLs and returns (sanitized URLs, invalid URLs).
// Invalid URLs are those that fail validation even after sanitization.
func SanitizeAndValidateURLs(urls []string) ([]string, []string) {
	sanitized := make([]string, 0, len(urls))
	var invalidURLs []string

	// Regex pattern for valid URLs
	// Must start with http:// or https://
	// Must have a valid domain (alphanumeric, dots, hyphens)
	// Can have path, query, fragment
	urlPattern := regexp.MustCompile(`^https?://[a-zA-Z0-9][-a-zA-Z0-9.]*[a-zA-Z0-9](/[^\s]*)?$`)

	for _, rawURL := range urls {
		// Sanitize first
		cleaned := SanitizeURL(rawURL)

		// Empty URLs after sanitization are invalid
		if cleaned == "" {
			invalidURLs = append(invalidURLs, rawURL)
			continue
		}

		// Reject URLs with literal spaces (must be pre-encoded as %20)
		if strings.Contains(cleaned, " ") {
			invalidURLs = append(invalidURLs, rawURL)
			continue
		}

		// Check basic pattern
		if !urlPattern.MatchString(cleaned) {
			invalidURLs = append(invalidURLs, rawURL)
			continue
		}

		// Use net/url to validate structure
		parsed, err := url.Parse(cleaned)
		if err != nil {
			invalidURLs = append(invalidURLs, rawURL)
			continue
		}

		// Ensure scheme is http or https
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			invalidURLs = append(invalidURLs, rawURL)
			continue
		}

		// Ensure host is not empty
		if parsed.Host == "" {
			invalidURLs = append(invalidURLs, rawURL)
			continue
		}

		// Check for suspicious characters in domain that indicate malformed URL
		// Example: "https://example.com{}" should fail
		if strings.ContainsAny(parsed.Host, "{}[]<>\"'") {
			invalidURLs = append(invalidURLs, rawURL)
			continue
		}

		// URL is valid, add sanitized version
		sanitized = append(sanitized, cleaned)
	}

	return sanitized, invalidURLs
}
