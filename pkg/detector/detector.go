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
