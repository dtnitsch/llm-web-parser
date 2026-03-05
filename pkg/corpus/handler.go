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
	// If no filter provided, show helpful examples instead of erroring
	if req.Filter == "" {
		return models.Response{
			Verb:       VerbQUERY,
			Data:       nil,
			Confidence: 0.0,
			Coverage:   0.0,
			Unknowns:   []string{},
			Error: &models.ErrorInfo{
				Type:             "missing_filter",
				Message:          generateQueryHelp(req.Session),
				SuggestedActions: []string{},
			},
		}
	}

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
				SuggestedActions: []string{"Ensure database is initialized", "Run 'llm-web-parser db init' if needed"},
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

// generateQueryHelp generates LLM-friendly help text with query examples.
func generateQueryHelp(sessionID int) string {
	sessionStr := ""
	if sessionID > 0 {
		sessionStr = fmt.Sprintf(" --session=%d", sessionID)
	}

	return fmt.Sprintf(`💡 No filter specified. Here's what you can query:

Content types (academic, docs, wiki, news, blog, repo):
  llm-web-parser corpus query%s --filter="content_type=academic"       # Research papers

Boolean features extracted during parsing (has_code_examples, has_abstract, has_toc, has_infobox):
  llm-web-parser corpus query%s --filter="has_code_examples"           # URLs with code blocks

Numeric metrics from parsed content (citation_count, section_count, code_block_count, detection_confidence):
  llm-web-parser corpus query%s --filter="citation_count>=20"          # Highly cited papers (>=20 citations)

Search by keyword (use any word from 'corpus extract'):
  llm-web-parser corpus query%s --filter="keyword:api"                 # URLs about "api"

Combine with AND/OR:
  llm-web-parser corpus query%s --filter="has_code_examples AND keyword:python"
  llm-web-parser corpus query%s --filter="content_type=academic OR content_type=docs"

Where this data comes from:
  - Extracted during 'llm-web-parser fetch --urls "..."'
  - Metadata from HTML parsing (meta tags, structure)
  - Citations/code blocks from content parsing
  - Keywords from text analysis (run 'corpus extract')

Run 'llm-web-parser corpus query --help' for full field reference.`,
		sessionStr, sessionStr, sessionStr, sessionStr,
		sessionStr, sessionStr)
}
