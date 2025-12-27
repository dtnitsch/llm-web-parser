package parser

import (
	"bufio"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/go-shiori/go-readability" // Changed import
)

type Parser struct{}

// ParseToStructured uses the go-readability library to extract the main article
// content and then parses that clean content into a structured Page object.
func (p *Parser) ParseToStructured(rawURL, html string) (*models.Page, error) {
	// Parse the URL to pass to the readability parser
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	// Let go-readability find the main content
	readabilityParser := readability.NewParser()
	article, err := readabilityParser.Parse(strings.NewReader(html), parsedURL) // Corrected: use method and pass *url.URL
	if err != nil {
		return nil, err
	}

	// Now, use goquery on the *clean* HTML content provided by readability
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.Content)) // Changed to field access
	if err != nil {
		return nil, err
	}

	var content []models.ContentBlock
	// Find all content-bearing tags we care about within the distilled content
    doc.Find("h1,h2,h3,h4,p,li,table,pre").Each(func(i int, s *goquery.Selection) {
      tag := goquery.NodeName(s)

      switch tag {

      case "table":
        table := extractTable(s)
        if table != nil {
          content = append(content, models.ContentBlock{
            Type:  "table",
            Table: table,
          })
        }

      case "pre":
        code := extractCodeBlock(s)
        if code != nil {
          content = append(content, models.ContentBlock{
            Type: "code",
            Code: code,
          })
        }

      default:
        text := normalizeText(s.Text())
        if text != "" {
          content = append(content, models.ContentBlock{
            Type: tag,
            Text: text,
          })
        }
      }
    })


	page := &models.Page{
		URL:     rawURL,
		Title:   normalizeText(article.Title), // Changed to field access
		Content: content,
	}

	return page, nil
}

// normalizeText cleans up a string by trimming space and removing excess newlines.
func normalizeText(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// Write the line and a single space for separation
			b.WriteString(line)
			b.WriteString(" ")
		}
	}
	// Return the result, trimming the final space
	return strings.TrimSpace(b.String())
}

func extractTable(s *goquery.Selection) *models.Table {
	var headers []string
	var rows [][]string

	// Try explicit headers
	s.Find("thead tr th").Each(func(i int, th *goquery.Selection) {
		headers = append(headers, normalizeText(th.Text()))
	})

	// Fallback: first row
	if len(headers) == 0 {
		s.Find("tr").First().Find("th,td").Each(func(i int, cell *goquery.Selection) {
			headers = append(headers, normalizeText(cell.Text()))
		})
	}

	// Body rows
	s.Find("tbody tr").Each(func(i int, tr *goquery.Selection) {
		var row []string
		tr.Find("td").Each(func(j int, td *goquery.Selection) {
			row = append(row, normalizeText(td.Text()))
		})
		if len(row) > 0 {
			rows = append(rows, row)
		}
	})

	if len(headers) == 0 && len(rows) == 0 {
		return nil
	}

	return &models.Table{
		Headers: headers,
		Rows:    rows,
	}
}

func extractCodeBlock(s *goquery.Selection) *models.Code {
	codeSel := s.Find("code")
	if codeSel.Length() == 0 {
		return nil
	}

	code := strings.TrimSpace(codeSel.Text())
	if code == "" {
		return nil
	}

	lang, _ := codeSel.Attr("class")
	lang = strings.TrimPrefix(lang, "language-")

	return &models.Code{
		Language: lang,
		Content:  code,
	}
}


