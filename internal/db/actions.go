package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func SessionsAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	limit := c.Int("limit")
	sessions, err := database.ListSessions(limit)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	// Print table header
	fmt.Printf("%-6s %-20s %-8s %-8s %-8s %-15s %-30s\n",
		"ID", "Created", "URLs", "Success", "Failed", "Parse Mode", "Session Dir")
	fmt.Println(strings.Repeat("-", 120))

	// Print each session
	for _, s := range sessions {
		fmt.Printf("%-6d %-20s %-8d %-8d %-8d %-15s %-30s\n",
			s.SessionID,
			s.CreatedAt.Format("2006-01-02 15:04:05"),
			s.URLCount,
			s.SuccessCount,
			s.FailedCount,
			s.ParseMode,
			s.SessionDir,
		)
	}

	fmt.Printf("\nTotal: %d sessions\n", len(sessions))
	fmt.Printf("\nTip: Use 'llm-web-parser db session <id>' to see details\n")

	return nil
}

// sessionAction shows details for a specific session
func SessionAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	sessionID, err := GetSessionIDOrLatest(c, database)
	if err != nil {
		return err
	}

	// Get session info
	session, err := database.GetSessionByID(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Get URLs for this session
	urls, err := database.GetSessionURLs(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session URLs: %w", err)
	}

	// Get results for this session
	results, err := database.GetSessionResults(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session results: %w", err)
	}

	// Print session details
	fmt.Printf("Session %d\n", session.SessionID)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Created:     %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Directory:   %s\n", session.SessionDir)
	fmt.Printf("URLs:        %d total (%d success, %d failed)\n",
		session.URLCount, session.SuccessCount, session.FailedCount)
	fmt.Printf("Features:    %s\n", session.Features)
	fmt.Printf("Parse Mode:  %s\n", session.ParseMode)

	// Print URLs
	fmt.Printf("\nURLs (%d):\n", len(urls))
	fmt.Println(strings.Repeat("-", 60))
	for i, u := range urls {
		fmt.Printf("%2d. %s\n", i+1, u.OriginalURL)
		canonicalURL := u.CanonicalURL.String
		if !u.CanonicalURL.Valid {
			canonicalURL = "(none)"
		}
		fmt.Printf("    Domain: %s, Canonical: %s\n", u.Domain, canonicalURL)
	}

	// Print results if available
	if len(results) > 0 {
		fmt.Printf("\nResults (%d):\n", len(results))
		fmt.Println(strings.Repeat("-", 60))
		for i, r := range results {
			fmt.Printf("%2d. [%s] %s\n", i+1, r.Status, r.URL)
			if r.Status == "failed" {
				fmt.Printf("    Error: [%s] %s\n", r.ErrorType, r.ErrorMessage)
			} else {
				fmt.Printf("    Status: %d | Size: %d bytes | Tokens: ~%d\n",
					r.StatusCode, r.FileSizeBytes, r.EstimatedTokens)
			}
		}
	}

	fmt.Printf("\nTip: Use 'llm-web-parser db get %d' to see summary YAML\n", sessionID)

	return nil
}

// getSessionAction retrieves and prints session content files
func GetSessionAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	sessionID, err := GetSessionIDOrLatest(c, database)
	if err != nil {
		return err
	}

	// Get session to find directory
	session, err := database.GetSessionByID(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Determine which file to read
	fileType := strings.ToLower(c.String("file"))
	var fileName string
	switch fileType {
	case "index":
		fileName = "summary-index.yaml"
	case "details":
		fileName = "summary-details.yaml"
	case "failed":
		fileName = "failed-urls.yaml"
	default:
		return fmt.Errorf("unknown file type: %s (use: index, details, or failed)", fileType)
	}

	// Build full path (session_dir is relative to output dir)
	outputDir := artifact_manager.DefaultBaseDir
	filePath := filepath.Join(outputDir, session.SessionDir, fileName)

	// Read and print file
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s\nSession directory: %s", fileName, session.SessionDir)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Print session reminder as YAML comment
	fmt.Printf("# Session: %d\n", sessionID)
	fmt.Print(string(data))

	return nil
}

// querySessionsAction queries sessions with filters
func QuerySessionsAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	todayOnly := c.Bool("today")
	failedOnly := c.Bool("failed")
	urlPattern := c.String("url")

	sessions, err := database.QuerySessions(todayOnly, failedOnly, urlPattern)
	if err != nil {
		return fmt.Errorf("failed to query sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found matching filters")
		if todayOnly {
			fmt.Println("  - Filter: today only")
		}
		if failedOnly {
			fmt.Println("  - Filter: with failures")
		}
		if urlPattern != "" {
			fmt.Printf("  - Filter: URL pattern '%s'\n", urlPattern)
		}
		return nil
	}

	// Print table header
	fmt.Printf("%-6s %-20s %-8s %-8s %-8s %-15s %-30s\n",
		"ID", "Created", "URLs", "Success", "Failed", "Parse Mode", "Session Dir")
	fmt.Println(strings.Repeat("-", 120))

	// Print each session
	for _, s := range sessions {
		fmt.Printf("%-6d %-20s %-8d %-8d %-8d %-15s %-30s\n",
			s.SessionID,
			s.CreatedAt.Format("2006-01-02 15:04:05"),
			s.URLCount,
			s.SuccessCount,
			s.FailedCount,
			s.ParseMode,
			s.SessionDir,
		)
	}

	fmt.Printf("\nFound: %d sessions\n", len(sessions))

	return nil
}

// urlsAction shows URLs for a session with sanitization tracking and metadata
func UrlsAction(c *cli.Context) error {
	database, err := dbpkg.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	sessionID, err := GetSessionIDOrLatest(c, database)
	if err != nil {
		return err
	}

	// Get URLs with full metadata
	urls, err := database.GetSessionURLsWithMetadata(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session URLs: %w", err)
	}

	if len(urls) == 0 {
		fmt.Printf("No URLs found for session %d\n", sessionID)
		return nil
	}

	sanitizedOnly := c.Bool("sanitized")

	if sanitizedOnly {
		// Show only sanitized URLs (existing behavior)
		sanitizedURLs := []dbpkg.URLWithMetadata{}
		for _, u := range urls {
			if u.WasSanitized {
				sanitizedURLs = append(sanitizedURLs, u)
			}
		}

		if len(sanitizedURLs) == 0 {
			fmt.Printf("Session: %d\n\n", sessionID)
			fmt.Printf("No URLs were auto-cleaned in this session\n")
			return nil
		}

		fmt.Printf("Session: %d\n\n", sessionID)
		fmt.Printf("Auto-cleaned URLs:\n\n")
		for i, u := range sanitizedURLs {
			fmt.Printf("%2d. [#%d] Original: %s\n", i+1, u.URLID, u.OriginalURL)
			fmt.Printf("          Cleaned:  %s\n\n", u.URL)
		}
	} else {
		// Show all URLs with rich metadata
		fmt.Printf("Session: %d\n\n", sessionID)
		for i, u := range urls {
			// Line 1: URL with sanitization indicator
			if u.WasSanitized {
				fmt.Printf("%2d. [#%d] %s (cleaned)\n", i+1, u.URLID, u.URL)
			} else {
				fmt.Printf("%2d. [#%d] %s\n", i+1, u.URLID, u.URL)
			}

			// Line 2: Metadata - content type, code flag, confidence
			codeFlag := "no_code"
			if u.HasCodeExamples {
				codeFlag = "has_code"
			}
			fmt.Printf("    %s | %s | conf:%.1f\n",
				u.ContentType, codeFlag, u.DetectionConfidence)

			// Line 3: Keywords (if available)
			if len(u.TopKeywords) > 0 {
				fmt.Printf("    Keywords: %s\n", strings.Join(u.TopKeywords, ", "))
			}

			fmt.Println() // Blank line between URLs
		}
	}

	return nil
}

// showAction shows parsed content for a URL by ID or URL
func ShowAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("URL ID or URL required\nUsage: llm-web-parser db show <url_id_or_url>\nExample: llm-web-parser db show 123 OR llm-web-parser db show 6,7,8 OR llm-web-parser db show https://example.com")
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
	// outputFormat already declared above

	// Apply outline filter (special output)
	if outlineMode {
		fmt.Print(filterOutline(&page))
		return nil
	}

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
		filtered, err := filterByGrep(&page, grepPattern, grepContext)
		if err != nil {
			return err
		}
		page = *filtered
	}

	// Re-marshal based on requested format
	var output []byte
	if outputFormat == "json" {
		output, err = json.MarshalIndent(&page, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal content as JSON: %w", err)
		}
		fmt.Print(string(output))
	} else {
		// Default to YAML
		output, err = yaml.Marshal(&page)
		if err != nil {
			return fmt.Errorf("failed to marshal content as YAML: %w", err)
		}
		if onlyTypes != "" || grepPattern != "" {
			fmt.Println("# YAML compact mode: Only non-null/non-default fields shown (filtered)")
		} else {
			fmt.Println("# YAML compact mode: Only non-null/non-default fields shown")
		}
		fmt.Print(string(output))
	}
	return nil
}

// rawAction shows raw HTML for a URL by ID or URL
func RawAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("URL ID or URL required\nUsage: llm-web-parser db raw <url_id_or_url>\nExample: llm-web-parser db raw 123 OR llm-web-parser db raw 6,7,8 OR llm-web-parser db raw https://example.com")
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
		return fmt.Errorf("URL required\nUsage: llm-web-parser db find-url <url>\nExample: llm-web-parser db find-url https://example.com")
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
func filterOutline(page *models.Page) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("url: %s\n", page.URL))
	sb.WriteString(fmt.Sprintf("title: %s\n\n", page.Title))

	// Count block types
	typeCounts := make(map[string]int)
	var countTypes func(sections []models.Section)
	countTypes = func(sections []models.Section) {
		for _, section := range sections {
			for _, block := range section.Blocks {
				typeCounts[block.Type]++
			}
			if len(section.Children) > 0 {
				countTypes(section.Children)
			}
		}
	}
	countTypes(page.Content)

	// Show block type counts
	sb.WriteString("content types:\n")
	// Order: table, code, li, p, then others
	priority := []string{"table", "code", "li", "p"}
	for _, t := range priority {
		if count, ok := typeCounts[t]; ok {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", t, count))
		}
	}
	// Show other types
	for t, count := range typeCounts {
		isPriority := false
		for _, pt := range priority {
			if t == pt {
				isPriority = true
				break
			}
		}
		if !isPriority {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", t, count))
		}
	}

	sb.WriteString("\ndocument structure (headings h1-h6):\n")

	// Recursively extract headings from hierarchical content
	var extractHeadings func(sections []models.Section)
	extractHeadings = func(sections []models.Section) {
		for _, section := range sections {
			if section.Heading != nil && section.Heading.Text != "" {
				indent := strings.Repeat("  ", section.Level-1)
				sb.WriteString(fmt.Sprintf("%s- %s (%s)\n", indent, section.Heading.Text, section.Heading.Type))
			}
			// Recurse into children
			if len(section.Children) > 0 {
				extractHeadings(section.Children)
			}
		}
	}

	extractHeadings(page.Content)

	// Add helpful examples based on what's in the document
	sb.WriteString("\nuseful commands:\n")
	if typeCounts["table"] > 0 {
		sb.WriteString("  llm-web-parser db show --only=table 1        # Show tables only\n")
	}
	if typeCounts["code"] > 0 {
		sb.WriteString("  llm-web-parser db show --only=code 1         # Show code blocks only\n")
	}
	if typeCounts["li"] > 0 {
		sb.WriteString("  llm-web-parser db show --only=li 1           # Show list items only\n")
	}
	sb.WriteString("  llm-web-parser db show --grep \"keyword\" 1    # Search for keyword\n")
	sb.WriteString("  llm-web-parser db show --format json 1       # Output as JSON for jq\n")

	return sb.String()
}

// filterByType filters ContentBlocks by type (comma-separated list).
func filterByType(page *models.Page, types string) (*models.Page, error) {
	if types == "" {
		return page, nil
	}

	typeList := strings.Split(types, ",")
	typeMap := make(map[string]bool)
	for _, t := range typeList {
		typeMap[strings.TrimSpace(t)] = true
	}

	filtered := &models.Page{
		URL:      page.URL,
		Title:    page.Title,
		Metadata: page.Metadata,
		Content:  make([]models.Section, 0),
	}

	// Recursively filter sections
	var filterSection func(section models.Section) *models.Section
	filterSection = func(section models.Section) *models.Section {
		filteredSection := models.Section{
			ID:       section.ID,
			Heading:  section.Heading,
			Level:    section.Level,
			Blocks:   make([]models.ContentBlock, 0),
			Children: make([]models.Section, 0),
		}

		// Filter blocks by type
		for _, block := range section.Blocks {
			if typeMap[block.Type] {
				filteredSection.Blocks = append(filteredSection.Blocks, block)
			}
			// Special handling for table type - check if block contains a table
			if typeMap["table"] && block.Table != nil {
				filteredSection.Blocks = append(filteredSection.Blocks, block)
			}
		}

		// Recursively filter children
		for _, child := range section.Children {
			if filtered := filterSection(child); filtered != nil {
				filteredSection.Children = append(filteredSection.Children, *filtered)
			}
		}

		// Only include section if it has matching content
		if len(filteredSection.Blocks) > 0 || len(filteredSection.Children) > 0 {
			return &filteredSection
		}

		return nil
	}

	for _, section := range page.Content {
		if filteredSec := filterSection(section); filteredSec != nil {
			filtered.Content = append(filtered.Content, *filteredSec)
		}
	}

	return filtered, nil
}

// filterByGrep searches for a pattern in ContentBlocks and includes context.
func filterByGrep(page *models.Page, pattern string, context int) (*models.Page, error) {
	if pattern == "" {
		return page, nil
	}

	re, err := regexp.Compile("(?i)" + pattern) // Case-insensitive
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	filtered := &models.Page{
		URL:   page.URL,
		Title: page.Title,
		FlatContent: make([]models.ContentBlock, 0),
	}

	// Find all matching indices
	matches := make(map[int]bool)
	for i, block := range page.FlatContent {
		if re.MatchString(block.Text) {
			// Mark this block and context blocks
			for j := i - context; j <= i+context; j++ {
				if j >= 0 && j < len(page.FlatContent) {
					matches[j] = true
				}
			}
		}
	}

	// Add all matched blocks (in order)
	for i, block := range page.FlatContent {
		if matches[i] {
			filtered.FlatContent = append(filtered.FlatContent, block)
		}
	}

	return filtered, nil
}
