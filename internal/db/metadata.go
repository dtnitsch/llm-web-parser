package db

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
	"gopkg.in/yaml.v3"
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

// convertToMarkdown converts a Page to markdown format
func convertToMarkdown(page *models.Page, urlID int64) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", page.Title))
	sb.WriteString(fmt.Sprintf("**URL:** %s  \n", page.URL))
	sb.WriteString(fmt.Sprintf("**URL ID:** %d\n\n", urlID))

	// Keywords if available
	if len(page.Metadata.MetaKeywords) > 0 {
		display := page.Metadata.MetaKeywords
		if len(display) > 5 {
			display = display[:5]
		}
		sb.WriteString(fmt.Sprintf("**Meta Keywords:** %s (from site)  \n", strings.Join(display, ", ")))
	}
	if len(page.Metadata.TopKeywords) > 0 {
		display := page.Metadata.TopKeywords
		if len(display) > 5 {
			display = display[:5]
		}
		sb.WriteString(fmt.Sprintf("**Top Keywords:** %s (extracted)  \n", strings.Join(display, ", ")))
	}

	if len(page.Metadata.MetaKeywords) > 0 || len(page.Metadata.TopKeywords) > 0 {
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")

	// Content blocks (FlatContent mode)
	if len(page.FlatContent) > 0 {
		for _, block := range page.FlatContent {
			switch block.Type {
			case "h1":
				sb.WriteString(fmt.Sprintf("# %s\n\n", block.Text))
			case "h2":
				sb.WriteString(fmt.Sprintf("## %s\n\n", block.Text))
			case "h3":
				sb.WriteString(fmt.Sprintf("### %s\n\n", block.Text))
			case "h4":
				sb.WriteString(fmt.Sprintf("#### %s\n\n", block.Text))
			case "h5":
				sb.WriteString(fmt.Sprintf("##### %s\n\n", block.Text))
			case "h6":
				sb.WriteString(fmt.Sprintf("###### %s\n\n", block.Text))
			case "code", "pre":
				sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", block.Text))
			case "li":
				sb.WriteString(fmt.Sprintf("- %s\n", block.Text))
			case "p":
				sb.WriteString(fmt.Sprintf("%s\n\n", block.Text))
			default:
				sb.WriteString(fmt.Sprintf("%s\n\n", block.Text))
			}

			// Add table if present
			if block.Table != nil {
				sb.WriteString(convertTableToMarkdown(block.Table))
				sb.WriteString("\n")
			}
		}
	} else {
		// Hierarchical Content mode
		var processSection func(section models.Section, level int)
		processSection = func(section models.Section, level int) {
			// Heading
			if section.Heading != nil && section.Heading.Text != "" {
				hashes := strings.Repeat("#", level)
				sb.WriteString(fmt.Sprintf("%s %s\n\n", hashes, section.Heading.Text))
			}

			// Blocks
			for _, block := range section.Blocks {
				switch block.Type {
				case "code", "pre":
					sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", block.Text))
				case "li":
					sb.WriteString(fmt.Sprintf("- %s\n", block.Text))
				case "p":
					sb.WriteString(fmt.Sprintf("%s\n\n", block.Text))
				default:
					sb.WriteString(fmt.Sprintf("%s\n\n", block.Text))
				}

				// Add table if present
				if block.Table != nil {
					sb.WriteString(convertTableToMarkdown(block.Table))
					sb.WriteString("\n")
				}
			}

			// Children sections
			for _, child := range section.Children {
				processSection(child, level+1)
			}
		}

		for _, section := range page.Content {
			processSection(section, 1)
		}
	}

	return sb.String()
}

// convertTableToMarkdown converts a table to markdown format
func convertTableToMarkdown(table *models.Table) string {
	if table == nil || len(table.Headers) == 0 {
		return ""
	}

	var sb strings.Builder

	// Headers
	sb.WriteString("| " + strings.Join(table.Headers, " | ") + " |\n")

	// Separator
	sep := make([]string, len(table.Headers))
	for i := range sep {
		sep[i] = "---"
	}
	sb.WriteString("| " + strings.Join(sep, " | ") + " |\n")

	// Rows
	for _, row := range table.Rows {
		// Pad row to match header count
		paddedRow := make([]string, len(table.Headers))
		copy(paddedRow, row)
		for i := len(row); i < len(table.Headers); i++ {
			paddedRow[i] = ""
		}
		sb.WriteString("| " + strings.Join(paddedRow, " | ") + " |\n")
	}

	return sb.String()
}

// convertToCSV converts a Page to CSV format
func convertToCSV(page *models.Page, urlID int64) string {
	var sb strings.Builder

	// CSV Header
	sb.WriteString("url_id,url,type,text,confidence\n")

	// Content blocks (FlatContent mode)
	if len(page.FlatContent) > 0 {
		for _, block := range page.FlatContent {
			// Escape quotes in text and URL
			text := strings.ReplaceAll(block.Text, "\"", "\"\"")
			url := strings.ReplaceAll(page.URL, "\"", "\"\"")

			sb.WriteString(fmt.Sprintf("%d,\"%s\",%s,\"%s\",%.2f\n",
				urlID, url, block.Type, text, block.Confidence))
		}
	} else {
		// Hierarchical Content mode - flatten sections
		var processSection func(section models.Section)
		processSection = func(section models.Section) {
			// Add heading as a block
			if section.Heading != nil && section.Heading.Text != "" {
				text := strings.ReplaceAll(section.Heading.Text, "\"", "\"\"")
				url := strings.ReplaceAll(page.URL, "\"", "\"\"")
				sb.WriteString(fmt.Sprintf("%d,\"%s\",%s,\"%s\",%.2f\n",
					urlID, url, section.Heading.Type, text, section.Heading.Confidence))
			}

			// Add blocks
			for _, block := range section.Blocks {
				text := strings.ReplaceAll(block.Text, "\"", "\"\"")
				url := strings.ReplaceAll(page.URL, "\"", "\"\"")
				sb.WriteString(fmt.Sprintf("%d,\"%s\",%s,\"%s\",%.2f\n",
					urlID, url, block.Type, text, block.Confidence))
			}

			// Process children
			for _, child := range section.Children {
				processSection(child)
			}
		}

		for _, section := range page.Content {
			processSection(section)
		}
	}

	return sb.String()
}
// displayGroupedGrep displays grep results grouped by sub-pattern when pattern contains |
func displayGroupedGrep(page *models.Page, pattern string, context int, urlID int64, outputFormat string) error {
	// Split pattern by | to get individual sub-patterns
	subPatterns := strings.Split(pattern, "|")

	// Track if any matches found across all patterns
	totalMatches := 0

	for _, subPattern := range subPatterns {
		subPattern = strings.TrimSpace(subPattern)
		if subPattern == "" {
			continue
		}

		// Filter by this sub-pattern
		filtered, err := filterByGrep(page, subPattern, context)
		if err != nil {
			return fmt.Errorf("error filtering by pattern '%s': %w", subPattern, err)
		}

		// Count matches
		matchCount := 0
		if len(filtered.FlatContent) > 0 {
			matchCount = len(filtered.FlatContent)
		} else {
			// Count blocks in hierarchical content
			var countBlocks func(sections []models.Section) int
			countBlocks = func(sections []models.Section) int {
				count := 0
				for _, section := range sections {
					count += len(section.Blocks)
					count += countBlocks(section.Children)
				}
				return count
			}
			matchCount = countBlocks(filtered.Content)
		}

		totalMatches += matchCount

		// Display header
		matchWord := "match"
		if matchCount != 1 {
			matchWord = "matches"
		}
		fmt.Printf("\n=== %s (%d %s) ===\n", subPattern, matchCount, matchWord)

		if matchCount == 0 {
			continue
		}

		// Display matches based on format
		if outputFormat == "json" {
			// JSON format
			outputStruct := struct {
				Pattern     string                 `json:"pattern"`
				Matches     int                    `json:"matches"`
				URLID       int64                  `json:"url_id"`
				URL         string                 `json:"url"`
				Title       string                 `json:"title"`
				Content     []models.Section       `json:"content,omitempty"`
				FlatContent []models.ContentBlock  `json:"flat_content,omitempty"`
			}{
				Pattern:     subPattern,
				Matches:     matchCount,
				URLID:       urlID,
				URL:         filtered.URL,
				Title:       filtered.Title,
				Content:     filtered.Content,
				FlatContent: filtered.FlatContent,
			}
			data, err := json.MarshalIndent(&outputStruct, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			// YAML format (default)
			outputStruct := struct {
				Pattern     string                 `yaml:"pattern"`
				Matches     int                    `yaml:"matches"`
				URL         string                 `yaml:"url"`
				Title       string                 `yaml:"title"`
				Content     []models.Section       `yaml:"content,omitempty"`
				FlatContent []models.ContentBlock  `yaml:"flatcontent,omitempty"`
			}{
				Pattern:     subPattern,
				Matches:     matchCount,
				URL:         filtered.URL,
				Title:       filtered.Title,
				Content:     filtered.Content,
				FlatContent: filtered.FlatContent,
			}
			data, err := yaml.Marshal(&outputStruct)
			if err != nil {
				return fmt.Errorf("failed to marshal YAML: %w", err)
			}
			fmt.Print(string(data))
		}
	}

	// Summary at the end
	fmt.Printf("\n---\nTotal: %d matches across %d patterns\n", totalMatches, len(subPatterns))

	return nil
}
