package manifest

// SummaryManifest represents the structure of the summary JSON file.
// It provides a lightweight overview of all fetched URLs, their status,
// and top keywords without requiring LLMs to read full JSON files.
type SummaryManifest struct {
	GeneratedAt       string       `json:"generated_at"`
	TotalURLs         int          `json:"total_urls"`
	Successful        int          `json:"successful"`
	Failed            int          `json:"failed"`
	AggregateKeywords []string     `json:"aggregate_keywords"`
	Results           []URLSummary `json:"results"`
}

// URLSummary represents summary information for a single URL.
// Includes status, file path, extraction quality, and top keywords.
type URLSummary struct {
	URL               string   `json:"url"`
	FilePath          string   `json:"file_path,omitempty"`
	Status            string   `json:"status"` // "success" or "error"
	ErrorType         string   `json:"error_type,omitempty"`
	ErrorMessage      string   `json:"error_message,omitempty"`
	SizeBytes         int64    `json:"size_bytes,omitempty"`
	WordCount         int      `json:"word_count,omitempty"`
	EstimatedTokens   int      `json:"estimated_tokens,omitempty"`
	ExtractionQuality string   `json:"extraction_quality,omitempty"`
	TopKeywords       []string `json:"top_keywords,omitempty"`
}
