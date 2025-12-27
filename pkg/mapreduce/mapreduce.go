package mapreduce

import "github.com/dtnitsch/llm-web-parser/pkg/analytics"

// Map generates a word frequency map for a single document's content.
func Map(content string, a *analytics.Analytics) map[string]int {
	return a.WordFrequency(content)
}

// Reduce aggregates a slice of word frequency maps into a single map.
func Reduce(intermediate []map[string]int) map[string]int {
	finalResults := make(map[string]int)

	for _, counts := range intermediate {
		for word, count := range counts {
			finalResults[word] += count
		}
	}

	return finalResults
}
