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
				fmt.Println(help.ColdstartYAML)
				os.Exit(0)
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "fetch",
				Usage: "Fetch and parse URLs",
				Description: `EXAMPLES:
   # Basic fetch (metadata only, no keywords)
   llm-web-parser fetch --urls "https://example.com"

   # With keywords for triage (recommended for LLMs)
   llm-web-parser fetch --urls "https://example.com" --features wordcount

   # Full content extraction
   llm-web-parser fetch --urls "https://example.com" --features full-parse

   # Inline filtering
   llm-web-parser fetch --urls "..." --features full-parse --filter "conf:>=0.7"

FEATURES:
   minimal (default)   - Metadata only. Fast but NO keywords (can't see what URLs are about)
   wordcount          - Adds keyword extraction (~1s extra). Shows top keywords per URL
   full-parse         - Full content + keywords. Use for deep content analysis

NOTES:
   • Sessions auto-tracked in SQLite database (lwp-web-parser.db)
   • Same URL = instant cache hit (no re-fetching within --max-age window)
   • Results stored in llm-web-parser-results/ directory
   • Next steps shown in fetch output (corpus commands, db commands)
`,
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
						Name:  "sessions",
						Usage: "List all sessions",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "limit",
								Usage: "Maximum number of sessions to show (0 = all)",
								Value: 20,
							},
						},
						Action: db.SessionsAction,
					},
					{
						Name:      "session",
						Usage:     "Show session details (defaults to latest)",
						ArgsUsage: "[session_id]",
						Action:    db.SessionAction,
					},
					{
						Name:      "get",
						Usage:     "Retrieve session content (defaults to latest)",
						ArgsUsage: "[session_id]",
						Flags: []cli.Flag{
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
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "sanitized",
								Usage: "Show only URLs that were auto-cleaned",
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
						Name:      "show",
						Usage:     "Show parsed JSON for a URL (by ID or URL)",
						ArgsUsage: "<url_id_or_url>",
						Action:    db.ShowAction,
					},
					{
						Name:      "raw",
						Usage:     "Show raw HTML for a URL (by ID or URL)",
						ArgsUsage: "<url_id_or_url>",
						Action:    db.RawAction,
					},
					{
						Name:      "find-url",
						Usage:     "Find the URL ID for a given URL",
						ArgsUsage: "<url>",
						Action:    db.FindURLAction,
					},
				},
			},
			{
				Name:  "corpus",
				Usage: "Corpus API - query and analyze web content collections",
				Subcommands: []*cli.Command{
					{
						Name:   "extract",
						Usage:  "Extract and aggregate keywords from URLs",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID (extract keywords from all URLs in session)"},
							&cli.StringFlag{Name: "url-ids", Usage: "Comma-separated URL IDs (e.g., 1,3,5)"},
							&cli.IntFlag{Name: "top", Value: 25, Usage: "Return top N keywords (0 for all)"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "query",
						Usage:  "Boolean filtering over metadata",
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
						Usage:  "Cross-document analysis (consensus, contradictions, approaches)",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "detect",
						Usage:  "Pattern recognition (clusters, warnings, gaps, anomalies, trends)",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "normalize",
						Usage:  "Canonicalize entities, dates, versions, code",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "trace",
						Usage:  "Citation graphs, authority scoring, provenance",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "score",
						Usage:  "Confidence and quality metrics",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "delta",
						Usage:  "Incremental updates (what changed since baseline)",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "summarize",
						Usage:  "Structured synthesis (decision-inputs, timelines, matrices)",
						Action: corpusactions.CorpusAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID"},
							&cli.StringFlag{Name: "view", Usage: "View name"},
							&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format (json, yaml, csv)"},
						},
					},
					{
						Name:   "suggest",
						Usage:  "Suggest queries based on session contents",
						Action: corpusactions.SuggestAction,
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "session", Usage: "Session ID", Required: true},
						},
					},
					{
						Name:   "explain-failure",
						Usage:  "Diagnostic transparency for low confidence / failures",
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
