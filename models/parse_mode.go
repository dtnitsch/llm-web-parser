package models

// ParseMode represents the depth of content parsing to perform.
type ParseMode int

const (
	// ParseModeMinimal extracts metadata only, no content parsing.
	ParseModeMinimal ParseMode = iota
	ParseModeCheap                    // Basic flat parsing
	ParseModeFull                     // Full hierarchical parsing
)

// ResolveParseMode determines the appropriate parse mode from a request.
func ResolveParseMode(req ParseRequest) (ParseMode) {
	if req.Mode == 0 {
		return ParseModeMinimal // NEW: Default to minimal
	}

	// Escalate automatically if needed
	if req.RequireCitations {
		return ParseModeFull
	}

	return req.Mode
}

