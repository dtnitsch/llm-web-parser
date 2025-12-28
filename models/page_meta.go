package models

type PageMetadata struct {
	// Classification
	ContentType string `json:"content_type"` // article, documentation, blog, landing, forum, unknown
	Language    string `json:"language"`     // ISO-639-1 if possible (e.g. "en")

	// Size & cost signals
	WordCount        int     `json:"word_count"`
	EstimatedReadMin float64 `json:"estimated_read_min"`

	// Structural signals
	SectionCount int `json:"section_count"`
	BlockCount   int `json:"block_count"`

	Computed bool `json:"computed"`

	// LLM signals
	ExtractionMode     string  `json:"extraction_mode"`     // "cheap" | "full"
	ExtractionQuality  string  `json:"extraction_quality"`  // "ok" | "low"
}

