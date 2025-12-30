package mapreduce

import (
	"fmt"
	"sort"
	"strings"
)

// isValidKeyword checks if a keyword should be included in results.
// Filters malformed tokens (unmatched delimiters, trailing special chars, unmatched quotes).
// Conservative approach: only removes obviously broken tokens, keeps technical terms like x_train.
func isValidKeyword(word string) bool {
	// Remove trailing special characters (likely incomplete tokens)
	if strings.HasSuffix(word, ":") || strings.HasSuffix(word, "=") {
		return false
	}

	// Check for unmatched opening delimiters
	if strings.Contains(word, "(") && !strings.Contains(word, ")") {
		return false
	}
	if strings.Contains(word, "[") && !strings.Contains(word, "]") {
		return false
	}
	if strings.Contains(word, "{") && !strings.Contains(word, "}") {
		return false
	}

	// Check for unmatched quotes (injection/malformed strings)
	quoteCount := strings.Count(word, "\"")
	if quoteCount%2 != 0 {
		return false
	}
	singleQuoteCount := strings.Count(word, "'")
	if singleQuoteCount%2 != 0 {
		return false
	}

	return true
}

// TopKeywords returns the top N keywords from aggregated word counts as formatted strings.
// Each string is formatted as "word:count" (e.g., "learning:1153").
// Filters out malformed tokens (unmatched delimiters, trailing special chars).
func TopKeywords(wordCounts map[string]int, n int) []string {
	type kv struct {
		Key   string
		Value int
	}

	// Convert map to slice for sorting, filtering out invalid keywords
	var ss []kv
	for k, v := range wordCounts {
		if isValidKeyword(k) {
			ss = append(ss, kv{k, v})
		}
	}

	// Sort by count (descending)
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})

	// Limit to top N
	limit := n
	if len(ss) < n {
		limit = len(ss)
	}

	// Format as "word:count" strings
	keywords := make([]string, limit)
	for i := 0; i < limit; i++ {
		keywords[i] = fmt.Sprintf("%s:%d", ss[i].Key, ss[i].Value)
	}

	return keywords
}

// PrintTopKeywords prints the top N keywords in a numbered list format.
// Filters out malformed tokens (unmatched delimiters, trailing special chars).
func PrintTopKeywords(wordCounts map[string]int, n int) {
	type kv struct {
		Key   string
		Value int
	}

	var ss []kv
	for k, v := range wordCounts {
		if isValidKeyword(k) {
			ss = append(ss, kv{k, v})
		}
	}

	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})

	limit := n
	if len(ss) < n {
		limit = len(ss)
	}
	if limit < 0 {
		limit = 0
	}

	for i := 0; i < limit; i++ {
		fmt.Printf("%d. %s: %d\n", i+1, ss[i].Key, ss[i].Value)
	}
}
