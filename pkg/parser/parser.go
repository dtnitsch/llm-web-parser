package parser

import (
	"bufio"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/go-shiori/go-readability"
)

type Parser struct{}

func (p *Parser) ParseToStructured(req models.ParseRequest) (*models.Page, error) {
	// Cheap or Full mode
	mode := models.ResolveParseMode(req)

	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return nil, err
	}

	readParser := readability.NewParser()
	article, err := readParser.Parse(strings.NewReader(req.HTML), parsedURL)
	if err != nil {
		return nil, err
	}

	if mode == models.ParseModeCheap {
		return p.parseCheap(req.URL, article, parsedURL)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.Content))
	if err != nil {
		return nil, err
	}

	var (
		rootSections   []models.Section
		sectionStack   []*models.Section
		sectionCounter int
		blockCounter   int
	)

	// Helper: get current section (or create implicit root)
	currentSection := func() *models.Section {
		if len(sectionStack) == 0 {
			sectionCounter++
			s := models.Section{
				ID:    fmt.Sprintf("section-%d", sectionCounter),
				Level: 0,
			}
			rootSections = append(rootSections, s)
			sectionStack = append(sectionStack, &rootSections[len(rootSections)-1])
		}
		return sectionStack[len(sectionStack)-1]
	}

	doc.Find("h1,h2,h3,h4,h5,h6,p,li,pre,code,table").Each(func(i int, s *goquery.Selection) {
		tag := goquery.NodeName(s)
		text := normalizeText(s.Text())
		if text == "" && tag != "table" {
			return
		}

		links := extractLinks(s, parsedURL)

		// ---- HEADINGS ----
		if strings.HasPrefix(tag, "h") {
			level := int(tag[1] - '0')
			
			sectionCounter++
			blockCounter++

			headingBlock := models.ContentBlock{
				ID:   fmt.Sprintf("block-%d", blockCounter),
				Type: tag,
				Text: text,
				Links: links,
				Confidence: 0.7, 
			}

			newSection := models.Section{
				ID:      fmt.Sprintf("section-%d", sectionCounter),
				Level:   level,
				Heading: &headingBlock,
			}

			// Pop until we find parent
			for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1].Level >= level {
				sectionStack = sectionStack[:len(sectionStack)-1]
			}

			if len(sectionStack) == 0 {
				rootSections = append(rootSections, newSection)
				sectionStack = append(sectionStack, &rootSections[len(rootSections)-1])
			} else {
				parent := sectionStack[len(sectionStack)-1]
				parent.Children = append(parent.Children, newSection)
				sectionStack = append(sectionStack, &parent.Children[len(parent.Children)-1])
			}

			return
		}

		// ---- TABLES ----
		if tag == "table" {
			blockCounter++
			table := extractTable(s)

			currentSection().Blocks = append(currentSection().Blocks, models.ContentBlock{
				ID:    fmt.Sprintf("block-%d", blockCounter),
				Type:  "table",
				Table: table,
				Links: links, 
				Confidence: 0.95, 
			})
			return
		}

		// ---- CODE ----
		if tag == "pre" || tag == "code" {
			blockCounter++
			currentSection().Blocks = append(currentSection().Blocks, models.ContentBlock{
				ID:   fmt.Sprintf("block-%d", blockCounter),
				Type: "code",
				Code: &models.Code{
					Content: s.Text(),
				},
				Links: extractLinks(s, parsedURL),
				Confidence: 0.95,
			})
			return
		}

		// ---- TEXT BLOCKS ----
		blockCounter++
		currentSection().Blocks = append(currentSection().Blocks, models.ContentBlock{
			ID:   fmt.Sprintf("block-%d", blockCounter),
			Type: tag,
			Text: text,
			Links: extractLinks(s, parsedURL),
			Confidence: computeConfidence(text, len(links), tag),
		})
	})

	page := &models.Page{
		URL:     req.URL,
		Title:   normalizeText(article.Title),
		Content: rootSections,
	}

	return page, nil
}

func (p *Parser) parseCheap(rawURL string, article readability.Article, parsedUrl *url.URL) (*models.Page, error) {

	doc, err := goquery.NewDocumentFromReader(
		strings.NewReader(article.Content),
	)
	if err != nil {
		return nil, err
	}

	var blocks []models.ContentBlock
	blockCounter := 0

	doc.Find("h1,h2,h3,p").Each(func(_ int, s *goquery.Selection) {
		text := normalizeText(s.Text())
		if text == "" {
			return
		}

		blockCounter++
		links := extractLinks(s, parsedUrl)

		blocks = append(blocks, models.ContentBlock{
			ID:         fmt.Sprintf("block-%d", blockCounter),
			Type:       goquery.NodeName(s),
			Text:       text,
			Links:      links,
			Confidence: 0.5, // neutral
		})
	})

	return &models.Page{
		URL:   rawURL,
		Title: normalizeText(article.Title),
		// Flat content, no sections
		FlatContent: blocks,
	}, nil
}

func extractTable(s *goquery.Selection) *models.Table {
	var headers []string
	var rows [][]string

	s.Find("tr").Each(func(i int, tr *goquery.Selection) {
		var row []string

		tr.Find("th").Each(func(_ int, th *goquery.Selection) {
			headers = append(headers, normalizeText(th.Text()))
		})

		tr.Find("td").Each(func(_ int, td *goquery.Selection) {
			row = append(row, normalizeText(td.Text()))
		})

		if len(row) > 0 {
			rows = append(rows, row)
		}
	})

	return &models.Table{
		Headers: headers,
		Rows:    rows,
	}
}

func normalizeText(input string) string {
	var b strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(input))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			b.WriteString(line)
			b.WriteString(" ")
		}
	}

	return strings.TrimSpace(b.String())
}

func extractLinks(s *goquery.Selection,	pageURL *url.URL) []models.Link {
	var links []models.Link

	s.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		text := normalizeText(a.Text())
		if href == "" {
			return
		}

		links = append(links, models.Link{
			Href: href,
			Text: text,
			Type: classifyLink(href, pageURL),
		})
	})

	return links
}

func classifyLink(href string, pageURL *url.URL) models.LinkType {
	if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "/") {
		return models.LinkInternal
	}

	u, err := url.Parse(href)
	if err != nil || u.Host == "" {
		return models.LinkInternal
	}

	if u.Host == pageURL.Host {
		return models.LinkInternal
	}

	return models.LinkExternal
}

func computeConfidence(text string, links int, blockType string) float64 {
	if blockType == "code" || blockType == "table" {
		return 0.95 // structured content is usually high-signal
	}

	words := len(strings.Fields(text))
	if words == 0 {
		return 0.0
	}

	score := 0.4

	// Text density
	switch {
	case words > 120:
		score += 0.4
	case words > 40:
		score += 0.25
	case words > 15:
		score += 0.1 
	}

	// Link penalty
	score -= float64(links) * 0.05

	// Clamp
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func resolveParseMode(req models.ParseRequest) models.ParseMode {
	// Explicit mode wins unless unsafe
	if req.Mode != 0 {
		if req.Mode == models.ParseModeCheap && req.RequireCitations {
			return models.ParseModeFull
		}
		return req.Mode
	}

	// Infer intent
	if req.RequireCitations {
		return models.ParseModeFull
	}

	// Default
	return models.ParseModeCheap
}

