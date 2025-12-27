package models

import "strings"

// Page represents the structured content of a single web page.
type Page struct {
	URL     string         `json:"url"`
	Title   string         `json:"title"`
	Content []Section `json:"content"` // top-level sections
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
}

// ContentBlock represents a semantic block of content on a page.
type ContentBlock struct {
	ID    string `json:"id"`
	Type  string `json:"type"`           // "p", "li", "table", "code", etc
	Text  string `json:"text,omitempty"` // fallback text

	// Optional structured content
	Table *Table `json:"table,omitempty"`
	Code  *Code  `json:"code,omitempty"`

	// NEW: extracted links scoped to this block
	Links []Link `json:"links,omitempty"`	
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
