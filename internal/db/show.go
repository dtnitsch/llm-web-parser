package db

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func ShowAction(c *cli.Context) error {
	if c.NArg() == 0 {
		fmt.Println("Error: URL ID or URL required")
		fmt.Println()
		cli.ShowSubcommandHelp(c)
		return nil
	}

	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	manager, err := artifact_manager.NewManager(artifact_manager.DefaultBaseDir, 0)
	if err != nil {
		return fmt.Errorf("failed to initialize artifact manager: %w", err)
	}

	arg := c.Args().First()

	// Check for format flag (declare at function scope)
	outputFormat := strings.ToLower(c.String("format"))

	// Check if argument contains comma (batch mode)
	if strings.Contains(arg, ",") {
		ids := strings.Split(arg, ",")
		results := make([]string, 0, len(ids))

		for _, id := range ids {
			id = strings.TrimSpace(id)
			urlID, err := ResolveURLID(id, database)
			if err != nil {
				return fmt.Errorf("failed to resolve ID %s: %w", id, err)
			}

			data, found, err := manager.GetParsedJSONByID(urlID)
			if err != nil {
				return fmt.Errorf("failed to read parsed content for URL ID %d: %w", urlID, err)
			}
			if !found {
				url, _ := database.GetURLByID(urlID)
				return fmt.Errorf("parsed content not found for URL ID %d (%s)\n\nThis URL may not have been fetched yet. Try:\n  llm-web-parser fetch --urls \"%s\"", urlID, url, url)
			}

			results = append(results, string(data))
		}

		// Print results based on format
		if outputFormat == "json" {
			// For JSON batch mode, output as array
			fmt.Println("[")
			for i, result := range results {
				if i > 0 {
					fmt.Println(",")
				}
				fmt.Print(result)
			}
			fmt.Println("\n]")
		} else {
			// YAML format from storage (default)
			fmt.Println("# YAML compact mode: Only non-null/non-default fields shown")
			for i, result := range results {
				if i > 0 {
					fmt.Print("\n---\n\n")
				}
				fmt.Print(result)
			}
		}
		return nil
	}

	// Single URL/ID mode
	urlID, err := ResolveURLID(arg, database)
	if err != nil {
		return err
	}

	data, found, err := manager.GetParsedJSONByID(urlID)
	if err != nil {
		return fmt.Errorf("failed to read parsed content: %w", err)
	}
	if !found {
		url, _ := database.GetURLByID(urlID)
		return fmt.Errorf("parsed content not found for URL ID %d (%s)\n\nThis URL may not have been fetched yet. Try:\n  lwp fetch --urls \"%s\"", urlID, url, url)
	}

	// Always parse YAML to apply compact marshaling and enable filters
	var page models.Page
	if err := yaml.Unmarshal(data, &page); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Check for filter flags
	outlineMode := c.Bool("outline")
	onlyTypes := c.String("only")
	grepPattern := c.String("grep")
	grepContext := c.Int("context")
	showMetadata := c.Bool("metadata")
	showMetadataFull := c.Bool("metadata-full")
	// outputFormat already declared above

	// Apply outline filter (special output)
	if outlineMode {
		fmt.Print(filterOutline(&page, urlID))
		return nil
	}

	// Store original page for "no results" message
	originalPage := page

	// Apply type filter
	if onlyTypes != "" {
		filtered, err := filterByType(&page, onlyTypes)
		if err != nil {
			return err
		}
		page = *filtered
	}

	// Apply grep filter
	if grepPattern != "" {
		// Check if pattern contains | (OR operator) - if so, group by match
		if strings.Contains(grepPattern, "|") {
			// Grouped grep mode: show results organized by sub-pattern
			err := displayGroupedGrep(&originalPage, grepPattern, grepContext, urlID, outputFormat)
			return err
		}

		// Regular grep mode: single pattern
		filtered, err := filterByGrep(&page, grepPattern, grepContext)
		if err != nil {
			return err
		}
		page = *filtered
	}

	// Check if filters returned empty results
	isEmpty := false
	if len(page.FlatContent) == 0 && len(page.Content) == 0 {
		isEmpty = true
	}

	// Show helpful message if no results found
	if isEmpty && (onlyTypes != "" || grepPattern != "") {
		filterType := ""
		filterValue := ""
		if onlyTypes != "" {
			filterType = "only"
			filterValue = onlyTypes
		} else if grepPattern != "" {
			filterType = "grep"
			filterValue = grepPattern
		}
		fmt.Print(generateNoResultsMessage(filterType, filterValue, &originalPage, urlID))
		return nil
	}

	// Determine metadata display mode
	var metadataToShow interface{}
	if showMetadataFull {
		// Show all metadata fields
		metadataToShow = page.Metadata
	} else if showMetadata {
		// Show only 5 essential fields (truthy)
		metadataToShow = getCompactMetadata(page.Metadata)
	}
	// else: no metadata (metadataToShow stays nil)

	// Re-marshal based on requested format
	var output []byte
	if outputFormat == "markdown" {
		// Convert to markdown format
		output = []byte(convertToMarkdown(&page, urlID))
		fmt.Print(string(output))
		return nil
	} else if outputFormat == "csv" {
		// Convert to CSV format
		output = []byte(convertToCSV(&page, urlID))
		fmt.Print(string(output))
		return nil
	} else if outputFormat == "json" {
		if metadataToShow != nil {
			// With metadata
			outputStruct := struct {
				URLID       int64                  `json:"url_id"`
				URL         string                 `json:"url"`
				Title       string                 `json:"title"`
				Content     []models.Section       `json:"content,omitempty"`
				FlatContent []models.ContentBlock  `json:"flat_content,omitempty"`
				Metadata    interface{}            `json:"metadata,omitempty"`
			}{
				URLID:       urlID,
				URL:         page.URL,
				Title:       page.Title,
				Content:     page.Content,
				FlatContent: page.FlatContent,
				Metadata:    metadataToShow,
			}
			output, err = json.MarshalIndent(&outputStruct, "", "  ")
		} else {
			// Without metadata
			outputStruct := struct {
				URLID       int64                  `json:"url_id"`
				URL         string                 `json:"url"`
				Title       string                 `json:"title"`
				Content     []models.Section       `json:"content,omitempty"`
				FlatContent []models.ContentBlock  `json:"flat_content,omitempty"`
			}{
				URLID:       urlID,
				URL:         page.URL,
				Title:       page.Title,
				Content:     page.Content,
				FlatContent: page.FlatContent,
			}
			output, err = json.MarshalIndent(&outputStruct, "", "  ")
		}
		if err != nil {
			return fmt.Errorf("failed to marshal content as JSON: %w", err)
		}
		fmt.Print(string(output))
	} else {
		// Default to YAML
		if metadataToShow != nil {
			// With metadata
			outputStruct := struct {
				URL         string                 `yaml:"url"`
				Title       string                 `yaml:"title"`
				Content     []models.Section       `yaml:"content,omitempty"`
				FlatContent []models.ContentBlock  `yaml:"flatcontent,omitempty"`
				Metadata    interface{}            `yaml:"metadata,omitempty"`
			}{
				URL:         page.URL,
				Title:       page.Title,
				Content:     page.Content,
				FlatContent: page.FlatContent,
				Metadata:    metadataToShow,
			}
			output, err = yaml.Marshal(&outputStruct)
		} else {
			// Without metadata
			outputStruct := struct {
				URL         string                 `yaml:"url"`
				Title       string                 `yaml:"title"`
				Content     []models.Section       `yaml:"content,omitempty"`
				FlatContent []models.ContentBlock  `yaml:"flatcontent,omitempty"`
			}{
				URL:         page.URL,
				Title:       page.Title,
				Content:     page.Content,
				FlatContent: page.FlatContent,
			}
			output, err = yaml.Marshal(&outputStruct)
		}
		if err != nil {
			return fmt.Errorf("failed to marshal content as YAML: %w", err)
		}

		// Print header with url_id and keywords for consistency with --outline
		if onlyTypes != "" || grepPattern != "" {
			fmt.Println("# YAML compact mode: Only non-null/non-default fields shown (filtered)")
		} else {
			fmt.Println("# YAML compact mode: Only non-null/non-default fields shown")
		}
		fmt.Printf("# url_id: %d\n", urlID)

		// Show keywords if available
		if len(page.Metadata.MetaKeywords) > 0 {
			display := page.Metadata.MetaKeywords
			if len(display) > 5 {
				display = display[:5]
			}
			fmt.Printf("# meta_keywords: %s (from site)\n", strings.Join(display, ", "))
		}
		if len(page.Metadata.TopKeywords) > 0 {
			display := page.Metadata.TopKeywords
			if len(display) > 5 {
				display = display[:5]
			}
			fmt.Printf("# top_keywords: %s (extracted)\n", strings.Join(display, ", "))
		}
		fmt.Println()

		fmt.Print(string(output))

		// Show metadata tip if no metadata flags used
		if !showMetadata && !showMetadataFull {
			truthyCount := countTruthyMetadata(page.Metadata)
			fmt.Printf("\nTip: %d metadata fields available (--metadata) | All fields (--metadata-full)\n", truthyCount)
		}
	}
	return nil
}

// rawAction shows raw HTML for a URL by ID or URL
func RawAction(c *cli.Context) error {
	if c.NArg() == 0 {
		fmt.Println("Error: URL ID or URL required")
		fmt.Println()
		cli.ShowSubcommandHelp(c)
		return nil
	}

	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	manager, err := artifact_manager.NewManager(artifact_manager.DefaultBaseDir, 0)
	if err != nil {
		return fmt.Errorf("failed to initialize artifact manager: %w", err)
	}

	arg := c.Args().First()

	// Check if argument contains comma (batch mode)
	if strings.Contains(arg, ",") {
		ids := strings.Split(arg, ",")

		for i, id := range ids {
			id = strings.TrimSpace(id)
			urlID, err := ResolveURLID(id, database)
			if err != nil {
				return fmt.Errorf("failed to resolve ID %s: %w", id, err)
			}

			data, found, err := manager.GetRawHTMLByID(urlID)
			if err != nil {
				return fmt.Errorf("failed to read raw HTML for URL ID %d: %w", urlID, err)
			}
			if !found {
				url, _ := database.GetURLByID(urlID)
				return fmt.Errorf("raw HTML not found for URL ID %d (%s)\n\nThis URL may not have been fetched yet. Try:\n  lwp fetch --urls \"%s\"", urlID, url, url)
			}

			if i > 0 {
				fmt.Print("\n<!-- ===== Next URL ===== -->\n\n")
			}
			fmt.Print(string(data))
		}
		return nil
	}

	// Single URL/ID mode
	urlID, err := ResolveURLID(arg, database)
	if err != nil {
		return err
	}

	data, found, err := manager.GetRawHTMLByID(urlID)
	if err != nil {
		return fmt.Errorf("failed to read raw HTML: %w", err)
	}
	if !found {
		url, _ := database.GetURLByID(urlID)
		return fmt.Errorf("raw HTML not found for URL ID %d (%s)\n\nThis URL may not have been fetched yet. Try:\n  lwp fetch --urls \"%s\"", urlID, url, url)
	}

	fmt.Print(string(data))
	return nil
}

func FindURLAction(c *cli.Context) error {
	if c.NArg() == 0 {
		fmt.Println("Error: URL required")
		fmt.Println()
		cli.ShowSubcommandHelp(c)
		return nil
	}

	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	url := c.Args().First()
	urlID, err := database.GetURLID(url)
	if err != nil {
		return fmt.Errorf("URL not found in database: %s\nNote: Only fetched URLs are tracked", url)
	}

	fmt.Printf("[#%d] %s\n", urlID, url)
	return nil
}

// filterOutline extracts headings from a Page and builds a hierarchical outline.
