package corpus

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/corpus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// CorpusAction handles corpus API commands.
func CorpusAction(c *cli.Context) error {
	// Parse URL IDs from comma-separated string
	var urlIDs []int64
	if urlIDsStr := c.String("url-ids"); urlIDsStr != "" {
		parts := strings.Split(urlIDsStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid URL ID: %s", part)
			}
			urlIDs = append(urlIDs, id)
		}
	}

	// Build constraints map for verb-specific parameters
	constraints := make(map[string]interface{})
	if top := c.Int("top"); top != 0 || c.IsSet("top") {
		constraints["top"] = top
	}

	// Build request from CLI flags
	req := models.Request{
		Verb:        c.Command.Name, // extract, query, etc.
		Session:     c.Int("session"),
		View:        c.String("view"),
		Schema:      c.String("schema"),
		Filter:      c.String("filter"),
		Format:      c.String("format"),
		URLIDs:      urlIDs,
		Constraints: constraints,
	}

	// Handle the request
	resp := corpus.Handle(req)

	// Output response as YAML
	yamlBytes, err := yaml.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Print(string(yamlBytes))
	return nil
}

// SuggestAction handles corpus suggest commands.
func SuggestAction(c *cli.Context) error {
	sessionID := int64(c.Int("session"))
	if sessionID == 0 {
		return fmt.Errorf("session ID is required")
	}

	// Generate suggestions
	suggestions, err := corpus.SuggestFromSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to generate suggestions: %w", err)
	}

	fmt.Print(suggestions)
	return nil
}
