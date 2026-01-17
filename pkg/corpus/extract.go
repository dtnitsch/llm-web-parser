package corpus

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/analytics"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
)

// KeywordCount represents a keyword with its aggregate count.
type KeywordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

// ExtractResponse is the data returned by EXTRACT verb.
type ExtractResponse struct {
	URLCount int            `json:"url_count"`
	Keywords []KeywordCount `json:"keywords"`
	TopLimit int            `json:"top_limit,omitempty"` // 0 means no limit
	Hints    *ExtractHints  `json:"hints,omitempty"`     // LLM-specific guidance
}

// ExtractHints provides contextual guidance for LLMs.
type ExtractHints struct {
	TopKeywords    []string `json:"top_keywords"`              // Top 3 keywords for quick scanning
	NextSteps      []string `json:"next_steps"`                // Suggested follow-up commands
	Interpretation string   `json:"interpretation,omitempty"`  // What the data suggests
}

// handleExtract implements the EXTRACT verb.
// Aggregates keywords from specified URLs by reading wordcount.txt files.
func handleExtract(req models.Request) models.Response {
	db, err := openDB()
	if err != nil {
		return models.Response{
			Verb:       VerbEXTRACT,
			Data:       nil,
			Confidence: 0.0,
			Coverage:   0.0,
			Unknowns:   []string{},
			Error: &models.ErrorInfo{
				Type:    "database_error",
				Message: fmt.Sprintf("Failed to open database: %v", err),
			},
		}
	}
	defer db.Close()

	// Get top limit from constraints (default 25)
	topLimit := 25
	if req.Constraints != nil {
		if topVal, ok := req.Constraints["top"].(float64); ok {
			topLimit = int(topVal)
		} else if topVal, ok := req.Constraints["top"].(int); ok {
			topLimit = topVal
		}
	}

	// Get URL IDs
	var urlIDs []int64
	if len(req.URLIDs) > 0 {
		// Explicit URL IDs provided
		urlIDs = req.URLIDs
	} else if req.Session > 0 {
		// Get all URLs from session
		sessionURLs, err := db.GetSessionURLs(int64(req.Session))
		if err != nil {
			return models.Response{
				Verb:       VerbEXTRACT,
				Data:       nil,
				Confidence: 0.0,
				Coverage:   0.0,
				Unknowns:   []string{},
				Error: &models.ErrorInfo{
					Type:    "session_error",
					Message: fmt.Sprintf("Failed to get session URLs: %v", err),
				},
			}
		}
		// Extract URL IDs from URLInfo structs
		for _, urlInfo := range sessionURLs {
			urlIDs = append(urlIDs, urlInfo.URLID)
		}
	} else {
		return models.Response{
			Verb:       VerbEXTRACT,
			Data:       nil,
			Confidence: 0.0,
			Coverage:   0.0,
			Unknowns:   []string{},
			Error: &models.ErrorInfo{
				Type:             "missing_parameter",
				Message:          "Either session or url_ids must be provided",
				SuggestedActions: []string{"Provide --session=N or --url-ids=1,2,3"},
			},
		}
	}

	// Aggregate keywords from wordcount.txt files
	aggregated, filesRead, err := aggregateKeywordsFromFiles(urlIDs)
	if err != nil {
		return models.Response{
			Verb:       VerbEXTRACT,
			Data:       nil,
			Confidence: 0.0,
			Coverage:   0.0,
			Unknowns:   []string{},
			Error: &models.ErrorInfo{
				Type:    "aggregation_error",
				Message: fmt.Sprintf("Failed to aggregate keywords: %v", err),
			},
		}
	}

	// Sort by count descending
	keywords := make([]KeywordCount, 0, len(aggregated))
	for word, count := range aggregated {
		keywords = append(keywords, KeywordCount{Word: word, Count: count})
	}
	sort.Slice(keywords, func(i, j int) bool {
		return keywords[i].Count > keywords[j].Count
	})

	// Apply top limit (0 means no limit)
	if topLimit > 0 && len(keywords) > topLimit {
		keywords = keywords[:topLimit]
	}

	// Generate LLM hints
	hints := generateExtractHints(req.Session, keywords)

	response := ExtractResponse{
		URLCount: len(urlIDs),
		Keywords: keywords,
		TopLimit: topLimit,
		Hints:    hints,
	}

	// Calculate confidence (high if we successfully read files)
	confidence := 0.95

	// Calculate coverage (what % of URLs had wordcount files)
	coverage := 0.0
	if len(urlIDs) > 0 {
		coverage = float64(filesRead) / float64(len(urlIDs))
	}

	return models.Response{
		Verb:       VerbEXTRACT,
		Data:       response,
		Confidence: confidence,
		Coverage:   coverage,
		Unknowns:   []string{},
	}
}

// aggregateKeywordsFromFiles reads wordcount.txt files and aggregates counts.
// Returns the aggregated map, count of successfully read files, and any error.
func aggregateKeywordsFromFiles(urlIDs []int64) (map[string]int, int, error) {
	aggregated := make(map[string]int)
	filesRead := 0

	for _, urlID := range urlIDs {
		wordcountPath := filepath.Join(
			artifact_manager.GetURLDir(artifact_manager.DefaultBaseDir, urlID),
			"wordcount.txt",
		)

		// Read and parse wordcount.txt
		// Path is safe: constructed from constant base dir + database ID, not user input
		file, err := os.Open(filepath.Clean(wordcountPath)) // #nosec G304
		if err != nil {
			// File might not exist for this URL (parse failure, etc.)
			// Skip silently and continue
			continue
		}

		scanner := bufio.NewScanner(file)
		fileHasData := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Parse "word:count" format
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			word := parts[0]
			count, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}

			// Normalize curly apostrophes to straight apostrophes
			// (legacy wordcount files may contain Unicode U+2019 instead of ASCII ')
			word = strings.ReplaceAll(word, "\u2019", "'")  // U+2019 (right single quote) → '
			word = strings.ReplaceAll(word, "\u2018", "'")  // U+2018 (left single quote) → '

			// Filter out stopwords (safety net for legacy wordcount files)
			if analytics.IsStopword(word) {
				continue
			}

			aggregated[word] += count
			fileHasData = true
		}

		// Close file - error ignored as we've already read the data we need
		_ = file.Close() // #nosec G104

		if err := scanner.Err(); err != nil {
			// Log error but continue with other files
			continue
		}

		if fileHasData {
			filesRead++
		}
	}

	return aggregated, filesRead, nil
}

// generateExtractHints creates LLM-specific guidance based on keywords.
func generateExtractHints(sessionID int, keywords []KeywordCount) *ExtractHints {
	if len(keywords) == 0 {
		return nil
	}

	hints := &ExtractHints{
		TopKeywords: extractTopN(keywords, 3),
		NextSteps:   generateNextSteps(sessionID, keywords),
	}

	// Add interpretation if we can infer content type
	if interpretation := inferContentType(keywords); interpretation != "" {
		hints.Interpretation = interpretation
	}

	return hints
}

// extractTopN returns the top N keyword names from the list.
func extractTopN(keywords []KeywordCount, n int) []string {
	if n > len(keywords) {
		n = len(keywords)
	}

	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = keywords[i].Word
	}
	return result
}

// generateNextSteps creates suggested follow-up commands.
func generateNextSteps(sessionID int, keywords []KeywordCount) []string {
	steps := []string{}

	// Suggest querying by top keyword if available
	if len(keywords) > 0 {
		topWord := keywords[0].Word
		if sessionID > 0 {
			steps = append(steps,
				fmt.Sprintf("lwp corpus query --session=%d --filter='keyword:%s'  # Find %s-related URLs",
					sessionID, topWord, topWord))
		}
	}

	// Suggest deep dive on specific URLs
	steps = append(steps,
		"lwp corpus extract --url-ids=<id> --top=50  # Deep dive on specific URL")

	// Suggest combined filter if multiple interesting keywords
	if len(keywords) > 1 && sessionID > 0 {
		steps = append(steps,
			fmt.Sprintf("lwp corpus query --session=%d --filter='keyword:%s AND has_code'  # Combine filters",
				sessionID, keywords[0].Word))
	}

	return steps
}

// inferContentType attempts to classify content based on keyword patterns.
func inferContentType(keywords []KeywordCount) string {
	// Build a map of top 15 keywords for quick lookup
	topWords := make(map[string]int)
	limit := 15
	if len(keywords) < limit {
		limit = len(keywords)
	}
	for i := 0; i < limit; i++ {
		topWords[keywords[i].Word] = keywords[i].Count
	}

	// Error handling / debugging content (needs both keywords)
	if hasWords(topWords, "error", "exception") || hasWords(topWords, "error", "handling") {
		return "Heavy error handling content - likely documentation or debugging guides"
	}

	// Tutorial / learning content
	if hasWords(topWords, "tutorial", "example") || hasWords(topWords, "guide", "example") {
		return "Tutorial/learning content with examples"
	}

	// API documentation
	if hasWords(topWords, "api", "endpoint") || hasWords(topWords, "method", "parameter") {
		return "API documentation or reference material"
	}

	// Programming/code focus (check for common programming keywords)
	progKeywords := []string{"function", "class", "type", "interface", "method", "variable", "object"}
	progCount := 0
	for _, kw := range progKeywords {
		if _, exists := topWords[kw]; exists {
			progCount++
		}
	}
	if progCount >= 2 {
		return "Programming-focused content - likely language documentation or reference"
	}

	// Academic / research
	if hasWords(topWords, "research", "study") || hasWords(topWords, "paper", "analysis") {
		return "Academic or research content"
	}

	return ""
}

// hasWords checks if the keyword map contains all specified words.
func hasWords(words map[string]int, targets ...string) bool {
	for _, target := range targets {
		if _, exists := words[target]; !exists {
			return false
		}
	}
	return true
}
