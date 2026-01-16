package models

// Request represents a Corpus API request.
type Request struct {
	Verb        string                 `json:"verb"`
	Session     int                    `json:"session,omitempty"`
	View        string                 `json:"view,omitempty"`
	URLIDs      []int64                `json:"url_ids,omitempty"`
	Schema      string                 `json:"schema,omitempty"`      // For EXTRACT
	Filter      string                 `json:"filter,omitempty"`      // For QUERY
	Constraints map[string]interface{} `json:"constraints,omitempty"` // Verb-specific
	Format      string                 `json:"format,omitempty"`      // json, yaml, csv
}

// Response represents a Corpus API response.
type Response struct {
	Verb       string      `json:"verb"`
	Data       interface{} `json:"data"`
	Confidence float64     `json:"confidence"`
	Coverage   float64     `json:"coverage"`
	Unknowns   []string    `json:"unknowns"`
	Error      *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo provides structured error information.
type ErrorInfo struct {
	Type             string   `json:"error_type"`
	Message          string   `json:"message"`
	SuggestedActions []string `json:"suggested_actions,omitempty"`
}

// NewNotImplementedResponse creates a response for unimplemented verbs.
func NewNotImplementedResponse(verb string) Response {
	return Response{
		Verb:       verb,
		Data:       nil,
		Confidence: 0.0,
		Coverage:   0.0,
		Unknowns:   []string{},
		Error: &ErrorInfo{
			Type:             "not_implemented",
			Message:          verb + " verb not implemented yet",
			SuggestedActions: []string{"Check docs/CORPUS-API.md for implementation status"},
		},
	}
}

// NewUnknownVerbResponse creates a response for unknown verbs.
func NewUnknownVerbResponse(verb string, suggestion string) Response {
	msg := "Verb '" + verb + "' not recognized"
	if suggestion != "" {
		msg += ". Did you mean '" + suggestion + "'?"
	}

	return Response{
		Verb:       verb,
		Data:       nil,
		Confidence: 0.0,
		Coverage:   0.0,
		Unknowns:   []string{},
		Error: &ErrorInfo{
			Type:    "unknown_verb",
			Message: msg,
			SuggestedActions: []string{
				"See docs/CORPUS-API.md for valid verbs",
				"Valid verbs: ingest, extract, normalize, compare, detect, trace, score, query, delta, summarize, explain-failure",
			},
		},
	}
}
