package fetch

import (
	"github.com/dtnitsch/llm-web-parser/models"
)

type Job struct {
	URL       string
	ParseMode models.ParseMode
}

// Result holds the outcome of a processed job.
type Result struct {
	URL           string
	FilePath      string
	Page          *models.Page
	Error         error
	ErrorType     string
	WordCounts    map[string]int
	FileSizeBytes int64
}

// ResultOutput is the structured output for a single URL.
type ResultOutput struct {
	URL       string `json:"url"`
	FilePath  string `json:"file_path,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"error_type,omitempty"`
}

// ResultSummary holds detailed summary data for a single processed URL.
type ResultSummary struct {
	URL               string         `json:"url"`
	FilePath          string         `json:"file_path,omitempty"`
	Status            string         `json:"status"`
	Error             string         `json:"error,omitempty"`
	FileSizeBytes     int64          `json:"file_size_bytes,omitempty"`
	EstimatedTokens   int            `json:"estimated_tokens,omitempty"`
	ContentType       string         `json:"content_type,omitempty"`
	ExtractionQuality string         `json:"extraction_quality,omitempty"`
	ConfidenceDist    map[string]int `json:"confidence_distribution,omitempty"`
	BlockTypeDist     map[string]int `json:"block_type_distribution,omitempty"`
}

// FinalOutput is the structured output for the entire run.
type FinalOutput struct {
	Status  string      `json:"status"`
	Results interface{} `json:"results"`
	Stats   Stats       `json:"stats"`
}

// Stats provides summary statistics for the run.
type Stats struct {
	TotalURLs        int      `json:"total_urls"`
	Successful       int      `json:"successful"`
	Failed           int      `json:"failed"`
	TotalTimeSeconds float64  `json:"total_time_seconds"`
	TopKeywords      []string `json:"top_keywords,omitempty"`
}

// ResultSummaryTerse is the token-optimized v2 format with abbreviated field names.
type ResultSummaryTerse struct {
	URL               string         `json:"u"`
	FilePath          string         `json:"p,omitempty"`
	Status            int            `json:"s"` // 0=success, 1=failed
	Error             string         `json:"e,omitempty"`
	FileSizeBytes     int64          `json:"sz,omitempty"`
	EstimatedTokens   int            `json:"tk,omitempty"`
	ContentType       string         `json:"ct,omitempty"` // l=landing, a=article, d=docs, u=unknown
	ExtractionQuality int            `json:"q,omitempty"`  // 1=ok, 0=low, -1=degraded
	ConfidenceDist    [3]int         `json:"cd,omitempty"` // [high, medium, low] fixed order
	BlockTypeDist     map[string]int `json:"bd,omitempty"`
}

// StatsTerse is the token-optimized v2 stats format.
type StatsTerse struct {
	Total    int      `json:"t"`
	Success  int      `json:"ok"`
	Failed   int      `json:"f"`
	Time     float64  `json:"ts"`
	Keywords []string `json:"kw,omitempty"`
}

// FinalOutputTerse is the v2 terse output wrapper.
type FinalOutputTerse struct {
	Status  string               `json:"s"`
	Results []ResultSummaryTerse `json:"r"`
	Stats   StatsTerse           `json:"st"`
}

// SummaryIndex is the ultra-minimal, scannable index format (~150 bytes/URL).
// Only includes successful fetches (200, 301).
type SummaryIndex struct {
	URL    string  `yaml:"url"`
	Cat    string  `yaml:"cat"`  // domain_category
	Conf   float64 `yaml:"conf"` // confidence 0-10
	Title  string  `yaml:"title,omitempty"`
	Desc   string  `yaml:"desc,omitempty"`   // excerpt
	Tokens int     `yaml:"tokens,omitempty"` // estimated_tokens
}

// SummaryDetails contains full enriched metadata for decision making (~400 bytes/URL).
// Includes all URLs (successful and failed).
type SummaryDetails struct {
	URL        string `yaml:"url"`
	URLID      int64  `yaml:"url_id,omitempty"`
	FilePath   string `yaml:"file_path,omitempty"`
	Status     string `yaml:"status"` // success, failed
	StatusCode int    `yaml:"status_code,omitempty"`
	Error      string `yaml:"error,omitempty"`

	// Basic metadata
	Title       string `yaml:"title,omitempty"`
	Excerpt     string `yaml:"excerpt,omitempty"`
	SiteName    string `yaml:"site_name,omitempty"`
	Author      string `yaml:"author,omitempty"`
	PublishedAt string `yaml:"published_at,omitempty"`

	// Smart detection
	DomainType     string  `yaml:"domain_type,omitempty"`
	DomainCategory string  `yaml:"domain_category,omitempty"`
	Country        string  `yaml:"country,omitempty"`
	Confidence     float64 `yaml:"confidence,omitempty"`

	// Academic signals
	AcademicScore float64 `yaml:"academic_score,omitempty"`
	HasDOI        bool    `yaml:"has_doi,omitempty"`
	HasArXiv      bool    `yaml:"has_arxiv,omitempty"`
	DOI           string  `yaml:"doi,omitempty"`
	ArXivID       string  `yaml:"arxiv_id,omitempty"`
	HasLaTeX      bool    `yaml:"has_latex,omitempty"`
	HasCitations  bool    `yaml:"has_citations,omitempty"`
	HasReferences bool    `yaml:"has_references,omitempty"`
	HasAbstract   bool    `yaml:"has_abstract,omitempty"`

	// Content metrics
	WordCount          int     `yaml:"word_count,omitempty"`
	EstimatedTokens    int     `yaml:"estimated_tokens,omitempty"`
	ReadTimeMin        float64 `yaml:"read_time_min,omitempty"`
	Language           string  `yaml:"language,omitempty"`
	LanguageConfidence float64 `yaml:"language_confidence,omitempty"`
	ContentType        string  `yaml:"content_type,omitempty"`
	ExtractionMode     string  `yaml:"extraction_mode,omitempty"`
	SectionCount       int     `yaml:"section_count,omitempty"`
	BlockCount         int     `yaml:"block_count,omitempty"`

	// Visual metadata (boolean/count only, not URLs)
	HasFavicon bool `yaml:"has_favicon,omitempty"`
	ImageCount int  `yaml:"image_count,omitempty"`

	// HTTP metadata
	FinalURL        string   `yaml:"final_url,omitempty"`
	RedirectChain   []string `yaml:"redirect_chain,omitempty"`
	HTTPContentType string   `yaml:"http_content_type,omitempty"`
}

// FailedURL represents a URL that failed during processing.
type FailedURL struct {
	URL          string `yaml:"url"`
	StatusCode   int    `yaml:"status_code"` // 0 for network errors
	ErrorType    string `yaml:"error_type"`  // http_error, network_error, parse_error, timeout
	ErrorMessage string `yaml:"error_message"`
}

// FailedURLs wraps the list of failed URLs for YAML output.
type FailedURLs struct {
	FailedURLs []FailedURL `yaml:"failed_urls"`
}

// toTerseStatus converts status string to int (0=success, 1=failed).
