package db

import (
	"fmt"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
)

// countTruthyMetadata counts non-empty/non-zero/non-false metadata fields
func countTruthyMetadata(meta models.PageMetadata) int {
	count := 0

	// String fields
	if meta.ContentType != "" && meta.ContentType != "unknown" { count++ }
	if meta.ContentSubtype != "" { count++ }
	if meta.Language != "" { count++ }
	if meta.SiteName != "" { count++ }
	if meta.Author != "" { count++ }
	if meta.Excerpt != "" { count++ }
	if meta.PublishedTime != "" { count++ }
	if meta.DomainType != "" && meta.DomainType != "unknown" { count++ }
	if meta.DomainCategory != "" { count++ }
	if meta.Country != "" && meta.Country != "unknown" { count++ }
	if meta.ExtractionMode != "" { count++ }
	if meta.ExtractionQuality != "" { count++ }
	if meta.Favicon != "" { count++ }
	if meta.Image != "" { count++ }
	if meta.DOIPattern != "" { count++ }
	if meta.ArXivID != "" { count++ }
	if meta.HTTPContentType != "" { count++ }
	if meta.FinalURL != "" { count++ }

	// Numeric fields (count if > 0)
	if meta.WordCount > 0 { count++ }
	if meta.EstimatedReadMin > 0 { count++ }
	if meta.SectionCount > 0 { count++ }
	if meta.BlockCount > 0 { count++ }
	if meta.CitationCount > 0 { count++ }
	if meta.CodeBlockCount > 0 { count++ }
	if meta.LanguageConfidence > 0 { count++ }
	if meta.Confidence > 0 { count++ }
	if meta.AcademicScore > 0 { count++ }
	if meta.StatusCode > 0 { count++ }

	// Boolean fields (count if true)
	if meta.HasInfobox { count++ }
	if meta.HasTOC { count++ }
	if meta.HasCodeExamples { count++ }
	if meta.Computed { count++ }
	if meta.HasDOI { count++ }
	if meta.HasArXiv { count++ }
	if meta.HasLaTeX { count++ }
	if meta.HasCitations { count++ }
	if meta.HasReferences { count++ }
	if meta.HasAbstract { count++ }

	// Slice fields
	if len(meta.RedirectChain) > 0 { count++ }

	return count
}

// getCompactMetadata returns only the 5 essential metadata fields (if truthy)
func getCompactMetadata(meta models.PageMetadata) map[string]interface{} {
	compact := make(map[string]interface{})

	if meta.SiteName != "" {
		compact["site_name"] = meta.SiteName
	}
	if meta.ContentType != "" && meta.ContentType != "unknown" {
		compact["content_type"] = meta.ContentType
	}
	if meta.Language != "" {
		compact["language"] = meta.Language
	}
	if meta.WordCount > 0 {
		compact["word_count"] = meta.WordCount
	}
	if meta.HasCodeExamples {
		compact["has_code_examples"] = true
	}

	return compact
}

// getContentTypeCounts returns a map of content type counts from the page
func getContentTypeCounts(page *models.Page) map[string]int {
	typeCounts := make(map[string]int)

	if len(page.FlatContent) > 0 {
		// Cheap mode: count from flat array
		for _, block := range page.FlatContent {
			typeCounts[block.Type]++
		}
	} else {
		// Full-parse mode: count from hierarchical sections
		var countTypes func(sections []models.Section)
		countTypes = func(sections []models.Section) {
			for _, section := range sections {
				for _, block := range section.Blocks {
					typeCounts[block.Type]++
				}
				if len(section.Children) > 0 {
					countTypes(section.Children)
				}
			}
		}
		countTypes(page.Content)
	}

	return typeCounts
}

// generateNoResultsMessage creates a helpful message when filters return no results
func generateNoResultsMessage(filterType string, filterValue string, page *models.Page, urlID int64) string {
	var sb strings.Builder

	// Header message
	if filterType == "only" {
		types := strings.Split(filterValue, ",")
		if len(types) == 1 {
			sb.WriteString(fmt.Sprintf("No '%s' content found in this document.\n\n", strings.TrimSpace(types[0])))
		} else {
			sb.WriteString(fmt.Sprintf("No '%s' content found in this document.\n\n", filterValue))
		}
	} else if filterType == "grep" {
		sb.WriteString(fmt.Sprintf("No matches found for pattern: %s\n\n", filterValue))
	}

	// Show available content types
	typeCounts := getContentTypeCounts(page)
	if len(typeCounts) > 0 {
		sb.WriteString(fmt.Sprintf("Available content types (%d total):\n", len(typeCounts)))

		// Sort types by count (descending)
		type typeCount struct {
			name  string
			count int
		}
		var sorted []typeCount
		for t, c := range typeCounts {
			sorted = append(sorted, typeCount{t, c})
		}
		// Simple bubble sort
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].count > sorted[i].count {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		// Show top types
		for i, tc := range sorted {
			if i < 10 { // Show top 10
				sb.WriteString(fmt.Sprintf("  %s: %d\n", tc.name, tc.count))
			}
		}
		sb.WriteString("\n")
	}

	// Smart suggestions
	sb.WriteString("Suggestions:\n")
	if len(typeCounts) > 0 {
		// Suggest top 2 content types
		suggestions := []string{}
		if typeCounts["p"] > 0 {
			suggestions = append(suggestions, "p")
		}
		if typeCounts["h2"] > 0 {
			suggestions = append(suggestions, "h2")
		}
		if len(suggestions) > 0 {
			sb.WriteString(fmt.Sprintf("  llm-web-parser db show --only=%s %d  # Most common types\n",
				strings.Join(suggestions, ","), urlID))
		}

		// Suggest code if available
		if typeCounts["code"] > 0 || typeCounts["pre"] > 0 {
			codeTypes := []string{}
			if typeCounts["code"] > 0 {
				codeTypes = append(codeTypes, "code")
			}
			if typeCounts["pre"] > 0 {
				codeTypes = append(codeTypes, "pre")
			}
			sb.WriteString(fmt.Sprintf("  llm-web-parser db show --only=%s %d  # Code blocks\n",
				strings.Join(codeTypes, ","), urlID))
		}
	}

	// Always suggest outline
	sb.WriteString(fmt.Sprintf("  llm-web-parser db show --outline %d  # Document structure\n", urlID))

	return sb.String()
}
