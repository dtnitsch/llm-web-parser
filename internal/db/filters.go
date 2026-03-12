package db

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
)

// filterOutline extracts headings from a Page and builds a hierarchical outline.
func filterOutline(page *models.Page, urlID int64) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("url_id: %d\n", urlID))
	sb.WriteString(fmt.Sprintf("url: %s\n", page.URL))
	sb.WriteString(fmt.Sprintf("title: %s\n", page.Title))

	// Show keywords from metadata (both are in the YAML artifact)
	if len(page.Metadata.MetaKeywords) > 0 {
		// Show top 5 meta keywords
		display := page.Metadata.MetaKeywords
		if len(display) > 5 {
			display = display[:5]
		}
		sb.WriteString(fmt.Sprintf("meta_keywords: %s (from site)\n", strings.Join(display, ", ")))
	}

	if len(page.Metadata.TopKeywords) > 0 {
		// Show top 5 extracted keywords
		display := page.Metadata.TopKeywords
		if len(display) > 5 {
			display = display[:5]
		}
		sb.WriteString(fmt.Sprintf("top_keywords: %s (extracted)\n", strings.Join(display, ", ")))
	}

	sb.WriteString("\n")

	// Count block types - handle both FlatContent and Content
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

	// Show block type counts
	sb.WriteString("content types:\n")
	// Order: table, code, li, p, then others
	priority := []string{"table", "code", "li", "p"}
	for _, t := range priority {
		if count, ok := typeCounts[t]; ok {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", t, count))
		}
	}
	// Show other types
	for t, count := range typeCounts {
		isPriority := false
		for _, pt := range priority {
			if t == pt {
				isPriority = true
				break
			}
		}
		if !isPriority {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", t, count))
		}
	}

	sb.WriteString("\ndocument structure (headings h1-h6):\n")

	// Track if any headings found
	headingCount := 0
	var headingsSB strings.Builder

	if len(page.FlatContent) > 0 {
		// Cheap mode: extract headings from flat array
		for _, block := range page.FlatContent {
			// Check if block is a heading (h1-h6)
			if len(block.Type) == 2 && block.Type[0] == 'h' && block.Type[1] >= '1' && block.Type[1] <= '6' {
				level := int(block.Type[1] - '0')
				indent := strings.Repeat("  ", level-1)
				headingsSB.WriteString(fmt.Sprintf("%s- %s (%s)\n", indent, block.Text, block.Type))
				headingCount++
			}
		}
	} else {
		// Full-parse mode: recursively extract headings from hierarchical content
		var extractHeadings func(sections []models.Section)
		extractHeadings = func(sections []models.Section) {
			for _, section := range sections {
				if section.Heading != nil && section.Heading.Text != "" {
					indent := strings.Repeat("  ", section.Level-1)
					headingsSB.WriteString(fmt.Sprintf("%s- %s (%s)\n", indent, section.Heading.Text, section.Heading.Type))
					headingCount++
				}
				// Recurse into children
				if len(section.Children) > 0 {
					extractHeadings(section.Children)
				}
			}
		}
		extractHeadings(page.Content)
	}

	// Show headings or "(none)" message
	if headingCount > 0 {
		sb.WriteString(headingsSB.String())
	} else {
		sb.WriteString("  (no headings found)\n")
	}

	// Add helpful examples based on what's in the document
	sb.WriteString("\nuseful commands:\n")
	if typeCounts["table"] > 0 {
		sb.WriteString("  llm-web-parser db show --only=table <id>        # Show tables only\n")
	}
	if typeCounts["code"] > 0 {
		sb.WriteString("  llm-web-parser db show --only=code <id>         # Show code blocks only\n")
	}
	if typeCounts["li"] > 0 {
		sb.WriteString("  llm-web-parser db show --only=li <id>           # Show list items only\n")
	}
	sb.WriteString("  llm-web-parser db show --grep \"keyword\" <id>    # Search for keyword\n")
	sb.WriteString("  llm-web-parser db show --format json <id>       # Output as JSON for jq\n")

	return sb.String()
}

// filterByType filters ContentBlocks by type (comma-separated list).
func filterByType(page *models.Page, types string) (*models.Page, error) {
	if types == "" {
		return page, nil
	}

	typeList := strings.Split(types, ",")
	typeMap := make(map[string]bool)
	for _, t := range typeList {
		typeMap[strings.TrimSpace(t)] = true
	}

	// Handle FlatContent mode (cheap/minimal/wordcount)
	if len(page.FlatContent) > 0 {
		filtered := &models.Page{
			URL:         page.URL,
			Title:       page.Title,
			Metadata:    page.Metadata,
			FlatContent: make([]models.ContentBlock, 0),
		}

		// Filter blocks by type
		for _, block := range page.FlatContent {
			if typeMap[block.Type] {
				filtered.FlatContent = append(filtered.FlatContent, block)
			}
			// Special handling for table type - check if block contains a table
			if typeMap["table"] && block.Table != nil {
				filtered.FlatContent = append(filtered.FlatContent, block)
			}
		}

		return filtered, nil
	}

	// Handle hierarchical Content mode (full-parse)
	filtered := &models.Page{
		URL:      page.URL,
		Title:    page.Title,
		Metadata: page.Metadata,
		Content:  make([]models.Section, 0),
	}

	// Recursively filter sections
	var filterSection func(section models.Section) *models.Section
	filterSection = func(section models.Section) *models.Section {
		filteredSection := models.Section{
			ID:       section.ID,
			Heading:  section.Heading,
			Level:    section.Level,
			Blocks:   make([]models.ContentBlock, 0),
			Children: make([]models.Section, 0),
		}

		// Filter blocks by type
		for _, block := range section.Blocks {
			if typeMap[block.Type] {
				filteredSection.Blocks = append(filteredSection.Blocks, block)
			}
			// Special handling for table type - check if block contains a table
			if typeMap["table"] && block.Table != nil {
				filteredSection.Blocks = append(filteredSection.Blocks, block)
			}
		}

		// Recursively filter children
		for _, child := range section.Children {
			if filtered := filterSection(child); filtered != nil {
				filteredSection.Children = append(filteredSection.Children, *filtered)
			}
		}

		// Only include section if it has matching content
		if len(filteredSection.Blocks) > 0 || len(filteredSection.Children) > 0 {
			return &filteredSection
		}

		return nil
	}

	for _, section := range page.Content {
		if filteredSec := filterSection(section); filteredSec != nil {
			filtered.Content = append(filtered.Content, *filteredSec)
		}
	}

	return filtered, nil
}

// filterByGrep searches for a pattern in ContentBlocks and includes context.
func filterByGrep(page *models.Page, pattern string, context int) (*models.Page, error) {
	if pattern == "" {
		return page, nil
	}

	re, err := regexp.Compile("(?i)" + pattern) // Case-insensitive
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Handle FlatContent mode (cheap/minimal/wordcount)
	if len(page.FlatContent) > 0 {
		filtered := &models.Page{
			URL:         page.URL,
			Title:       page.Title,
			Metadata:    page.Metadata,
			FlatContent: make([]models.ContentBlock, 0),
		}

		// Find all matching indices
		matches := make(map[int]bool)
		for i, block := range page.FlatContent {
			if re.MatchString(block.Text) {
				// Mark this block and context blocks
				for j := i - context; j <= i+context; j++ {
					if j >= 0 && j < len(page.FlatContent) {
						matches[j] = true
					}
				}
			}
		}

		// Add all matched blocks (in order)
		for i, block := range page.FlatContent {
			if matches[i] {
				filtered.FlatContent = append(filtered.FlatContent, block)
			}
		}

		return filtered, nil
	}

	// Handle hierarchical Content mode (full-parse)
	filtered := &models.Page{
		URL:      page.URL,
		Title:    page.Title,
		Metadata: page.Metadata,
		Content:  make([]models.Section, 0),
	}

	// Recursively filter sections
	var filterSection func(section models.Section) *models.Section
	filterSection = func(section models.Section) *models.Section {
		filteredSection := models.Section{
			ID:       section.ID,
			Heading:  section.Heading,
			Level:    section.Level,
			Blocks:   make([]models.ContentBlock, 0),
			Children: make([]models.Section, 0),
		}

		// Check blocks for matches
		for _, block := range section.Blocks {
			if re.MatchString(block.Text) {
				filteredSection.Blocks = append(filteredSection.Blocks, block)
			}
		}

		// Recursively filter children
		for _, child := range section.Children {
			if filtered := filterSection(child); filtered != nil {
				filteredSection.Children = append(filteredSection.Children, *filtered)
			}
		}

		// Only include section if it has matching content
		if len(filteredSection.Blocks) > 0 || len(filteredSection.Children) > 0 {
			return &filteredSection
		}

		return nil
	}

	for _, section := range page.Content {
		if filteredSec := filterSection(section); filteredSec != nil {
			filtered.Content = append(filtered.Content, *filteredSec)
		}
	}

	return filtered, nil
}
