package db

import (
	"fmt"

	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/urfave/cli/v2"
)

func ResolveURLFromIDOrURL(arg string, database *dbpkg.DB) (string, error) {
	// Check if it's a numeric ID
	if id, err := fmt.Sscanf(arg, "%d", new(int64)); err == nil && id == 1 {
		// It's a number, parse it properly
		var urlID int64
		if _, err := fmt.Sscanf(arg, "%d", &urlID); err == nil {
			return database.GetURLByID(urlID)
		}
	}

	// Otherwise treat as URL
	return arg, nil
}

// ResolveURLID converts a URL ID or URL string to a URL ID
func ResolveURLID(arg string, database *dbpkg.DB) (int64, error) {
	// Check if it's a numeric ID
	var urlID int64
	if _, err := fmt.Sscanf(arg, "%d", &urlID); err == nil {
		// It's a number, verify it exists
		_, err := database.GetURLByID(urlID)
		if err != nil {
			return 0, fmt.Errorf("URL ID %d not found in database", urlID)
		}
		return urlID, nil
	}

	// Otherwise it's a URL, look up the ID
	urlID, err := database.GetURLID(arg)
	if err != nil {
		return 0, fmt.Errorf("URL not found in database: %s", arg)
	}
	return urlID, nil
}

// GetSessionIDOrLatest returns the session ID from args, or the latest session if not provided
func GetSessionIDOrLatest(c *cli.Context, database *dbpkg.DB) (int64, error) {
	if c.NArg() == 0 {
		// No session ID provided, use latest
		sessions, err := database.ListSessions(1)
		if err != nil {
			return 0, fmt.Errorf("failed to get latest session: %w", err)
		}
		if len(sessions) == 0 {
			return 0, fmt.Errorf("no sessions found. Run 'lwp fetch --urls \"...\"' first")
		}
		return sessions[0].SessionID, nil
	}

	// Parse provided session ID
	var sessionID int64
	_, err := fmt.Sscanf(c.Args().First(), "%d", &sessionID)
	if err != nil {
		return 0, fmt.Errorf("invalid session ID: %s", c.Args().First())
	}
	return sessionID, nil
}
