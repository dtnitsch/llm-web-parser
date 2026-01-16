package models

// PageMetadata contains computed metadata about a page.
type PageMetadata struct{
	// Classification (enhanced content type detection)
	ContentType    string  `json:"content_type"`              // academic, docs, wiki, news, repo, blog, landing, unknown
	ContentSubtype string  `json:"content_subtype,omitempty"` // arxiv-paper, api-docs, reference, etc.
	Language       string  `json:"language"`                  // ISO-639-1 if possible (e.g. "en")
	LanguageConfidence float64 `json:"language_confidence,omitempty"`

	// Size & cost signals
	WordCount        int     `json:"word_count"`
	EstimatedReadMin float64 `json:"estimated_read_min"`

	// Structural signals
	SectionCount int `json:"section_count"`
	BlockCount   int `json:"block_count"`

	// Content features (for specialized extraction)
	HasInfobox      bool `json:"has_infobox,omitempty"`
	HasTOC          bool `json:"has_toc,omitempty"`
	HasCodeExamples bool `json:"has_code_examples,omitempty"`
	CitationCount   int  `json:"citation_count,omitempty"`
	CodeBlockCount  int  `json:"code_block_count,omitempty"`

	Computed bool `json:"computed"`

	// LLM signals
	ExtractionMode     string  `json:"extraction_mode"`     // "cheap" | "full"
	ExtractionQuality  string  `json:"extraction_quality"`  // "ok" | "low"

	// Readability enrichment (from go-readability)
	Author        string `json:"author,omitempty"`
	Excerpt       string `json:"excerpt,omitempty"`        // meta description
	SiteName      string `json:"site_name,omitempty"`
	PublishedTime string `json:"published_time,omitempty"` // ISO-8601 date
	Favicon       string `json:"favicon,omitempty"`
	Image         string `json:"image,omitempty"` // main image URL

	// Smart detection (from pkg/detector)
	DomainType     string  `json:"domain_type,omitempty"`     // gov, edu, academic, commercial, mobile
	DomainCategory string  `json:"domain_category,omitempty"` // gov/health, academic/ai, news/tech, docs/api, etc
	Country        string  `json:"country,omitempty"`         // TLD-based: us, uk, de, jp, etc
	Confidence     float64 `json:"confidence,omitempty"`      // 0-10 scale

	// Academic signals
	HasDOI         bool    `json:"has_doi,omitempty"`
	DOIPattern     string  `json:"doi_pattern,omitempty"`
	HasArXiv       bool    `json:"has_arxiv,omitempty"`
	ArXivID        string  `json:"arxiv_id,omitempty"`
	HasLaTeX       bool    `json:"has_latex,omitempty"`
	HasCitations   bool    `json:"has_citations,omitempty"`
	HasReferences  bool    `json:"has_references,omitempty"`
	HasAbstract    bool    `json:"has_abstract,omitempty"`
	AcademicScore  float64 `json:"academic_score,omitempty"` // 0-10

	// HTTP metadata
	StatusCode      int      `json:"status_code,omitempty"`
	HTTPContentType string   `json:"http_content_type,omitempty"`
	FinalURL        string   `json:"final_url,omitempty"` // after redirects
	RedirectChain   []string `json:"redirect_chain,omitempty"`
}

