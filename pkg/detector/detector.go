package detector

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/go-shiori/go-readability"
)

// EnrichedMetadata contains smart detection results from free/cheap analysis
type EnrichedMetadata struct {
	// Domain classification
	DomainType     string  // gov, edu, academic, commercial, mobile, unknown
	DomainCategory string  // gov/health, academic/ai, news/tech, docs/api, commerce, blog
	Country        string  // TLD-based guess: us, uk, de, jp, etc
	Confidence     float64 // 0-10 scale based on signal strength

	// Academic signals
	HasDOI         bool
	DOIPattern     string
	HasArXiv       bool
	ArXivID        string
	HasLaTeX       bool
	HasCitations   bool
	HasReferences  bool
	HasAbstract    bool
	AcademicScore  float64 // 0-10 academic confidence

	// Readability enrichment
	Author        string
	Excerpt       string
	SiteName      string
	PublishedTime string
	Favicon       string
	Image         string

	// HTTP enrichment
	FinalURL      string
	RedirectChain []string
	HTTPContentType  string
	StatusCode    int
}

// HTTPMetadata contains optional HTTP response data
type HTTPMetadata struct {
	StatusCode    int
	ContentType   string
	FinalURL      string
	RedirectChain []string
}

// Analyze performs smart detection on URL, readability article, and content
func Analyze(rawURL string, article readability.Article, content string, httpMeta *HTTPMetadata) *EnrichedMetadata {
	em := &EnrichedMetadata{}

	// Add HTTP metadata if provided
	if httpMeta != nil {
		em.StatusCode = httpMeta.StatusCode
		em.HTTPContentType = httpMeta.ContentType
		em.FinalURL = httpMeta.FinalURL
		em.RedirectChain = httpMeta.RedirectChain
	}

	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return em
	}

	// Extract readability metadata
	em.Author = article.Byline
	em.Excerpt = article.Excerpt
	em.SiteName = article.SiteName
	if article.PublishedTime != nil {
		em.PublishedTime = article.PublishedTime.Format("2006-01-02")
	}
	em.Favicon = article.Favicon
	em.Image = article.Image

	// Domain type detection
	em.DomainType = detectDomainType(parsedURL)

	// Country detection from TLD
	em.Country = detectCountry(parsedURL)

	// Site category detection
	em.DomainCategory = detectCategory(parsedURL, em.DomainType)

	// Academic signal detection
	em.detectAcademicSignals(parsedURL, content)

	// Calculate overall confidence score
	em.Confidence = em.calculateConfidence()

	return em
}

// detectDomainType identifies domain classification
func detectDomainType(u *url.URL) string {
	host := strings.ToLower(u.Host)

	// Government domains
	if strings.HasSuffix(host, ".gov") || strings.HasSuffix(host, ".mil") {
		return "gov"
	}

	// Educational domains
	if strings.HasSuffix(host, ".edu") {
		return "edu"
	}

	// Academic/research domains
	academicDomains := []string{
		"arxiv.org", "doi.org", "pubmed.ncbi.nlm.nih.gov",
		"scholar.google.com", "researchgate.net", "academia.edu",
		"biorxiv.org", "medrxiv.org", "ssrn.com",
	}
	for _, domain := range academicDomains {
		if strings.Contains(host, domain) {
			return "academic"
		}
	}

	// Mobile sites
	if strings.HasPrefix(host, "m.") || strings.HasPrefix(host, "mobile.") {
		return "mobile"
	}

	return "commercial"
}

// detectCountry extracts country from TLD
func detectCountry(u *url.URL) string {
	host := strings.ToLower(u.Host)
	parts := strings.Split(host, ".")

	if len(parts) < 2 {
		return "unknown"
	}

	tld := parts[len(parts)-1]

	// Common country TLDs
	countries := map[string]string{
		"uk": "uk", "de": "de", "fr": "fr", "jp": "jp", "cn": "cn",
		"au": "au", "ca": "ca", "in": "in", "br": "br", "ru": "ru",
		"it": "it", "es": "es", "nl": "nl", "se": "se", "ch": "ch",
	}

	if country, ok := countries[tld]; ok {
		return country
	}

	// US implied for .gov, .edu, .com without country TLD
	if tld == "gov" || tld == "edu" || tld == "mil" {
		return "us"
	}

	return "unknown"
}

// detectCategory determines site category from URL patterns
func detectCategory(u *url.URL, domainType string) string {
	host := strings.ToLower(u.Host)
	path := strings.ToLower(u.Path)

	// Government/Health
	if domainType == "gov" {
		if strings.Contains(host, "health") || strings.Contains(host, "cdc") ||
			strings.Contains(host, "nih") || strings.Contains(host, "fda") {
			return "gov/health"
		}
		return "gov/general"
	}

	// Academic/AI
	if domainType == "academic" || domainType == "edu" {
		if strings.Contains(path, "/ai/") || strings.Contains(path, "/ml/") ||
			strings.Contains(host, "ai.") {
			return "academic/ai"
		}
		return "academic/general"
	}

	// Documentation/API
	if strings.Contains(host, "docs.") || strings.Contains(host, "documentation.") ||
		strings.Contains(path, "/docs/") || strings.Contains(path, "/documentation/") {
		return "docs/api"
	}
	if strings.Contains(host, "api.") || strings.Contains(path, "/api/") {
		return "docs/api"
	}

	// Blog
	if strings.Contains(host, "blog.") || strings.Contains(path, "/blog/") {
		return "blog"
	}

	// News/Tech
	newsDomains := []string{"techcrunch", "wired", "arstechnica", "theverge", "hacker", "news"}
	for _, newsDomain := range newsDomains {
		if strings.Contains(host, newsDomain) {
			return "news/tech"
		}
	}

	return "general"
}

// detectAcademicSignals scans content for academic indicators
func (em *EnrichedMetadata) detectAcademicSignals(u *url.URL, content string) {
	lowerContent := strings.ToLower(content)

	// DOI pattern detection: 10.xxxx/...
	doiPattern := regexp.MustCompile(`10\.\d{4,}/[^\s]+`)
	if matches := doiPattern.FindString(content); matches != "" {
		em.HasDOI = true
		em.DOIPattern = matches
	}

	// ArXiv detection
	arxivPattern := regexp.MustCompile(`arXiv:(\d{4}\.\d{4,5})`)
	if matches := arxivPattern.FindStringSubmatch(content); len(matches) > 1 {
		em.HasArXiv = true
		em.ArXivID = matches[1]
	}

	// LaTeX markers
	latexMarkers := []string{"\\begin{", "\\end{", "\\cite{", "\\ref{", "\\label{"}
	for _, marker := range latexMarkers {
		if strings.Contains(content, marker) {
			em.HasLaTeX = true
			break
		}
	}

	// Citation indicators
	citationMarkers := []string{"et al.", "et al ", "[1]", "[2]", "(1)", "(2)"}
	citationCount := 0
	for _, marker := range citationMarkers {
		if strings.Contains(lowerContent, marker) {
			citationCount++
		}
	}
	em.HasCitations = citationCount >= 2

	// References section
	if strings.Contains(lowerContent, "references") || strings.Contains(lowerContent, "bibliography") {
		em.HasReferences = true
	}

	// Abstract section
	if strings.Contains(lowerContent, "abstract") {
		em.HasAbstract = true
	}

	// Calculate academic score (0-10)
	score := 0.0
	if em.HasDOI {
		score += 3.0
	}
	if em.HasArXiv {
		score += 3.0
	}
	if em.HasLaTeX {
		score += 1.5
	}
	if em.HasCitations {
		score += 1.0
	}
	if em.HasReferences {
		score += 1.0
	}
	if em.HasAbstract {
		score += 0.5
	}

	em.AcademicScore = score
}

// calculateConfidence computes overall confidence (0-10) based on signal strength
func (em *EnrichedMetadata) calculateConfidence() float64 {
	confidence := 5.0 // baseline

	// Strong domain signals
	switch em.DomainType {
	case "gov", "edu":
		confidence += 2.0
	case "academic":
		confidence += 3.0
	case "mobile":
		confidence += 1.0
	}

	// Academic signals boost confidence
	if em.AcademicScore > 0 {
		confidence += em.AcademicScore * 0.3 // Scale academic score
	}

	// Metadata presence boosts confidence
	if em.Author != "" {
		confidence += 0.5
	}
	if em.PublishedTime != "" {
		confidence += 0.5
	}
	if em.SiteName != "" {
		confidence += 0.3
	}

	// Cap at 10
	if confidence > 10.0 {
		confidence = 10.0
	}

	return confidence
}

// ContentTypeResult represents the detected content type classification.
type ContentTypeResult struct {
	ContentType    string  // academic, docs, wiki, news, repo, blog, landing, unknown
	ContentSubtype string  // arxiv-paper, api-docs, reference, etc.
	Confidence     float64 // 0-10 confidence score
}

// DetectContentType classifies page content type based on URL, title, and content patterns.
func DetectContentType(rawURL, title, content string) ContentTypeResult {
	result := ContentTypeResult{
		ContentType: "unknown",
		Confidence:  5.0,
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return result
	}

	host := strings.ToLower(parsedURL.Host)
	path := strings.ToLower(parsedURL.Path)
	lowerTitle := strings.ToLower(title)
	lowerContent := strings.ToLower(content)

	// Academic detection (highest priority for research content)
	if detectAcademic(host, path, lowerTitle, lowerContent) {
		result.ContentType = "academic"
		result.Confidence = 9.0

		// Detect subtype
		if strings.Contains(host, "arxiv.org") {
			result.ContentSubtype = "arxiv-paper"
		} else if strings.Contains(host, "pubmed") {
			result.ContentSubtype = "pubmed-article"
		} else if strings.Contains(host, "doi.org") {
			result.ContentSubtype = "doi-reference"
		} else if strings.Contains(lowerContent, "abstract") && strings.Contains(lowerContent, "references") {
			result.ContentSubtype = "research-paper"
		} else {
			result.ContentSubtype = "academic-general"
		}
		return result
	}

	// Documentation detection
	if detectDocs(host, path, lowerTitle, lowerContent) {
		result.ContentType = "docs"
		result.Confidence = 8.5

		// Detect subtype
		if strings.Contains(lowerTitle, "api") || strings.Contains(path, "/api/") {
			result.ContentSubtype = "api-docs"
		} else if strings.Contains(lowerTitle, "reference") {
			result.ContentSubtype = "reference"
		} else if strings.Contains(lowerTitle, "tutorial") || strings.Contains(lowerTitle, "guide") {
			result.ContentSubtype = "tutorial"
		} else {
			result.ContentSubtype = "general-docs"
		}
		return result
	}

	// Wikipedia detection
	if detectWiki(host, path, lowerContent) {
		result.ContentType = "wiki"
		result.ContentSubtype = "wikipedia"
		result.Confidence = 9.5
		return result
	}

	// Repository detection (GitHub, GitLab, etc.)
	if detectRepo(host, path) {
		result.ContentType = "repo"
		result.Confidence = 8.0

		if strings.Contains(host, "github.com") {
			result.ContentSubtype = "github"
		} else if strings.Contains(host, "gitlab.com") {
			result.ContentSubtype = "gitlab"
		} else {
			result.ContentSubtype = "git-repository"
		}
		return result
	}

	// Blog detection
	if detectBlog(host, path, lowerContent) {
		result.ContentType = "blog"
		result.ContentSubtype = "blog-post"
		result.Confidence = 7.5
		return result
	}

	// News detection
	if detectNews(host, lowerTitle) {
		result.ContentType = "news"
		result.Confidence = 8.0

		if strings.Contains(host, "tech") {
			result.ContentSubtype = "tech-news"
		} else {
			result.ContentSubtype = "news-article"
		}
		return result
	}

	// Landing page detection (low content, marketing-focused)
	if detectLanding(lowerContent) {
		result.ContentType = "landing"
		result.ContentSubtype = "marketing"
		result.Confidence = 6.0
		return result
	}

	// Default: unknown with medium-low confidence
	result.Confidence = 4.0
	return result
}

// detectAcademic checks for academic paper patterns
func detectAcademic(host, path, title, content string) bool {
	// URL-based detection
	academicHosts := []string{
		"arxiv.org", "doi.org", "pubmed", "scholar.google",
		"researchgate.net", "academia.edu", "biorxiv.org",
		"medrxiv.org", "ssrn.com",
	}
	for _, ah := range academicHosts {
		if strings.Contains(host, ah) {
			return true
		}
	}

	// Path patterns
	if strings.Contains(path, "/abs/") || strings.Contains(path, "/paper/") {
		return true
	}

	// Title patterns
	titlePatterns := []string{"abstract", "arxiv:", "doi:"}
	for _, pattern := range titlePatterns {
		if strings.Contains(title, pattern) {
			return true
		}
	}

	// Strong academic signals in content
	academicSignals := 0
	if strings.Contains(content, "abstract") {
		academicSignals++
	}
	if strings.Contains(content, "references") || strings.Contains(content, "bibliography") {
		academicSignals++
	}
	if regexp.MustCompile(`10\.\d{4,}/`).MatchString(content) { // DOI pattern
		academicSignals++
	}
	if strings.Contains(content, "et al.") {
		academicSignals++
	}

	return academicSignals >= 3
}

// detectDocs checks for documentation patterns
func detectDocs(host, path, title, content string) bool {
	// URL-based detection
	if strings.Contains(host, "docs.") || strings.Contains(host, "documentation.") {
		return true
	}
	if strings.Contains(path, "/docs/") || strings.Contains(path, "/documentation/") {
		return true
	}
	if strings.Contains(path, "/reference/") || strings.Contains(path, "/manual/") {
		return true
	}

	// Title patterns
	docsTitlePatterns := []string{
		"documentation", "api reference", "getting started",
		"user guide", "developer guide", "manual",
	}
	for _, pattern := range docsTitlePatterns {
		if strings.Contains(title, pattern) {
			return true
		}
	}

	// Content patterns (code examples + structured sections)
	hasCodeBlocks := strings.Count(content, "```") >= 2 || strings.Count(content, "<code>") >= 3
	hasSections := strings.Count(content, "##") >= 3 || strings.Count(content, "<h2") >= 3

	return hasCodeBlocks && hasSections
}

// detectWiki checks for Wikipedia or wiki-style content
func detectWiki(host, path, content string) bool {
	// Wikipedia domains
	if strings.Contains(host, "wikipedia.org") {
		return true
	}

	// Path pattern
	if strings.Contains(path, "/wiki/") {
		return true
	}

	// Infobox detection (strong Wikipedia signal)
	if strings.Contains(content, "infobox") || strings.Contains(content, "class=\"infobox\"") {
		return true
	}

	return false
}

// detectRepo checks for code repository patterns
func detectRepo(host, path string) bool {
	repoHosts := []string{"github.com", "gitlab.com", "bitbucket.org"}
	for _, rh := range repoHosts {
		if strings.Contains(host, rh) {
			return true
		}
	}
	return false
}

// detectBlog checks for blog patterns
func detectBlog(host, path, content string) bool {
	// URL-based detection
	if strings.Contains(host, "blog.") || strings.Contains(path, "/blog/") {
		return true
	}

	// Blog platforms
	blogPlatforms := []string{"medium.com", "substack.com", "wordpress.com", "blogger.com"}
	for _, bp := range blogPlatforms {
		if strings.Contains(host, bp) {
			return true
		}
	}

	// Author byline + published date (common blog pattern)
	hasAuthor := strings.Contains(content, "by ") || strings.Contains(content, "author")
	hasDate := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).MatchString(content)

	return hasAuthor && hasDate
}

// detectNews checks for news article patterns
func detectNews(host, title string) bool {
	newsDomains := []string{
		"techcrunch", "wired", "arstechnica", "theverge",
		"reuters", "bbc", "cnn", "nytimes", "wsj",
		"bloomberg", "forbes",
	}
	for _, nd := range newsDomains {
		if strings.Contains(host, nd) {
			return true
		}
	}

	// News-style headlines (all caps words, breaking, exclusive)
	newsPatterns := []string{"breaking:", "exclusive:", "report:", "update:"}
	for _, pattern := range newsPatterns {
		if strings.Contains(title, pattern) {
			return true
		}
	}

	return false
}

// detectLanding checks for landing page patterns
func detectLanding(content string) bool {
	// Landing pages tend to have:
	// - Low word count
	// - High button/CTA count
	// - Minimal actual content

	wordCount := len(strings.Fields(content))
	ctaCount := strings.Count(content, "sign up") + strings.Count(content, "get started") +
		strings.Count(content, "try free") + strings.Count(content, "buy now")

	// Low content + high CTAs = landing page
	return wordCount < 500 && ctaCount >= 2
}
