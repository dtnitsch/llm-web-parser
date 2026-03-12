// Package main provides the llm-web-parser CLI tool for extracting and analyzing web content.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/dtnitsch/llm-web-parser/internal/analyze"
	corpusactions "github.com/dtnitsch/llm-web-parser/internal/corpus"
	"github.com/dtnitsch/llm-web-parser/internal/db"
	"github.com/dtnitsch/llm-web-parser/internal/fetch"
	"github.com/dtnitsch/llm-web-parser/pkg/artifact_manager"
	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
	"github.com/dtnitsch/llm-web-parser/pkg/help"

	"github.com/urfave/cli/v2"
)

func main() {
	// Will be overridden in commands based on --quiet flag
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	app := &cli.App{
		Name:  "llm-web-parser",
		Usage: "A CLI tool to fetch and parse web content for LLMs.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "coldstart",
				Usage: "Show quick start guide with concepts, examples, and invariants",
			},
		},
		Before: func(c *cli.Context) error {
			if c.Bool("coldstart") {
				fmt.Print(help.ColdstartYAML)
				os.Exit(0)
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "fetch",
				Usage: "Fetch and parse URLs",
				Description: `Fetches URLs and extracts metadata, keywords, and content.

Features: minimal (metadata only), wordcount (default, adds keywords), full-parse (full content).
Sessions are auto-tracked in SQLite database for easy refetching.

Run 'llm-web-parser fetch' (no args) for examples.`,
				Action: fetch.FetchAction,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "quiet",
						Usage: "Suppress log output and URL ID display (default: true, use --quiet=false for verbose output)",
						Value: true,
					},
					&cli.StringFlag{
						Name:  "features",
						Usage: "Features to enable: minimal, wordcount (default), full-parse",
						Value: "wordcount",
					},
					&cli.StringFlag{
						Name:    "urls",
						Usage:   "Comma-separated list of URLs to process",
						Aliases: []string{"u"},
					},
					&cli.IntFlag{
						Name:  "session",
						Usage: "Refetch URLs from a previous session (use session ID)",
					},
					&cli.BoolFlag{
						Name:  "failed-only",
						Usage: "Only refetch failed URLs (requires --session)",
					},
					&cli.IntFlag{
						Name:    "workers",
						Usage:   "Number of concurrent workers",
						Aliases: []string{"w"},
						Value:   8,
					},
					&cli.StringFlag{
						Name:    "format",
						Usage:   "Output format (json or yaml). Default: yaml (more token-efficient)",
						Aliases: []string{"f"},
						Value:   "yaml",
					},
					&cli.StringFlag{
						Name:  "output-mode",
						Usage: "Output mode (tier2, summary, full, minimal). Default: tier2 (index to stdout + details file)",
						Value: "tier2",
					},
					&cli.StringFlag{
						Name:  "max-age",
						Usage: "Maximum age for raw HTML artifacts (e.g., '1h', '0s' to always fetch fresh)",
						Value: "1h",
					},
					&cli.BoolFlag{
						Name:  "force-fetch",
						Usage: "Force fetching all URLs, ignoring max-age and existing artifacts",
					},
					&cli.StringFlag{
						Name:  "output-dir",
						Usage: "Base directory for storing raw and parsed artifacts",
						Value: artifact_manager.DefaultBaseDir,
					},
					&cli.StringFlag{
						Name:  "summary-version",
						Usage: "Summary output format version (v1=verbose, v2=terse)",
						Value: "v1",
					},
					&cli.StringFlag{
						Name:  "summary-fields",
						Usage: "Comma-separated list of fields to include in summary (e.g., 'url,tokens,quality'). Empty = all fields.",
						Value: "",
					},
					&cli.StringFlag{
						Name:  "filter",
						Usage: "Filter parsed content by confidence/type (e.g., 'conf:>=0.7', 'type:code', 'conf:>=0.8,type:p|code')",
						Value: "",
					},
				},
			},
			{
				Name:   "extract",
				Usage:  "DEPRECATED: Use 'fetch --filter' instead",
				Hidden: true,
				Action: analyze.ExtractAction,
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "from",
						Usage:   "Path or glob pattern to one or more parsed JSON files",
						Aliases: []string{"i"}, // for input
					},
					&cli.StringFlag{
						Name:    "strategy",
						Usage:   "Filtering strategy (e.g., 'conf:>=0.7,type:code|table')",
						Aliases: []string{"s"},
					},
				},
			},
			{
				Name:   "analyze",
				Usage:  "DEPRECATED: Use 'fetch' instead (auto-detects cache)",
				Hidden: true,
				Action: analyze.AnalyzeAction,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "urls",
						Usage:   "Comma-separated list of URLs to re-analyze from cache",
						Aliases: []string{"u"},
					},
					&cli.StringFlag{
						Name:  "features",
						Usage: "Comma-separated features (full-parse, wordcount)",
						Value: "full-parse",
					},
					&cli.StringFlag{
						Name:  "output-dir",
						Usage: "Base directory for cached artifacts",
						Value: artifact_manager.DefaultBaseDir,
					},
					&cli.StringFlag{
						Name:  "max-age",
						Usage: "Maximum age for cached HTML (e.g., '24h'). Use '0s' to require fresh cache.",
						Value: "24h",
					},
					&cli.BoolFlag{
						Name:  "quiet",
						Usage: "Suppress log output (default: true, use --quiet=false for verbose logs)",
						Value: true,
					},
				},
			},
			{
				Name:  "db",
				Usage: "Database operations",
				Before: func(c *cli.Context) error {
					// If no subcommand provided, show helpful examples
					if c.Args().Len() == 0 {
						printDBHelp()
						os.Exit(0)
					}
					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "init",
						Usage: "Initialize database schema",
						Action: func(c *cli.Context) error {
							database, err := dbpkg.Open()
							if err != nil {
								return fmt.Errorf("failed to open database: %w", err)
							}
							defer database.Close()

							if err := database.InitSchema(); err != nil {
								return fmt.Errorf("failed to initialize schema: %w", err)
							}

							fmt.Printf("Database initialized at: %s\n", database.Path())
							return nil
						},
					},
					{
						Name:  "path",
						Usage: "Show database file location",
						Action: func(c *cli.Context) error {
							database, err := dbpkg.Open()
							if err != nil {
								return fmt.Errorf("failed to open database: %w", err)
							}
							defer database.Close()

							fmt.Println(database.Path())
							return nil
						},
					},
					{
						Name:  "sessions",
						Usage: "List all sessions",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "limit",
								Usage: "Maximum number of sessions to show (0 = all)",
								Value: 20,
							},
							&cli.BoolFlag{
								Name:  "verbose",
								Usage: "Show aggregated keywords, content types, and code percentage",
							},
						},
						Action: db.SessionsAction,
					},
					{
						Name:      "session",
						Usage:     "Show session details (defaults to latest)",
						ArgsUsage: "[session_id]",
						Description: `EXAMPLES:
   llm-web-parser db session          # Latest session
   llm-web-parser db session 5        # Session 5 (positional)
   llm-web-parser db session --session 5  # Session 5 (flag)

NOTE: Use --session 5 (space, not equals)`,
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "session",
								Usage: "Session ID to show (alternative to positional arg)",
							},
						},
						Action: db.SessionAction,
					},
					{
						Name:      "get",
						Usage:     "Retrieve session content (defaults to latest)",
						ArgsUsage: "[session_id]",
						Description: `EXAMPLES:
   llm-web-parser db get --file=details      # Latest session
   llm-web-parser db get 5                   # Session 5 (positional)
   llm-web-parser db get --session 5         # Session 5 (flag)
   llm-web-parser db get --session 5 --file=index

NOTE: Use --session 5 (space, not equals)`,
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "session",
								Usage: "Session ID to retrieve (alternative to positional arg)",
							},
							&cli.StringFlag{
								Name:  "file",
								Usage: "Which file to show: index, details, failed (default: details)",
								Value: "details",
							},
						},
						Action: db.GetSessionAction,
					},
					{
						Name:      "urls",
						Usage:     "Show URLs for a session (defaults to latest)",
						ArgsUsage: "[session_id]",
						Description: `EXAMPLES:
   llm-web-parser db urls             # Latest session
   llm-web-parser db urls 7           # Session 7 (positional)
   llm-web-parser db urls --session 7 # Session 7 (flag)
   llm-web-parser db urls --session 7 --verbose

NOTE: Use --session 7 (space, not equals)`,
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "session",
								Usage: "Session ID to show URLs for (alternative to positional arg)",
							},
							&cli.BoolFlag{
								Name:  "verbose",
								Usage: "Show detailed 3-line format with metadata (default: compact 1-line format)",
							},
						},
						Action: db.UrlsAction,
					},
					{
						Name:  "query",
						Usage: "Query sessions with filters",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "today",
								Usage: "Show only sessions created today",
							},
							&cli.BoolFlag{
								Name:  "failed",
								Usage: "Show only sessions with failed URLs",
							},
							&cli.StringFlag{
								Name:  "url",
								Usage: "Filter by URL pattern (LIKE match)",
							},
						},
						Action: db.QuerySessionsAction,
					},
					{
						Name:      "use",
						Usage:     "Set or show active session (no args = show current)",
						ArgsUsage: "[session_id|latest]",
						Description: `EXAMPLES:
   # Set active session
   llm-web-parser db use 12

   # Switch to latest session
   llm-web-parser db use latest

   # Show current active session
   llm-web-parser db use

   # Clear active session
   llm-web-parser db use --clear

Active session is stored in .lwp/config (per-project).
When active, commands default to it instead of latest:
   llm-web-parser db urls            # Uses active session
   llm-web-parser db show 42         # Uses active session context

NOTE: New fetches auto-switch to the new session.`,
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "clear",
								Usage: "Clear active session",
							},
						},
						Action: db.UseAction,
					},
					{
						Name:      "show",
						Usage:     "Show parsed content for a URL (by ID or URL)",
						ArgsUsage: "<url_id_or_url>",
						Description: `EXAMPLES:
   # By URL ID (most efficient - saves tokens)
   llm-web-parser db show 42

   # Get just the outline (headings)
   llm-web-parser db show --outline 42

   # Filter by content type (code blocks only)
   llm-web-parser db show --only=code,pre 42

   # Search for specific pattern with context
   llm-web-parser db show --grep="async" --context=3 42

   # Batch retrieve (comma-separated IDs)
   llm-web-parser db show 42,43,44

   # Export formats
   llm-web-parser db show --format json 42        # JSON for jq processing
   llm-web-parser db show --format markdown 42    # Markdown for notes/docs
   llm-web-parser db show --format csv 42         # CSV for spreadsheets

   # Complex filtering with jq
   llm-web-parser db show --format json 42 | jq '.flatcontent[] | select(.type == "code")'

NOTE: Use 'llm-web-parser db urls' to see URL IDs for the latest session.
NOTE: Flags must come BEFORE the ID/URL (urfave/cli requirement).`,
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "outline",
								Usage: "Show document outline (headings only)",
							},
							&cli.StringFlag{
								Name:  "only",
								Usage: "Filter by block type (comma-separated: code,pre,h1,h2,p)",
							},
							&cli.StringFlag{
								Name:  "grep",
								Usage: "Search for pattern in content (regex supported)",
							},
							&cli.IntFlag{
								Name:  "context",
								Usage: "Number of blocks to show before/after grep matches",
								Value: 3,
							},
							&cli.BoolFlag{
								Name:  "metadata",
								Usage: "Show compact metadata (5 essential fields)",
							},
							&cli.BoolFlag{
								Name:  "metadata-full",
								Usage: "Show all metadata fields (including empty)",
							},
							&cli.StringFlag{
								Name:  "format",
								Usage: "Output format: yaml (default), json, markdown, or csv",
								Value: "yaml",
							},
						},
						Action: db.ShowAction,
					},
					{
						Name:      "raw",
						Usage:     "Show raw HTML for a URL (by ID or URL)",
						ArgsUsage: "<url_id_or_url>",
						Description: `EXAMPLES:
   # By URL ID
   llm-web-parser db raw 42

   # By full URL
   llm-web-parser db raw https://golang.org

NOTE: This shows the cached HTML. Use 'llm-web-parser db urls' to find URL IDs.`,
						Action:    db.RawAction,
					},
					{
						Name:      "find-url",
						Usage:     "Find the URL ID for a given URL",
						ArgsUsage: "<url>",
						Description: `EXAMPLES:
   llm-web-parser db find-url https://golang.org
   # Output: [#42] https://golang.org

   # Then use the ID for efficient access:
   llm-web-parser db show 42
   llm-web-parser db raw 42`,
						Action:    db.FindURLAction,
					},
				},
			},
			{
				Name:  "corpus",
				Usage: "Corpus API - query and analyze web content collections",
				Before: func(c *cli.Context) error {
					// If no subcommand provided, show helpful examples
					if c.Args().Len() == 0 {
						printCorpusHelp()
						os.Exit(0)
					}
					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:      "grep",
						Usage:     "Search across multiple URLs in a session",
						ArgsUsage: "<pattern>",
						Action:    corpusactions.GrepAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID (default: active session, fallback to latest)"},
							&cli.StringFlag{Name: "urls", Usage: "Comma-separated URL IDs or URLs (default: all URLs in session)"},
							&cli.IntFlag{Name: "context", Usage: "Lines of context around matches (not yet implemented)"},
							&cli.StringFlag{Name: "format", Value: "text", Usage: "Output format (text, json, yaml, csv)"},
						},
					},
					{
						Name:   "extract",
						Usage:  "[WORKING] Extract and aggregate keywords from URLs",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID (extract keywords from all URLs in session)"},
							&cli.StringFlag{Name: "url-ids", Usage: "Comma-separated URL IDs (e.g., 1,3,5)"},
							&cli.IntFlag{Name: "top", Value: 10, Usage: "Return top N keywords (0 for all)"},
							&cli.IntFlag{Name: "limit", Value: 10, Usage: "Alias for --top", Hidden: true},
							&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Show full output (confidence, coverage, hints)"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "query",
						Usage:  "[NOT IMPLEMENTED] Boolean filtering over metadata",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "filter", Usage: "Filter expression (e.g., 'has_code AND citations>50')"},
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "compare",
						Usage:  "[NOT IMPLEMENTED] Cross-document analysis (consensus, contradictions, approaches)",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "detect",
						Usage:  "[NOT IMPLEMENTED] Pattern recognition (clusters, warnings, gaps, anomalies, trends)",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "normalize",
						Usage:  "[NOT IMPLEMENTED] Canonicalize entities, dates, versions, code",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "trace",
						Usage:  "[NOT IMPLEMENTED] Citation graphs, authority scoring, provenance",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "score",
						Usage:  "[NOT IMPLEMENTED] Confidence and quality metrics",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "delta",
						Usage:  "[NOT IMPLEMENTED] Incremental updates (what changed since baseline)",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "summarize",
						Usage:  "[NOT IMPLEMENTED] Structured synthesis (decision-inputs, timelines, matrices)",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "suggest",
						Usage:  "[WORKING] Suggest queries based on session contents",
						Action: corpusactions.SuggestAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID", Required: true},
						},
					},
					{
						Name:   "explain-failure",
						Usage:  "[NOT IMPLEMENTED] Diagnostic transparency for low confidence / failures",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Error("application exited with an error", "error", err)
		os.Exit(1)
	}
}

// printDBHelp prints LLM-friendly examples for db operations.
func printDBHelp() {
	// Get current working directory for context
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "{current-directory}"
	}

	fmt.Printf(`💡 No subcommand specified. Here are the database operations:

Session management (sessions, session, get, urls):
  llm-web-parser db sessions                        # List all sessions
  llm-web-parser db session                         # Show latest session details
  llm-web-parser db session 5                       # Show session 5 details
  llm-web-parser db get --file=details             # Get latest session YAML (summary-details.yaml)
  llm-web-parser db get --file=index 5             # Get session 5 summary-index.yaml
  llm-web-parser db urls                            # Show URL IDs for latest session
  llm-web-parser db urls --verbose                  # Show URLs with keywords and metadata

URL content operations (show, raw, find-url):
  llm-web-parser db show 42                         # Show parsed content for URL ID 42
  llm-web-parser db show --outline 42               # Show document outline (headings only)
  llm-web-parser db show --only=h2,code 42          # Filter by block type
  llm-web-parser db show 42,43,44                   # Batch retrieve multiple URLs
  llm-web-parser db raw 42                          # Show raw HTML for URL ID 42
  llm-web-parser db find-url https://example.com    # Find URL ID for a URL

Process with external tools (root path is .content[]):
  # YAML output (default) - use yq for YAML processing:
  llm-web-parser db show 42 | yq '.content[] | select(.level == 2) | .heading.text'

  # JSON output - use jq for JSON processing (note: --format flag before URL ID):
  llm-web-parser db show --format json 42 | jq '.content[] | select(.level == 2) | .heading.text'

Query operations:
  llm-web-parser db query --today                   # Sessions created today
  llm-web-parser db query --failed                  # Sessions with failed URLs
  llm-web-parser db query --url=example.com         # Sessions containing URL

Database info:
  llm-web-parser db path                            # Show database location
  llm-web-parser db init                            # Initialize database schema

Where data lives:
  - Database: %s/llm-web-parser.db
  - Sessions: %s/lwp-sessions/YYYY-MM-DD-{id}/
  - Each session has: summary-index.yaml, summary-details.yaml, failed-urls.yaml (if any)

Tip: Most commands default to latest session. Use session ID to specify a different one.

Run 'llm-web-parser db --help' for subcommand list.
`, cwd, cwd)
}

// printCorpusHelp prints LLM-friendly examples for corpus operations.
func printCorpusHelp() {
	fmt.Print(`💡 No subcommand specified. Here are the corpus operations:

Extract keywords (get top keywords across URLs):
  llm-web-parser corpus extract --session=1                  # Top 10 keywords from session 1
  llm-web-parser corpus extract --session=1 --top=25         # Top 25 keywords
  llm-web-parser corpus extract --url-ids=42,43,44 --top=50  # Keywords from specific URLs

Query metadata (filter URLs by detected properties):
  llm-web-parser corpus query --session=1 --filter="content_type=academic"
  llm-web-parser corpus query --session=1 --filter="has_code_examples"
  llm-web-parser corpus query --session=1 --filter="citation_count>=20"
  llm-web-parser corpus query --session=1 --filter="keyword:api"
  llm-web-parser corpus query --session=1 --filter="has_code_examples AND keyword:python"

Get query suggestions (see what's available in your session):
  llm-web-parser corpus suggest --session=1                  # Analyzes session and suggests queries

Working commands:
  ✅ extract  - Aggregate keywords across URLs
  ✅ query    - Boolean filtering over metadata (has_code_examples, content_type, citations, etc.)
  ✅ suggest  - Smart query suggestions based on session content

Planned commands (not yet implemented):
  ⏳ compare, detect, normalize, trace, score, delta, summarize, explain-failure

Tip: Run any command without arguments to see detailed examples:
  llm-web-parser corpus query           # Shows all available filters with examples
  llm-web-parser corpus extract --help  # Traditional help

Run 'llm-web-parser corpus --help' for subcommand list.
`)
}
