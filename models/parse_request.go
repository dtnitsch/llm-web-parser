package models

type ParseRequest struct {
	URL   string
	HTML  string

	// Optional hints
	Mode ParseMode `json:"mode,omitempty"`

	// Optional future knobs
	MaxDepth        int  `json:"max_depth,omitempty"`
	ExtractLinks    bool `json:"extract_links,omitempty"`
	RequireCitations bool `json:"require_citations,omitempty"`
}

