package extractors

import (
	"regexp"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
)

// DocsExtraction contains documentation-specific extracted data.
type DocsExtraction struct {
	CodeBlocks   []CodeBlock   `yaml:"code_blocks,omitempty" json:"code_blocks,omitempty"`
	APIParams    []APIParam    `yaml:"api_params,omitempty" json:"api_params,omitempty"`
	VersionInfo  string        `yaml:"version_info,omitempty" json:"version_info,omitempty"`
	Examples     []Example     `yaml:"examples,omitempty" json:"examples,omitempty"`
	Sections     []Section     `yaml:"sections,omitempty" json:"sections,omitempty"`
}

// CodeBlock represents an extracted code example.
type CodeBlock struct {
	Language string `yaml:"language,omitempty" json:"language,omitempty"`
	Code     string `yaml:"code" json:"code"`
	Context  string `yaml:"context,omitempty" json:"context,omitempty"` // surrounding text
}

// APIParam represents an API parameter from documentation.
type APIParam struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
}

// Example represents a code example with description.
type Example struct {
	Title       string `yaml:"title,omitempty" json:"title,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Code        string `yaml:"code" json:"code"`
	Language    string `yaml:"language,omitempty" json:"language,omitempty"`
}

// ExtractDocs extracts documentation-specific content from a parsed page.
func ExtractDocs(page *models.Page) *DocsExtraction {
	if page == nil {
		return nil
	}

	extraction := &DocsExtraction{}

	// Extract code blocks
	if len(page.Content) > 0 {
		extraction.CodeBlocks = extractCodeBlocks(page.Content)
		extraction.Sections = extractSections(page.Content)
		extraction.Examples = extractExamples(page.Content)
	} else if len(page.FlatContent) > 0 {
		extraction.CodeBlocks = extractCodeBlocksFlat(page.FlatContent)
	}

	// Extract version info
	extraction.VersionInfo = extractVersionInfo(page)

	// Extract API parameters (from tables or structured content)
	if len(page.Content) > 0 {
		extraction.APIParams = extractAPIParams(page.Content)
	}

	return extraction
}

// extractCodeBlocks finds code blocks in hierarchical sections.
func extractCodeBlocks(sections []models.Section) []CodeBlock {
	var blocks []CodeBlock

	var processSection func(models.Section, string)
	processSection = func(section models.Section, context string) {
		for _, block := range section.Blocks {
			if block.Code != nil {
				blocks = append(blocks, CodeBlock{
					Language: block.Code.Language,
					Code:     block.Code.Content,
					Context:  context,
				})
			}
		}
		for _, child := range section.Children {
			childContext := context
			if child.Heading != nil {
				childContext = child.Heading.Text
			}
			processSection(child, childContext)
		}
	}

	for _, section := range sections {
		context := ""
		if section.Heading != nil {
			context = section.Heading.Text
		}
		processSection(section, context)
	}

	return blocks
}

// extractCodeBlocksFlat finds code blocks in flat content.
func extractCodeBlocksFlat(blocks []models.ContentBlock) []CodeBlock {
	var result []CodeBlock

	for _, block := range blocks {
		if block.Code != nil {
			result = append(result, CodeBlock{
				Language: block.Code.Language,
				Code:     block.Code.Content,
			})
		}
	}

	return result
}

// extractVersionInfo looks for version information in the page.
func extractVersionInfo(page *models.Page) string {
	// Check metadata first
	if page.Metadata.PublishedTime != "" {
		return page.Metadata.PublishedTime
	}

	// Look for version patterns in title or content
	versionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)version\s+(\d+\.\d+(?:\.\d+)?)`),
		regexp.MustCompile(`(?i)v(\d+\.\d+(?:\.\d+)?)`),
		regexp.MustCompile(`(?i)Python\s+(\d+\.\d+)`),
		regexp.MustCompile(`(?i)Node\.js\s+(\d+\.\d+)`),
		regexp.MustCompile(`(?i)(?:React|Vue|Angular)\s+(\d+(?:\.\d+)?)`),
	}

	// Check title
	for _, pattern := range versionPatterns {
		if match := pattern.FindStringSubmatch(page.Title); len(match) > 1 {
			return match[1]
		}
	}

	// Check first few sections
	if len(page.Content) > 0 && len(page.Content) < 5 {
		for _, section := range page.Content[:min(3, len(page.Content))] {
			for _, block := range section.Blocks {
				for _, pattern := range versionPatterns {
					if match := pattern.FindStringSubmatch(block.Text); len(match) > 1 {
						return match[1]
					}
				}
			}
		}
	}

	return ""
}

// extractAPIParams looks for API parameter tables or structured lists.
func extractAPIParams(sections []models.Section) []APIParam {
	var params []APIParam

	var processSection func(models.Section)
	processSection = func(section models.Section) {
		for _, block := range section.Blocks {
			if block.Table != nil {
				// Look for parameter tables
				params = append(params, extractParamsFromTable(block.Table)...)
			}
		}
		for _, child := range section.Children {
			processSection(child)
		}
	}

	for _, section := range sections {
		// Look for "Parameters", "Arguments", "Options" sections
		if section.Heading != nil {
			title := strings.ToLower(section.Heading.Text)
			if strings.Contains(title, "parameter") ||
				strings.Contains(title, "argument") ||
				strings.Contains(title, "option") {
				processSection(section)
			}
		}
	}

	return params
}

// extractParamsFromTable extracts parameters from a table.
func extractParamsFromTable(table *models.Table) []APIParam {
	var params []APIParam

	if len(table.Headers) == 0 || len(table.Rows) == 0 {
		return params
	}

	// Find column indices
	nameIdx, typeIdx, descIdx, reqIdx := -1, -1, -1, -1
	for i, header := range table.Headers {
		lower := strings.ToLower(header)
		if strings.Contains(lower, "name") || strings.Contains(lower, "parameter") {
			nameIdx = i
		} else if strings.Contains(lower, "type") {
			typeIdx = i
		} else if strings.Contains(lower, "description") || strings.Contains(lower, "desc") {
			descIdx = i
		} else if strings.Contains(lower, "required") {
			reqIdx = i
		}
	}

	if nameIdx == -1 {
		return params
	}

	// Extract parameters from rows
	for _, row := range table.Rows {
		if len(row) <= nameIdx {
			continue
		}

		param := APIParam{
			Name: row[nameIdx],
		}

		if typeIdx >= 0 && len(row) > typeIdx {
			param.Type = row[typeIdx]
		}
		if descIdx >= 0 && len(row) > descIdx {
			param.Description = row[descIdx]
		}
		if reqIdx >= 0 && len(row) > reqIdx {
			lower := strings.ToLower(row[reqIdx])
			param.Required = strings.Contains(lower, "yes") || strings.Contains(lower, "true") || strings.Contains(lower, "required")
		}

		params = append(params, param)
	}

	return params
}

// extractExamples finds code examples with their descriptions.
func extractExamples(sections []models.Section) []Example {
	var examples []Example

	var processSection func(models.Section)
	processSection = func(section models.Section) {
		// Look for "Example" sections
		if section.Heading != nil {
			title := strings.ToLower(section.Heading.Text)
			if strings.Contains(title, "example") {
				// Extract code blocks from this section
				for _, block := range section.Blocks {
					if block.Code != nil {
						example := Example{
							Title:    section.Heading.Text,
							Code:     block.Code.Content,
							Language: block.Code.Language,
						}
						// Get description from previous paragraph
						examples = append(examples, example)
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

	return examples
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
