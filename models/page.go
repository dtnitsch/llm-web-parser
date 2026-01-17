package models

import (
	"math"
	"strings"

	lingua "github.com/pemistahl/lingua-go"
)

// LinkType represents the type of a hyperlink (internal or external).
type LinkType string

const (
	// LinkInternal indicates a link to the same domain.
	LinkInternal LinkType = "internal"
	LinkExternal LinkType = "external"
)

var languageDetector = lingua.NewLanguageDetectorBuilder().
	FromAllLanguages().
	WithMinimumRelativeDistance(0.2).
	Build()

// Page represents the structured content of a single web page.
type Page struct {
	URL     string         `json:"url"`
	Title   string         `json:"title"`

	// Full mode
	Content []Section `json:"content"`

	// Cheap mode
	FlatContent []ContentBlock `json:"flat_content,omitempty"`

	// Word counts, section counts, language, etc
	Metadata PageMetadata   `json:"metadata"`
}

// Section represents a logical section of a document,
// typically introduced by a heading.
type Section struct {
	ID       string         `json:"id"`
	Heading  *ContentBlock  `json:"heading,omitempty"`
	Level    int            `json:"level"` // h1 = 1, h2 = 2, etc
	Blocks   []ContentBlock `json:"blocks"`
	Children []Section      `json:"children,omitempty"`
}

// Table represents a data table extracted from HTML.
type Table struct{
	Headers []string   `json:"headers,omitempty"`
	Rows    [][]string `json:"rows"`
}

// Code represents a code block extracted from HTML.
type Code struct {
	Language string `json:"language,omitempty"`
	Content  string `json:"content"`
}

// Link represents a hyperlink found in a content block.
type Link struct {
	Href string `json:"href"`
	Text string `json:"text"`
	Type LinkType `json:"type"`
}

// ContentBlock represents a semantic block of content on a page.
type ContentBlock struct {
	ID    string `json:"id"`
	Type  string `json:"type"`           // "p", "li", "table", "code", etc
	Text  string `json:"text,omitempty"` // fallback text

	// Optional structured content
	Table *Table `json:"table,omitempty"`
	Code  *Code  `json:"code,omitempty"`

	// extracted links scoped to this block
	Links []Link `json:"links,omitempty"`

	// LLM confidence Scores
	Confidence float64 `json:"confidence"`
}

// MarshalYAML creates a compact YAML representation by omitting null/empty/default fields.
// This reduces token waste by ~75% for LLM consumption.
func (cb ContentBlock) MarshalYAML() (interface{}, error) {
	// Create map with only non-null, non-default fields
	m := make(map[string]interface{})

	// Always include type (required field)
	m["type"] = cb.Type

	// Include text if not empty
	if cb.Text != "" {
		m["text"] = cb.Text
	}

	// Include table only if present
	if cb.Table != nil {
		m["table"] = cb.Table
	}

	// Include code only if present
	if cb.Code != nil {
		m["code"] = cb.Code
	}

	// Include links only if non-empty
	if len(cb.Links) > 0 {
		m["links"] = cb.Links
	}

	// Include confidence only if not default (0.5)
	// Float comparison: check if not within epsilon of 0.5
	if cb.Confidence < 0.49 || cb.Confidence > 0.51 {
		m["confidence"] = cb.Confidence
	}

	// Note: ID is intentionally omitted (sequential IDs not useful for LLM reading)

	return m, nil
}

// ToPlainText flattens the document into readable text.
func (p *Page) ToPlainText() string {
	var sb strings.Builder

	// Try FlatContent first (used in cheap/minimal parse modes)
	if len(p.FlatContent) > 0 {
		for _, block := range p.FlatContent {
			// Handle different block types
			switch block.Type {
			case "table":
				if block.Table != nil {
					for _, row := range block.Table.Rows {
						sb.WriteString(strings.Join(row, " "))
						sb.WriteString("\n")
					}
				}
			case "code":
				if block.Code != nil {
					sb.WriteString(block.Code.Content)
					sb.WriteString("\n")
				}
			default:
				// Regular text blocks (p, li, h1, h2, etc.)
				if block.Text != "" {
					sb.WriteString(block.Text)
					sb.WriteString("\n")
				}
			}
		}
		return sb.String()
	}

	// Fall back to hierarchical Content (full parse mode)
	for _, section := range p.Content {
		flattenSection(&sb, section)
	}

	return sb.String()
}

func flattenSection(sb *strings.Builder, s Section) {
	if s.Heading != nil {
		sb.WriteString(s.Heading.Text)
		sb.WriteString("\n")
	}

	for _, block := range s.Blocks {
		switch block.Type {
		case "table":
			for _, row := range block.Table.Rows {
				sb.WriteString(strings.Join(row, " "))
				sb.WriteString("\n")
			}
		case "code":
			sb.WriteString(block.Code.Content)
			sb.WriteString("\n")
		default:
			sb.WriteString(block.Text)
			sb.WriteString("\n")
		}
	}

	for _, child := range s.Children {
		flattenSection(sb, child)
	}
}

// ComputeMetadata calculates metadata fields from page content.
func (p *Page) ComputeMetadata() {
	if p.Metadata.Computed {
		return
	}

	blocks := p.AllTextBlocks()

	var textBuilder strings.Builder
	for _, b := range blocks {
		if b.Text != "" {
			textBuilder.WriteString(b.Text)
			textBuilder.WriteString(" ")
		}
	}

	text := textBuilder.String()

	p.Metadata.BlockCount = len(blocks)
	p.Metadata.WordCount = len(strings.Fields(text))
	p.Metadata.EstimatedReadMin = math.Round((float64(p.Metadata.WordCount)/225.0)*10) / 10

	p.Metadata.SectionCount = p.countSectionsRecursive(p.Content)
	p.Metadata.Language, p.Metadata.LanguageConfidence = p.detectLanguage(text)
	// ContentType is now set by parser via detector.DetectContentType() - don't overwrite it here

	p.Metadata.Computed = true
}


func (p *Page) countSectionsRecursive(sections []Section) int {
	count := 0
	for _, s := range sections {
		count++
		count += p.countSectionsRecursive(s.Children)
	}
	return count
}

// AllTextBlocks returns all content blocks from the page (flat list).
func (p *Page) AllTextBlocks() []ContentBlock {
	var blocks []ContentBlock

	// Cheap mode
	if len(p.FlatContent) > 0 {
		return p.FlatContent
	}

	// Full mode
	var walkSections func(sections []Section)
	walkSections = func(sections []Section) {
		for _, s := range sections {
			if s.Heading != nil {
				blocks = append(blocks, *s.Heading)
			}
			blocks = append(blocks, s.Blocks...)
			if len(s.Children) > 0 {
				walkSections(s.Children)
			}
		}
	}

	walkSections(p.Content)
	return blocks
}
func (p *Page) detectLanguage(text string) (string, float64) {
	if len(text) < 100 {
		return "unknown", 0.0
	}

	lang, exists := languageDetector.DetectLanguageOf(text)
	if !exists {
		return "unknown", 0.0
	}

	iso := lang.IsoCode639_1().String()
	if iso == "" {
		return "unknown", 0.0
	}

	// Heuristic confidence based on text length
	words := len(strings.Fields(text))

	var confidence float64
	switch {
	case words > 5000:
		confidence = 0.99
	case words > 1000:
		confidence = 0.95
	case words > 300:
		confidence = 0.9
	default:
		confidence = 0.75
	}

	return strings.ToLower(iso), confidence
}
