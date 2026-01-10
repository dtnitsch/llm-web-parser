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


// ToPlainText flattens the document into readable text.
func (p *Page) ToPlainText() string {
	var sb strings.Builder

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
	p.Metadata.ContentType = detectContentType(p)

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



func countBlocksByType(p *Page, types ...string) int {
	set := make(map[string]struct{}, len(types))
	for _, t := range types {
		set[t] = struct{}{}
	}

	count := 0
	for _, b := range p.allBlocks() {
		if _, ok := set[b.Type]; ok {
			count++
		}
	}
	return count
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

func detectContentType(p *Page) string {
	score := map[string]int{
		"documentation": 0,
		"article":       0,
		"landing":       0,
	}

	// Size signals
	if p.Metadata.WordCount > 1200 {
		score["article"] += 2
	}
	if p.Metadata.WordCount < 500 {
		score["landing"] += 2
	}

	// Structure
	if p.Metadata.SectionCount >= 8 {
		score["article"] += 2
	}
	if p.Metadata.SectionCount <= 2 {
		score["landing"]++
	}

	// Code density
	if countBlocksByType(p, "code", "pre") > 5 {
		score["documentation"] += 3
	}

	// Tables
	if countBlocksByType(p, "table") > 0 {
		score["documentation"]++
	}

	// Choose best
	best := "unknown"
	maxScore := 0
	for k, v := range score {
		if v > maxScore {
			best = k
			maxScore = v
		}
	}

	return best
}

func (p *Page) allBlocks() []ContentBlock {
	if len(p.Content) > 0 {
		var blocks []ContentBlock
		for _, s := range p.Content {
			blocks = append(blocks, s.Blocks...)
		}
		return blocks
	}
	return p.FlatContent
}
