package models

type ParseMode int

const (
	ParseModeMinimal ParseMode = iota // NEW DEFAULT: metadata only, no content parsing
	ParseModeCheap                    // Basic flat parsing
	ParseModeFull                     // Full hierarchical parsing
)

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

