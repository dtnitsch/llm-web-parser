package extractors

import (
	"regexp"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
)

// AcademicExtraction contains academic-specific extracted data.
type AcademicExtraction struct {
	Abstract   *Section   `yaml:"abstract,omitempty" json:"abstract,omitempty"`
	Sections   []Section  `yaml:"sections,omitempty" json:"sections,omitempty"`
	Citations  []Citation `yaml:"citations,omitempty" json:"citations,omitempty"`
	References []Reference `yaml:"references,omitempty" json:"references,omitempty"`
}

// Section represents a structured section (e.g., Introduction, Methods, Results).
type Section struct {
	Title   string `yaml:"title" json:"title"`
	Content string `yaml:"content" json:"content"`
	Level   int    `yaml:"level" json:"level"` // heading level (1-6)
}

// Citation represents a numbered citation [1], [2], etc.
type Citation struct {
	Number int    `yaml:"number" json:"number"`
	Text   string `yaml:"text,omitempty" json:"text,omitempty"`
}

// Reference represents a bibliography entry.
type Reference struct {
	Index int    `yaml:"index" json:"index"`
	Text  string `yaml:"text" json:"text"`
}

// ExtractAcademic extracts academic-specific content from a parsed page.
func ExtractAcademic(page *models.Page) *AcademicExtraction {
	if page == nil {
		return nil
	}

	extraction := &AcademicExtraction{}

	// Extract from full mode content (hierarchical sections)
	if len(page.Content) > 0 {
		extraction.Abstract = extractAbstract(page.Content)
		extraction.Sections = extractSections(page.Content)
		extraction.References = extractReferences(page.Content)
	}

	// Extract citations from flat content or full content
	if len(page.FlatContent) > 0 {
		extraction.Citations = extractCitations(page.FlatContent)
	} else {
		extraction.Citations = extractCitationsFromSections(page.Content)
	}

	return extraction
}

// extractAbstract finds the abstract section.
func extractAbstract(sections []models.Section) *Section {
	for _, section := range sections {
		if section.Heading != nil {
			title := strings.ToLower(section.Heading.Text)
			if strings.Contains(title, "abstract") {
				content := extractSectionText(section)
				return &Section{
					Title:   section.Heading.Text,
					Content: content,
					Level:   section.Level,
				}
			}
		}
	}
	return nil
}

// extractSections extracts all major sections with their content.
func extractSections(sections []models.Section) []Section {
	var result []Section

	for _, section := range sections {
		if section.Heading != nil && section.Level <= 2 {
			// Only top-level sections (h1, h2)
			content := extractSectionText(section)
			if content != "" {
				result = append(result, Section{
					Title:   section.Heading.Text,
					Content: content,
					Level:   section.Level,
				})
			}
		}
	}

	return result
}

// extractSectionText extracts all text content from a section and its children.
func extractSectionText(section models.Section) string {
	var sb strings.Builder

	// Extract text from all blocks
	for _, block := range section.Blocks {
		if block.Text != "" {
			sb.WriteString(block.Text)
			sb.WriteString("\n\n")
		}
	}

	// Extract text from child sections
	for _, child := range section.Children {
		childText := extractSectionText(child)
		sb.WriteString(childText)
	}

	return strings.TrimSpace(sb.String())
}

// extractCitationsFromSections extracts citations from hierarchical sections.
func extractCitationsFromSections(sections []models.Section) []Citation {
	var citations []Citation
	seen := make(map[int]bool)
	citationPattern := regexp.MustCompile(`\[(\d+)\]`)

	var processSection func(models.Section)
	processSection = func(section models.Section) {
		for _, block := range section.Blocks {
			matches := citationPattern.FindAllStringSubmatch(block.Text, -1)
			for _, match := range matches {
				if len(match) > 1 {
					var num int
					for _, c := range match[1] {
						num = num*10 + int(c-'0')
					}
					if num > 0 && !seen[num] {
						seen[num] = true
						citations = append(citations, Citation{
							Number: num,
						})
					}
				}
			}
		}
		for _, child := range section.Children {
			processSection(child)
		}
	}

	for _, section := range sections {
		processSection(section)
	}

	return citations
}

// extractReferences finds the references/bibliography section.
func extractReferences(sections []models.Section) []Reference {
	var references []Reference

	// Find the references section
	var refSection *models.Section
	for i := range sections {
		if sections[i].Heading != nil {
			title := strings.ToLower(sections[i].Heading.Text)
			if strings.Contains(title, "reference") || 
			   strings.Contains(title, "bibliography") ||
			   strings.Contains(title, "works cited") {
				refSection = &sections[i]
				break
			}
		}
	}

	if refSection == nil {
		return references
	}

	// Extract reference entries
	index := 1
	for _, block := range refSection.Blocks {
		if block.Type == "p" && len(block.Text) > 20 {
			// Each paragraph in references section is likely a reference
			references = append(references, Reference{
				Index: index,
				Text:  strings.TrimSpace(block.Text),
			})
			index++
		}
	}

	return references
}

// extractCitations finds numbered citations [1], [2], etc.
func extractCitations(blocks []models.ContentBlock) []Citation {
	var citations []Citation
	seen := make(map[int]bool)

	// Pattern: [1], [2], [123], etc.
	citationPattern := regexp.MustCompile(`\[(\d+)\]`)

	for _, block := range blocks {
		matches := citationPattern.FindAllStringSubmatch(block.Text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				// Simple string to int conversion
				var num int
				for _, c := range match[1] {
					num = num*10 + int(c-'0')
				}

				if num > 0 && !seen[num] {
					seen[num] = true
					citations = append(citations, Citation{
						Number: num,
					})
				}
			}
		}
	}

	return citations
}
