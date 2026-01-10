package parser

import (
	"bufio"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/detector"
	"github.com/go-shiori/go-readability"
)

type Parser struct{}

func (p *Parser) Parse(req models.ParseRequest) (*models.Page, error) {
	mode := models.ResolveParseMode(req)

	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	readParser := readability.NewParser()
	article, err := readParser.Parse(strings.NewReader(req.HTML), parsedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML with readability: %w", err)
	}

	var page *models.Page

	switch mode {
	case models.ParseModeMinimal:
		page, err = p.parseMinimal(req.URL, article, parsedURL)
		if err != nil {
			return nil, err
		}
		// No auto-escalation for minimal mode - user must explicitly use --features

	case models.ParseModeCheap:
		page, err = p.parseCheap(req.URL, article, parsedURL)
		if err != nil {
			return nil, err
		}

		page.ComputeMetadata()

		// 🔑 escalation logic lives HERE
		if page.Metadata.ExtractionQuality == "low" {
			return p.parseFull(req.URL, article, parsedURL)
		}

	case models.ParseModeFull:
		page, err = p.parseFull(req.URL, article, parsedURL)
		if err != nil {
			return nil, err
		}

		page.ComputeMetadata()
	}

	return page, nil
}

func (p *Parser) parseMinimal(rawURL string, article readability.Article, _ *url.URL) (*models.Page, error) {
	// Minimal mode: ONLY extract metadata from go-readability, no content parsing
	page := &models.Page{
		URL:   rawURL,
		Title: normalizeText(article.Title),
		// No Content or FlatContent - empty!
	}

	page.Metadata.ExtractionMode = "minimal"
	page.Metadata.ExtractionQuality = "minimal" // New quality level

	// Enrich with free metadata (readability + smart detection)
	enrichMetadata(page, article, rawURL)

	// Don't compute full metadata - we have no content blocks
	// Just mark as computed so downstream doesn't try
	page.Metadata.Computed = true

	return page, nil
}

func (p *Parser) parseFull(
	rawURL string,
	article readability.Article,
	parsedURL *url.URL,
) (*models.Page, error) {

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML document: %w", err)
	}

	var (
		rootSections   []models.Section
		sectionStack   []*models.Section
		sectionCounter int
		blockCounter   int
	)

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

	doc.Find("h1,h2,h3,h4,h5,h6,p,li,pre,code,table").Each(func(_ int, s *goquery.Selection) {
		tag := goquery.NodeName(s)
		text := normalizeText(s.Text())
		if text == "" && tag != "table" {
			return
		}

		links := extractLinks(s, parsedURL)

		// HEADINGS
		if strings.HasPrefix(tag, "h") {
			level := int(tag[1] - '0')
			sectionCounter++
			blockCounter++

			headingBlock := models.ContentBlock{
				ID:         fmt.Sprintf("block-%d", blockCounter),
				Type:       tag,
				Text:       text,
				Links:      links,
				Confidence: 0.7,
			}

			newSection := models.Section{
				ID:      fmt.Sprintf("section-%d", sectionCounter),
				Level:   level,
				Heading: &headingBlock,
			}

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

		// TABLES
		if tag == "table" {
			blockCounter++
			currentSection().Blocks = append(currentSection().Blocks, models.ContentBlock{
				ID:         fmt.Sprintf("block-%d", blockCounter),
				Type:       "table",
				Table:      extractTable(s),
				Links:      links,
				Confidence: 0.95,
			})
			return
		}

		// CODE
		if tag == "pre" || tag == "code" {
			blockCounter++
			currentSection().Blocks = append(currentSection().Blocks, models.ContentBlock{
				ID:         fmt.Sprintf("block-%d", blockCounter),
				Type:       "code",
				Code:       &models.Code{Content: s.Text()},
				Links:      links,
				Confidence: 0.95,
			})
			return
		}

		// TEXT
		blockCounter++
		currentSection().Blocks = append(currentSection().Blocks, models.ContentBlock{
			ID:         fmt.Sprintf("block-%d", blockCounter),
			Type:       tag,
			Text:       text,
			Links:      links,
			Confidence: computeConfidence(text, len(links), tag),
		})
	})

	page := &models.Page{
		URL:     rawURL,
		Title:   normalizeText(article.Title),
		Content: rootSections,
	}

	page.Metadata.ExtractionMode = "full"
	page.Metadata.ExtractionQuality = "ok"

	// Enrich metadata from article and detector
	enrichMetadata(page, article, rawURL)

	return page, nil
}

func (p *Parser) parseCheap(rawURL string, article readability.Article, parsedURL *url.URL) (*models.Page, error) {

	doc, err := goquery.NewDocumentFromReader(
		strings.NewReader(article.Content),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML document: %w", err)
	}

	var blocks []models.ContentBlock
	blockCounter := 0

	doc.Find("h1,h2,h3,p,div,pre,blockquote").Each(func(_ int, s *goquery.Selection) {
		// Skip container divs with children to avoid duplication
		if s.Children().Length() > 0 && goquery.NodeName(s) == "div" {
			return
		}

		text := normalizeText(s.Text())
		if text == "" {
			return
		}

		blockCounter++
		links := extractLinks(s, parsedURL)

		blocks = append(blocks, models.ContentBlock{
			ID:         fmt.Sprintf("block-%d", blockCounter),
			Type:       goquery.NodeName(s),
			Text:       text,
			Links:      links,
			Confidence: 0.5, // neutral
		})
	})

	quality := "ok"
	if len(blocks) < 5 {
		quality = "low"
	}

	page := &models.Page{
		URL:         rawURL,
		Title:       normalizeText(article.Title),
		FlatContent: blocks,
	}

	page.Metadata.ExtractionMode = "cheap"
	page.Metadata.ExtractionQuality = quality

	// Enrich metadata from article and detector
	enrichMetadata(page, article, rawURL)

	return page, nil
}

func extractTable(s *goquery.Selection) *models.Table {
	var headers []string
	var rows [][]string

	s.Find("tr").Each(func(_ int, tr *goquery.Selection) {
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


// enrichMetadata populates page metadata from readability article and detector analysis
func enrichMetadata(page *models.Page, article readability.Article, rawURL string) {
	// Populate readability metadata
	page.Metadata.Author = article.Byline
	page.Metadata.Excerpt = article.Excerpt
	page.Metadata.SiteName = article.SiteName
	if article.PublishedTime != nil {
		page.Metadata.PublishedTime = article.PublishedTime.Format("2006-01-02")
	}
	page.Metadata.Favicon = article.Favicon
	page.Metadata.Image = article.Image

	// Get content for detector analysis (use article.Content for academic detection)
	// This is more reliable than page.ToPlainText() which may be empty in cheap mode
	enriched := detector.Analyze(rawURL, article, article.Content, nil)

	// Populate detector metadata
	page.Metadata.DomainType = enriched.DomainType
	page.Metadata.DomainCategory = enriched.DomainCategory
	page.Metadata.Country = enriched.Country
	page.Metadata.Confidence = enriched.Confidence

	page.Metadata.HasDOI = enriched.HasDOI
	page.Metadata.DOIPattern = enriched.DOIPattern
	page.Metadata.HasArXiv = enriched.HasArXiv
	page.Metadata.ArXivID = enriched.ArXivID
	page.Metadata.HasLaTeX = enriched.HasLaTeX
	page.Metadata.HasCitations = enriched.HasCitations
	page.Metadata.HasReferences = enriched.HasReferences
	page.Metadata.HasAbstract = enriched.HasAbstract
	page.Metadata.AcademicScore = enriched.AcademicScore
}

