package corpus

import (
	"fmt"

	"github.com/dtnitsch/llm-web-parser/models"
	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
)

// Handle dispatches a Corpus API request to the appropriate verb handler.
func Handle(req models.Request) models.Response {
	// Validate verb
	if !IsValidVerb(req.Verb) {
		return models.NewUnknownVerbResponse(req.Verb, suggestVerb(req.Verb))
	}

	// Dispatch to verb handler
	switch req.Verb {
	case VerbINGEST:
		return handleIngest(req)
	case VerbEXTRACT:
		return handleExtract(req)
	case VerbNORMALIZE:
		return handleNormalize(req)
	case VerbCOMPARE:
		return handleCompare(req)
	case VerbDETECT:
		return handleDetect(req)
	case VerbTRACE:
		return handleTrace(req)
	case VerbSCORE:
		return handleScore(req)
	case VerbQUERY:
		return handleQuery(req)
	case VerbDELTA:
		return handleDelta(req)
	case VerbSUMMARIZE:
		return handleSummarize(req)
	case VerbEXPLAIN:
		return handleExplain(req)
	default:
		// Should never reach here due to IsValidVerb check
		return models.NewUnknownVerbResponse(req.Verb, "")
	}
}

// Placeholder handlers - all return "NOT IMPLEMENTED YET"

func handleIngest(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbINGEST)
}

// handleExtract is implemented in extract.go

func handleNormalize(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbNORMALIZE)
}

func handleCompare(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbCOMPARE)
}

func handleDetect(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbDETECT)
}

func handleTrace(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbTRACE)
}

func handleScore(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbSCORE)
}

func handleQuery(req models.Request) models.Response {
	// Open database
	db, err := openDB()
	if err != nil {
		return models.Response{
			Verb:       VerbQUERY,
			Data:       nil,
			Confidence: 0.0,
			Coverage:   0.0,
			Unknowns:   []string{},
			Error: &models.ErrorInfo{
				Type:             "database_error",
				Message:          fmt.Sprintf("Failed to open database: %v", err),
				SuggestedActions: []string{"Ensure database is initialized", "Run 'lwp db init' if needed"},
			},
		}
	}
	defer db.Close()

	// Execute query
	resp, err := ExecuteQuery(db, req.Filter, req.Session)
	if err != nil {
		return models.Response{
			Verb:       VerbQUERY,
			Data:       nil,
			Confidence: 0.0,
			Coverage:   0.0,
			Unknowns:   []string{},
			Error: &models.ErrorInfo{
				Type:             "query_error",
				Message:          fmt.Sprintf("Query execution failed: %v", err),
				SuggestedActions: []string{"Check filter syntax", "Verify database contains data"},
			},
		}
	}

	return resp
}

func handleDelta(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbDELTA)
}

func handleSummarize(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbSUMMARIZE)
}

func handleExplain(req models.Request) models.Response {
	return models.NewNotImplementedResponse(VerbEXPLAIN)
}

// suggestVerb attempts to find a similar verb for typos.
// Simple implementation - can be enhanced later with edit distance.
func suggestVerb(verb string) string {
	// Simple prefix matching for now
	for _, v := range AllVerbs() {
		if len(verb) > 2 && len(v) > 2 {
			if verb[:2] == v[:2] {
				return v
			}
		}
	}
	return ""
}

// openDB opens the database connection.
func openDB() (*dbpkg.DB, error) {
	return dbpkg.Open()
}
