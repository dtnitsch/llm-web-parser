package models

import (
	"strings"
	
	"github.com/abadojack/whatlanggo"
)


type LinkType string

const (
	LinkInternal LinkType = "internal"
	LinkExternal LinkType = "external"
)

// Page represents the structured content of a single web page.
type Page struct {
	URL     string         `json:"url"`
	Title   string         `json:"title"`

	// Full mode
	Content []Section `json:"content,omitempty"`

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

type Table struct {
	Headers []string   `json:"headers,omitempty"`
	Rows    [][]string `json:"rows"`
}

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

func (p *Page) ComputeMetadata() {
	if p.Metadata.Computed {
		return
	}

	blocks := p.allTextBlocks()

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
	p.Metadata.EstimatedReadMin =
		float64(p.Metadata.WordCount) / 225.0

	p.Metadata.SectionCount = p.countSectionsRecursive(p.Content)
	p.Metadata.Language = p.detectLanguage(text)
	p.Metadata.ContentType = detectContentType(p)

	p.Metadata.Computed = true
}

func (p *Page) textForAnalysis() string {
	var sb strings.Builder

	if len(p.Content) > 0 {
		for _, s := range p.Content {
			for _, b := range s.Blocks {
				sb.WriteString(b.Text)
				sb.WriteString(" ")
			}
		}
		return sb.String()
	}

	for _, b := range p.FlatContent {
		sb.WriteString(b.Text)
		sb.WriteString(" ")
	}

	return sb.String()
}


func (p *Page) countWords() int {
	total := 0
	for _, b := range p.allBlocks() {
		total += len(strings.Fields(b.Text))
	}
	return total
}

func (p *Page) estimateReadTime(wordCount int) float64 {
	return float64(wordCount) / 225.0
}

func (p *Page) countSections() int {
	if len(p.Content) > 0 {
		return len(p.Content)
	}
	return 0
}

func (p *Page) countSectionsRecursive(sections []Section) int {
	count := 0
	for _, s := range sections {
		count++
		count += p.countSectionsRecursive(s.Children)
	}
	return count
}
/*
func (p *Page) allTextBlocks() []ContentBlock {
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
*/

func (p *Page) allTextBlocks() []ContentBlock {
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


func (p *Page) countBlocks() int {
	return len(p.allBlocks())
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


func (p *Page) detectLanguage(text string) string {
	info := whatlanggo.Detect(text)
	if info.IsReliable() {
		return info.Lang.Iso6391()
	}
	return "unknown"
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
		score["landing"] += 1
	}

	// Code density
	if countBlocksByType(p, "code", "pre") > 5 {
		score["documentation"] += 3
	}

	// Tables
	if countBlocksByType(p, "table") > 0 {
		score["documentation"] += 1
	}

	// Choose best
	best := "unknown"
	max := 0
	for k, v := range score {
		if v > max {
			best = k
			max = v
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
