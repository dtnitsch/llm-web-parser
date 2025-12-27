package models

import "strings"

// Page represents the structured content of a single web page.
type Page struct {
	URL     string         `json:"url"`
	Title   string         `json:"title"`
	Content []ContentBlock `json:"content"`
}

type Table struct {
	Headers []string   `json:"headers,omitempty"`
	Rows    [][]string `json:"rows"`
}

type Code struct {
	Language string `json:"language,omitempty"`
	Content  string `json:"content"`
}

// ContentBlock represents a semantic block of text on a page.
type ContentBlock struct {
	Type string  `json:"type"` // e.g., "h1", "h2", "p", "li"
	Text string  `json:"text"`
	Table *Table `json:"table,omitempty"` // structured tables
	Code  *Code  `json:"code,omitempty"`  // structured code blocksuu
}

// ToPlainText concatenates readable text from all content blocks.
func (p *Page) ToPlainText() string {
	var sb strings.Builder

	for _, block := range p.Content {
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

	return sb.String()
}
