package models

type ParseMode int

const (
	ParseModeCheap ParseMode = iota
	ParseModeFull
)

func ResolveParseMode(req ParseRequest) (ParseMode) {
	if req.Mode == 0 {
		return ParseModeCheap
	}

	// Escalate automatically if needed
	if req.RequireCitations {
		return ParseModeFull
	}

	return ParseModeFull
}

