package corpus

// Verb constants for the Corpus API.
const (
	VerbINGEST    = "ingest"
	VerbEXTRACT   = "extract"
	VerbNORMALIZE = "normalize"
	VerbCOMPARE   = "compare"
	VerbDETECT    = "detect"
	VerbTRACE     = "trace"
	VerbSCORE     = "score"
	VerbQUERY     = "query"
	VerbDELTA     = "delta"
	VerbSUMMARIZE = "summarize"
	VerbEXPLAIN   = "explain-failure"
)

// AllVerbs returns a list of all valid verbs.
func AllVerbs() []string {
	return []string{
		VerbINGEST,
		VerbEXTRACT,
		VerbNORMALIZE,
		VerbCOMPARE,
		VerbDETECT,
		VerbTRACE,
		VerbSCORE,
		VerbQUERY,
		VerbDELTA,
		VerbSUMMARIZE,
		VerbEXPLAIN,
	}
}

// IsValidVerb checks if a verb is valid.
func IsValidVerb(verb string) bool {
	for _, v := range AllVerbs() {
		if v == verb {
			return true
		}
	}
	return false
}
