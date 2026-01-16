package extractors

import (
	"regexp"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
)

// WikiExtraction contains Wikipedia-specific extracted data.
type WikiExtraction struct {
	Infobox    *Infobox  `yaml:"infobox,omitempty" json:"infobox,omitempty"`
	TOC        []TOCEntry `yaml:"toc,omitempty" json:"toc,omitempty"`
	Sections   []Section  `yaml:"sections,omitempty" json:"sections,omitempty"`
	Categories []string   `yaml:"categories,omitempty" json:"categories,omitempty"`
}

// Infobox represents a Wikipedia-style infobox with key-value pairs.
type Infobox struct {
	Title  string            `yaml:"title,omitempty" json:"title,omitempty"`
	Fields map[string]string `yaml:"fields" json:"fields"`
}

// TOCEntry represents a table of contents entry.
type TOCEntry struct {
	Title string `yaml:"title" json:"title"`
	Level int    `yaml:"level" json:"level"`
	ID    string `yaml:"id,omitempty" json:"id,omitempty"`
}

// ExtractWiki extracts Wikipedia-specific content from a parsed page.
func ExtractWiki(page *models.Page) *WikiExtraction {
	if page == nil {
		return nil
	}

	extraction := &WikiExtraction{}

	if len(page.Content) > 0 {
		extraction.Infobox = extractInfobox(page.Content)
		extraction.TOC = extractTOC(page.Content)
		extraction.Sections = extractSections(page.Content)
		extraction.Categories = extractCategories(page.Content)
	}

	return extraction
}

// extractInfobox finds and parses the infobox.
func extractInfobox(sections []models.Section) *Infobox {
	// Infoboxes are typically in tables at the start of the article
	for _, section := range sections {
		for _, block := range section.Blocks {
			if block.Table != nil {
				// Check if this looks like an infobox
				if isInfoboxTable(block.Table) {
					return parseInfobox(block.Table)
				}
			}
		}
		// Only check first section
		break
	}
	return nil
}

// isInfoboxTable checks if a table looks like an infobox.
func isInfoboxTable(table *models.Table) bool {
	// Infoboxes typically have 2 columns (key, value) and no headers
	if len(table.Headers) > 0 {
		return false
	}

	// Check if rows have 2 columns
	if len(table.Rows) == 0 {
		return false
	}

	twoColumnCount := 0
	for _, row := range table.Rows {
		if len(row) == 2 {
			twoColumnCount++
		}
	}

	// If most rows have 2 columns, it's likely an infobox
	return float64(twoColumnCount)/float64(len(table.Rows)) > 0.7
}

// parseInfobox extracts key-value pairs from an infobox table.
func parseInfobox(table *models.Table) *Infobox {
	infobox := &Infobox{
		Fields: make(map[string]string),
	}

	for _, row := range table.Rows {
		if len(row) >= 2 {
			key := strings.TrimSpace(row[0])
			value := strings.TrimSpace(row[1])

			if key != "" && value != "" {
				infobox.Fields[key] = value
			}
		}
	}

	return infobox
}

// extractTOC generates a table of contents from section headings.
func extractTOC(sections []models.Section) []TOCEntry {
	var toc []TOCEntry

	var processSections func([]models.Section, int)
	processSections = func(sectionList []models.Section, depth int) {
		for _, section := range sectionList {
			if section.Heading != nil {
				toc = append(toc, TOCEntry{
					Title: section.Heading.Text,
					Level: section.Level,
					ID:    section.ID,
				})
			}

			if len(section.Children) > 0 {
				processSections(section.Children, depth+1)
			}
		}
	}

	processSections(sections, 0)
	return toc
}

// extractCategories finds Wikipedia categories.
func extractCategories(sections []models.Section) []string {
	var categories []string

	// Categories are usually at the end of the page
	if len(sections) == 0 {
		return categories
	}

	lastSection := sections[len(sections)-1]
	categoryPattern := regexp.MustCompile(`(?i)categor(?:y|ies):\s*(.+)`)

	var processSection func(models.Section)
	processSection = func(section models.Section) {
		for _, block := range section.Blocks {
			if matches := categoryPattern.FindStringSubmatch(block.Text); len(matches) > 1 {
				// Split categories by comma or pipe
				cats := regexp.MustCompile(`[,|]`).Split(matches[1], -1)
				for _, cat := range cats {
					cat = strings.TrimSpace(cat)
					if cat != "" {
						categories = append(categories, cat)
					}
				}
			}
		}

		for _, child := range section.Children {
			processSection(child)
		}
	}

	processSection(lastSection)
	return categories
}
