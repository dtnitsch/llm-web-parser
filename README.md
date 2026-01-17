# llm-web-parser

**Fast, LLM-optimized web content extraction with intelligent caching and session tracking.**

Extract clean metadata from web pages in minimal mode (fast) or full content with confidence scoring (thorough). Built for LLM workflows with token-efficient YAML output and smart session management.

## What Works Right Now

✅ **Core Features (Stable)**
- Fetch & parse URLs (minimal, wordcount, full-parse modes)
- Session management & URL ID system
- Database operations (sessions, urls, show, raw, find-url, **path**)
- Keyword extraction (`corpus extract`)
- Query suggestions (`corpus suggest`)

⏳ **Planned Features**
- Advanced corpus verbs (query, compare, detect, normalize, trace, score, delta, summarize)
- See `docs/CORPUS-API.md` for roadmap

📚 **Quick Start**: See `examples/` directory for ready-to-run workflows

## Features

- **Zero-config setup** - Auto-initializes database, just run it
- **Fast minimal mode** - Extract metadata only (~150 bytes/URL, 2-5 seconds for 50 URLs)
- **Full-parse mode** - Deep content extraction with confidence scores
- **Session tracking** - SQLite-backed sessions with auto-incrementing IDs
- **URL ID system** - Reference URLs by ID to save tokens (90% reduction)
- **Smart caching** - Instant cache hits for duplicate URL sets
- **Parallel processing** - 8 concurrent workers by default
- **URL sanitization** - Auto-cleans markdown links, whitespace, trailing punctuation
- **Flexible refetch** - Refetch sessions with different modes or retry failures

## Quick Start

```bash
# Build
go build

# Fetch URLs (auto-creates database)
./llm-web-parser fetch --urls "https://golang.org,https://www.python.org"

# View results (defaults to latest session)
./llm-web-parser db get --file=details

# See URL IDs for easy reference
./llm-web-parser db urls

# Get content by ID (saves tokens!)
./llm-web-parser db show 1
```

**That's it!** No configuration needed. Database and results auto-initialize.

## For LLMs

**Complete command reference:** See [`docs/development/index.yaml`](docs/development/index.yaml)

- **Start here:** Read `index.yaml` for overview and structure
- **Detailed examples:** Consult `fetch.md`, `db.md`, `corpus.md` for specific commands
- **Quick search:** Use grep to find patterns (index shows grep hints)

All commands with combined examples, workflows, and implementation status.

## Core Workflows

### 1. Fast Scan → Deep Dive (Recommended for LLMs)

```bash
# Stage 1: Fast minimal scan (metadata only)
llm-web-parser fetch --urls "url1,url2,...,url50"
# Output: Session 1: 48/50 URLs successful (2-5 seconds)

# Stage 2: Analyze confidence scores
llm-web-parser db get --file=details | yq '.[] | select(.confidence >= 7)'

# Stage 3: Deep parse high-confidence URLs
llm-web-parser fetch --session 1 --features full-parse
# Refetches same URLs with full content extraction
```

### 2. Retry Failed URLs

```bash
# Some URLs failed during fetch
llm-web-parser fetch --urls "url1,bad-url,url3"
# Session 2: 2/3 URLs successful

# Retry only the failures
llm-web-parser fetch --session 2 --failed-only
# Retrying 1 failed URLs from session 2
```

### 3. Use URL IDs to Save Tokens

```bash
# Get URL IDs from latest session
llm-web-parser db urls
# Output:
# Session: 5
#  1. [#42] https://golang.org
#  2. [#43] https://www.python.org

# Reference by ID instead of full URL (10 tokens → 1 token)
llm-web-parser db show 42        # Instead of full URL
llm-web-parser db raw 43         # Get raw HTML
llm-web-parser db show 42,43     # Batch retrieve
```

## Session System

Sessions track every fetch operation with auto-incrementing IDs (1, 2, 3...).

**Key behaviors:**
- Same URLs = same session ID = instant cache hit (no re-fetching)
- Session directories: `sessions/2026-01-15-1` (date + ID)
- Sessions stored in SQLite database + YAML files on disk
- Commands default to latest session (no ID needed)

**Structure:**
```
llm-web-parser-results/
├── FIELDS.yaml                    # Field reference with query examples
├── index.yaml                     # All sessions registry
├── llm-web-parser.db              # SQLite database (auto-created)
├── sessions/
│   └── 2026-01-15-1/              # Session directory
│       ├── summary-index.yaml     # Minimal data (~150 bytes/URL)
│       ├── summary-details.yaml   # Full metadata (~400 bytes/URL)
│       └── failed-urls.yaml       # Failed URLs (if any)
├── raw/                           # Shared HTML cache
└── parsed/                        # Shared JSON cache
```

## Common Commands

### Fetching

```bash
# Basic fetch
llm-web-parser fetch --urls "url1,url2,url3"

# Full parse mode
llm-web-parser fetch --urls "url1,url2" --features full-parse

# Refetch session with different mode
llm-web-parser fetch --session 5 --features full-parse

# Retry failures
llm-web-parser fetch --session 5 --failed-only

# Force fresh fetch (ignore cache)
llm-web-parser fetch --session 5 --force-fetch
```

### Session Management

```bash
# List all sessions
llm-web-parser db sessions

# Show latest session details
llm-web-parser db session

# Show specific session
llm-web-parser db session 5

# Get session YAML (latest)
llm-web-parser db get --file=details
llm-web-parser db get --file=index
llm-web-parser db get --file=failed

# Get specific session
llm-web-parser db get --file=details 5

# Show URLs with IDs (latest session)
llm-web-parser db urls

# Show only sanitized URLs
llm-web-parser db urls --sanitized
```

### URL Operations

```bash
# Show parsed content by URL ID
llm-web-parser db show 42

# Show raw HTML by URL ID
llm-web-parser db raw 42

# Batch retrieve
llm-web-parser db show 42,43,44

# Find URL ID for a URL
llm-web-parser db find-url https://golang.org
# Output: [#42] https://golang.org
```

### Querying

```bash
# Query sessions
llm-web-parser db query --today
llm-web-parser db query --failed
llm-web-parser db query --url=example.com

# Query YAML results with yq
llm-web-parser db get --file=details | yq '.[] | select(.confidence >= 7)'
llm-web-parser db get --file=details | yq '.[] | select(.domain_type == "academic")'
llm-web-parser db get --file=details | yq '.[] | select(.has_doi and .academic_score >= 7)'
```

## Parse Modes

### Minimal Mode
**Metadata-only extraction** - Fastest, but NO keyword extraction

Use when you only need basic metadata and don't care about content topics.

Fields extracted:
- `title`, `excerpt`, `site_name`, `author`, `published_at`
- `domain_type` (gov, edu, academic, commercial, mobile, unknown)
- `domain_category` (gov/health, academic/ai, news/tech, docs/api, etc.)
- `confidence` (0-10 quality/credibility score)
- `academic_score` (0-10 academic signal strength)
- Academic signals: `has_doi`, `has_arxiv`, `has_latex`, `has_citations`, etc.
- Content metrics: `word_count`, `estimated_tokens`, `read_time_min`
- Language: `language`, `language_confidence`

Usage:
```bash
llm-web-parser fetch --urls "..." --features minimal
```

### Wordcount Mode (Default)
**Metadata + keyword extraction** - Recommended for LLM workflows

Same as minimal mode, plus generates top keywords for each URL. Adds ~1 second per URL.

Use this to understand what each URL is about before deciding which to deep-dive.

Usage:
```bash
# Default behavior (wordcount is implicit)
llm-web-parser fetch --urls "..."

# Explicit
llm-web-parser fetch --urls "..." --features wordcount

# Then extract keywords
llm-web-parser corpus extract --session 1
```

### Full-Parse Mode
**Complete content extraction** with confidence scoring

Everything from wordcount mode, plus:
- Full text content blocks with confidence scores
- Section structure and headings
- Block-level content typing (paragraph, code, table, list, etc.)
- Detailed metadata for filtering and analysis

Usage:
```bash
llm-web-parser fetch --urls "..." --features full-parse
```

## URL Sanitization

Automatic "mostly mean mode" - cleans common errors but fails fast on malformed URLs:

**Auto-cleaned:**
- Whitespace (leading/trailing)
- Trailing punctuation (`,`, `.`, `)`, `]`, etc.)
- Markdown links: `[text](url)` → `url`
- Surrounding quotes/brackets

**Hard fails:**
- Literal spaces in URLs (must be `%20`)
- Braces `{}` in domains
- Empty URLs

**Track what was cleaned:**
```bash
llm-web-parser db urls --sanitized
# Shows original vs cleaned URLs
```

## Output Formats

### YAML (Default, Token-Efficient)
```yaml
# Session: 1
- url: https://golang.org
  url_id: 42
  status: success
  title: The Go Programming Language
  confidence: 8.5
  domain_type: commercial
  domain_category: docs/api
  word_count: 450
  estimated_tokens: 180
```

### JSON (Alternative)
```bash
llm-web-parser fetch --urls "..." --format json
```

## Performance

**Minimal mode:**
- 40-50 URLs: 2-5 seconds (optimal batch size)
- 80 URLs: 10-20 seconds (depends on site response times)
- Cache hits: <100ms for any size

**Bottlenecks:**
- 5-second timeout per URL (slow sites impact total time)
- Network latency for fresh fetches
- Cache hits are effectively instant

**Recommendations:**
- Use 40-50 URL batches for optimal performance
- Two-stage workflow: minimal scan → full-parse on selected URLs
- Cache makes repeated queries instant

## Environment

**Database location:**
- Stored in current working directory: `./llm-web-parser.db`
- Auto-creates on first use
- SQLite with WAL mode for performance
- Find location with: `llm-web-parser db path`

**Results directory:**
- Default: `./llm-web-parser-results/`
- Override: `--output-dir /path/to/results`

**Reset everything:**
```bash
rm llm-web-parser.db
rm -rf llm-web-parser-results/
# Auto-recreates on next fetch
```

## Tips for LLMs

1. **Use URL IDs** - Saves 90% tokens vs full URLs
   ```
   lwp db show 42    # Instead of lwp db show https://example.com
   ```

2. **Default to latest session** - Most commands use latest automatically
   ```
   lwp db urls       # No session ID needed
   lwp db get --file=details
   ```

3. **Batch operations** - Get multiple URLs at once
   ```
   lwp db show 42,43,44,45
   ```

4. **Filter in YAML** - Use yq for powerful queries
   ```
   lwp db get --file=details | yq '.[] | select(.confidence >= 7 and .word_count > 500)'
   ```

5. **Session refetch** - Easy multi-stage workflows
   ```
   lwp fetch --urls "..."              # Minimal scan
   lwp fetch --session 1 --features full-parse  # Deep dive
   ```

## Error Handling

**Malformed URLs:**
```
Error: 1 URL(s) are malformed (even after cleanup):
  - invalid url with spaces

Note: URLs are auto-cleaned (whitespace trimmed, trailing punctuation removed, markdown links extracted)
      Spaces in URLs must be pre-encoded as %20. Braces {} in domains are not allowed.
```

**Failed fetches:**
- Logged to `failed-urls.yaml` in session directory
- Retry with: `lwp fetch --session <id> --failed-only`
- Exit codes: 0 = success, 1 = partial failure, 2 = complete failure

**No sessions:**
```
Error: no sessions found. Run 'lwp fetch --urls "..."' first
```

## Examples

See `llm-web-parser-results/FIELDS.yaml` for:
- Complete field reference
- Query examples with yq
- Usage patterns

Run `llm-web-parser --coldstart` for:
- Quick start guide
- Common commands
- Session system invariants

## License

MIT

## Contributing

Pull requests welcome! This tool is actively maintained.
