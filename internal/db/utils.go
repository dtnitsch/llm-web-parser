package db

import (
	"fmt"
	"os"
	"strings"

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
// Supports both --session flag and positional arg
func GetSessionIDOrLatest(c *cli.Context, database *dbpkg.DB) (int64, error) {
	// Detect common mistake: --session=X (equals sign)
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--session=") {
			value := strings.TrimPrefix(arg, "--session=")
			return 0, fmt.Errorf("invalid flag syntax. Use --session %s (space, not equals)\n\nExamples:\n  %s --session %s\n  %s %s",
				value, c.Command.FullName(), value, c.Command.FullName(), value)
		}
	}

	// 1. Check for --session flag first
	if c.IsSet("session") {
		sessionID := int64(c.Int("session"))
		if sessionID <= 0 {
			return 0, fmt.Errorf("invalid session ID: %d (must be > 0)", sessionID)
		}
		return sessionID, nil
	}

	// 2. Check for positional arg
	if c.NArg() > 0 {
		var sessionID int64
		_, err := fmt.Sscanf(c.Args().First(), "%d", &sessionID)
		if err != nil {
			return 0, fmt.Errorf("invalid session ID: %s", c.Args().First())
		}
		if sessionID <= 0 {
			return 0, fmt.Errorf("invalid session ID: %d (must be > 0)", sessionID)
		}
		return sessionID, nil
	}

	// 3. Check for active session
	activeSessionID := getActiveSessionFromConfig()
	if activeSessionID > 0 {
		// Verify it still exists
		_, err := database.GetSessionByID(activeSessionID)
		if err == nil {
			return activeSessionID, nil
		}
		// Active session doesn't exist anymore, fall through to latest
	}

	// 4. No session specified, use latest
	sessions, err := database.ListSessions(1)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest session: %w", err)
	}
	if len(sessions) == 0 {
		return 0, fmt.Errorf("no sessions found. Run 'lwp fetch --urls \"...\"' first")
	}
	return sessions[0].SessionID, nil
}

// getActiveSessionFromConfig reads active session from .lwp/config
func getActiveSessionFromConfig() int64 {
	configPath := ".lwp/config"
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0
	}

	// Parse as YAML with active_session field
	var config struct {
		ActiveSession int64 `yaml:"active_session"`
	}
	_, err = fmt.Sscanf(string(data), "active_session: %d", &config.ActiveSession)
	if err != nil {
		return 0
	}

	return config.ActiveSession
}
