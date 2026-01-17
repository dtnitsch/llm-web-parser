# CLI Reference

**llm-web-parser** - Fast, parallel web scraper for LLM consumption

## Commands

### `fetch` - Fetch and parse URLs

```bash
./llm-web-parser fetch [flags]
```

| Flag | Alias | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--urls` | `-u` | string | | Comma-separated list of URLs to fetch |
| `--features` | | string | `` | Comma-separated features to enable: `full-parse`, `wordcount`. Default: minimal mode (metadata only) |
| `--workers` | `-w` | int | 8 | Number of concurrent workers |
| `--format` | `-f` | string | `yaml` | Output format: `json` or `yaml` (YAML is more token-efficient) |
| `--output-mode` | | string | `tier2` | Output mode: `tier2`, `summary`, `full`, or `minimal`. tier2 = index to stdout + details file |
| `--max-age` | | duration | `1h` | Maximum age for cached artifacts (e.g., `24h`, `30m`) |
| `--force-fetch` | | bool | `false` | Force refetch, ignore cache |
| `--output-dir` | | string | `llm-web-parser-results` | Base directory for artifacts |
| `--summary-version` | | string | `v1` | Summary format: `v1` (verbose) or `v2` (terse, 40% smaller) |
| `--summary-fields` | | string | `` | Comma-separated fields to include (e.g., `url,tokens,quality`). Empty = all fields |
| `--quiet` | | bool | `true` | Suppress log output (only errors and final output). Use `--quiet=false` for verbose logs |

**Examples:**

```bash
# Fetch single URL (quiet by default, outputs tier2 YAML)
./llm-web-parser fetch --urls "https://example.com"

# Fetch with verbose logging
./llm-web-parser fetch --urls "https://example.com" --quiet=false

# Fetch multiple URLs with 4 workers
./llm-web-parser fetch --urls "https://a.com,https://b.com" --workers 4

# Force refetch ignoring cache
./llm-web-parser fetch --urls "https://example.com" --force-fetch

# Adjust cache TTL to 24 hours
./llm-web-parser fetch --urls "https://example.com" --max-age "24h"

# Use JSON instead of YAML (for backward compatibility)
./llm-web-parser fetch --urls "https://example.com" --format json

# Use terse v2 format (40% smaller summary output)
./llm-web-parser fetch --urls "https://example.com" --summary-version v2

# Field filtering for minimal output (84% reduction)
./llm-web-parser fetch --urls "https://example.com" --summary-fields "url,status,estimated_tokens"

# Ultra-minimal v2 with field filtering (works with verbose or terse names)
./llm-web-parser fetch --urls "https://example.com" --summary-version v2 --summary-fields "u,tk,q"
```

---

### `analyze` - Parse cached HTML on-demand

**NEW:** Multi-stage workflow command for selective deep-dive parsing

```bash
./llm-web-parser analyze [flags]
```

| Flag | Alias | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--urls` | `-u` | string | | Comma-separated list of URLs to re-analyze from cache |
| `--features` | | string | `full-parse` | Features to enable: `full-parse`, `wordcount` |
| `--output-dir` | | string | `llm-web-parser-results` | Base directory for cached artifacts |
| `--max-age` | | duration | `24h` | Maximum age for cached HTML. Use `0s` to require fresh cache |
| `--quiet` | | bool | `true` | Suppress log output (only errors and final output). Use `--quiet=false` for verbose logs |

**Use Case:**

The `analyze` command enables a powerful multi-stage workflow:

1. **Step 1: Fetch quickly** - Use default minimal mode to fetch 100s of URLs (metadata only, very fast)
2. **Step 2: Scan & decide** - LLM scans metadata (domain type, categories, confidence) to identify interesting URLs
3. **Step 3: Deep dive** - Use `analyze` to selectively parse only the relevant URLs with full-parse

**Examples:**

```bash
# Multi-stage workflow
# 1. Fetch 100 URLs in minimal mode (fast: metadata only, outputs tier2 YAML)
./llm-web-parser fetch --urls "..." > index.yaml

# 2. LLM scans index, decides CDC and ArXiv are worth deep analysis
# 3. Analyze specific URLs from cache with full parsing
./llm-web-parser analyze --urls "https://www.cdc.gov,https://arxiv.org/abs/2103.00020" --features full-parse

# Analyze single URL
./llm-web-parser analyze --urls "https://example.com" --features full-parse

# Analyze with strict cache requirements (must be fresh)
./llm-web-parser analyze --urls "https://example.com" --max-age "0s"
```

**Performance:**

- Minimal fetch (100 URLs): ~2-3 seconds
- Analyze selected (5 URLs): ~0.5 seconds
- **Total: 3.5s vs 15s for full-parse all 100 URLs**

---

### `extract` - Filter existing parsed JSON files

```bash
./llm-web-parser extract [flags]
```

| Flag | Alias | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--from` | `-i` | []string | | Path or glob pattern to parsed JSON files (can specify multiple) |
| `--strategy` | `-s` | string | | Filtering strategy (see below) |

**Strategy Syntax:**

- **Confidence filtering:** `conf:>=0.7` (range: 0.0-1.0)
- **Type filtering:** `type:p\|code\|table` (pipe-separated list)
- **Combined:** `conf:>=0.8,type:p` (comma-separated)

**Supported Types:** `p`, `li`, `code`, `table`, `h1`, `h2`, `h3`, `h4`, `h5`, `h6`

**Examples:**

```bash
# Extract high-confidence content only
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="conf:>=0.7"

# Extract only code blocks
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="type:code"

# Extract high-confidence paragraphs
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="conf:>=0.8,type:p"

# Extract all headings (h2 only)
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="type:h2"

# Multiple input patterns
./llm-web-parser extract --from 'results1/*.json' --from 'results2/*.json' --strategy="conf:>=0.5"
```

---

## Parse Modes & Features

**Breaking Change (v0.x → v1.0):** Default parsing mode changed from `full-parse` to `minimal`.

The tool now operates in a multi-stage workflow optimized for LLM-driven analysis:

### Minimal Mode (Default)

**What it does:**
- Fetches and caches HTML
- Extracts only "free" metadata (no content parsing):
  - Title, description, author, published date (from `go-readability`)
  - Domain type, category, country (smart detection)
  - Academic signals (DOI, ArXiv, LaTeX markers)
  - HTTP metadata (status, content-type, redirects)
- **No content blocks** - saves to cache for later analysis

**When to use:** Fetching large batches of URLs (100-1000+) to quickly scan and filter

**Speed:** 2-3x faster than full-parse, 9x less CPU

**Example:**
```bash
./llm-web-parser fetch --urls "..."
```

### Full-Parse Mode

**What it does:**
- Everything from minimal mode
- Plus: hierarchical content parsing (sections, headings, blocks)
- Confidence scores (0-1.0) for each block
- Link extraction and classification
- Wordcount and keyword extraction

**When to use:** When you need the actual content for LLM consumption

**Enable with:**
```bash
./llm-web-parser fetch --urls "..." --features full-parse
```

### Wordcount Feature

**What it does:**
- Enables keyword/word frequency analysis
- Requires at least "cheap" parsing mode (flat blocks)
- Used for MapReduce aggregation across URLs

**Enable with:**
```bash
./llm-web-parser fetch --urls "..." --features wordcount
```

### Multi-Stage Workflow Pattern

**Recommended for 50+ URLs:**

```bash
# Step 1: Fetch all URLs in minimal mode (fast, outputs tier2 YAML by default)
./llm-web-parser fetch --urls "url1,url2,...,url100" > index.yaml

# Step 2: LLM scans index.yaml, filters by confidence/category
yq '.[] | select(.conf >= 7.0 and .cat == "academic/ai")' index.yaml

# Step 3: Analyze only the relevant URLs with full parsing
./llm-web-parser analyze --urls "url5,url12,url47" --features full-parse
```

**Result:** 3-5x faster than parsing everything upfront, with same final output for selected URLs.

---

## Field Filtering

The `--summary-fields` flag allows you to select specific fields for ultra-minimal output.

**Available Fields (v1/verbose names):**
- `url`, `file_path`, `status`, `error`
- `file_size_bytes`, `estimated_tokens`
- `content_type`, `extraction_quality`
- `confidence_distribution`, `block_type_distribution`

**Available Fields (v2/terse names):**
- `u`, `p`, `s`, `e`
- `sz`, `tk`
- `ct`, `q`
- `cd`, `bd`

**Smart Mapping:** You can use verbose names even with v2 format:
```bash
# Both produce the same output:
./llm-web-parser fetch --urls "..." --summary-version v2 --summary-fields "u,tk,q"
./llm-web-parser fetch --urls "..." --summary-version v2 --summary-fields "url,estimated_tokens,extraction_quality"
```

**Common Use Cases:**
```bash
# Just check status (61 bytes, 84% reduction)
--summary-fields "url,status"

# Token budget check
--summary-fields "url,estimated_tokens"

# Quality audit
--summary-fields "url,extraction_quality,estimated_tokens"

# Error debugging
--summary-fields "url,status,error"
```

---

## Output Formats

### Summary Mode (Default)

Structured JSON with per-URL metadata:

```json
{
  "status": "success",
  "results": [
    {
      "url": "https://example.com",
      "file_path": "llm-web-parser-results/parsed/example_com-abc123.json",
      "status": "success",
      "file_size_bytes": 12345,
      "estimated_tokens": 450,
      "content_type": "documentation",
      "extraction_quality": "ok",
      "confidence_distribution": {"high": 20, "medium": 15, "low": 5},
      "block_type_distribution": {"p": 25, "code": 10, "table": 5}
    }
  ],
  "stats": {
    "total_urls": 10,
    "successful": 9,
    "failed": 1,
    "total_time_seconds": 4.2,
    "top_keywords": ["api:45", "authentication:23", ...]
  }
}
```

### Full Mode

Returns complete parsed JSON for each URL (same as individual parsed files).

### Tier2 Mode (Two-Tier Summary System)

**Designed for handling 100-1000 URLs efficiently**

Outputs two complementary summaries:

**1. Summary Index** (stdout, YAML):
- Only successful fetches (HTTP 200, 301)
- Ultra-minimal: ~150 bytes/URL
- Fields: `url`, `cat` (category), `conf` (confidence), `title`, `desc`, `tokens`
- Includes usage header with `yq` examples

**2. Summary Details** (file: `summary-details-YYYY-MM-DD.yaml`):
- All URLs (including failures)
- Full metadata: ~400 bytes/URL
- Enriched with smart detection, academic signals, HTTP metadata

**Example:**
```bash
# Fetch multiple URLs (tier2 YAML output by default)
./llm-web-parser fetch --urls "https://cdc.gov,https://arxiv.org,https://github.com"

# Filter by confidence >= 7.0
./llm-web-parser fetch --urls "..." | yq '.[] | select(.conf >= 7.0)'

# Filter by category
./llm-web-parser fetch --urls "..." | yq '.[] | select(.cat == "gov/health")'

# Get high-token documents
./llm-web-parser fetch --urls "..." | yq '.[] | select(.tokens > 500) | .url'
```

**Scaling:**
- 100 URLs: ~15KB index + ~40KB details (vs ~470KB full parse)
- 1000 URLs: ~150KB index + ~400KB details
- LLMs can scan index, filter, then drill down to details only for relevant URLs

**Enriched Metadata Fields:**

The tier2 system adds smart detection with zero API cost:

- **Domain Type**: `gov`, `edu`, `academic`, `commercial`, `mobile`
- **Domain Category**: `gov/health`, `academic/ai`, `news/tech`, `docs/api`, `blog`, `general`
- **Country**: TLD-based detection (`us`, `uk`, `de`, `jp`, etc.)
- **Confidence**: 0-10 scale based on signal strength
- **Academic Signals**: DOI patterns, ArXiv IDs, LaTeX markers, citations, references
- **Academic Score**: 0-10 composite score for academic content
- **Author**: From page metadata
- **Published Date**: ISO-8601 format
- **Site Name**: E.g., "arXiv.org", "GitHub"

All detection is "free" - leveraging HTTP headers, URL structure, and go-readability output.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success - all URLs processed successfully |
| 1 | Partial failure - some URLs failed |
| 2 | Complete failure - all URLs failed or critical error |

---

## Artifact Structure

```
llm-web-parser-results/
├── raw/
│   └── example_com-abc123.html     # Cached raw HTML (respects --max-age)
├── parsed/
│   └── example_com-abc123.json     # Structured JSON output
└── summary-2026-01-10.json         # Daily summary manifest
```

---

## Common Workflows

### Batch fetch + filter
```bash
# 1. Fetch all URLs (outputs tier2 YAML by default)
./llm-web-parser fetch --urls "https://a.com,https://b.com"

# 2. Extract high-confidence content only
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="conf:>=0.7"
```

### Recursive crawling
```bash
# 1. Fetch seed URL
./llm-web-parser fetch --urls "https://docs.example.com"

# 2. Extract internal links with jq
jq -r '.content[].blocks[].links[]? | select(.type == "internal") | .href' llm-web-parser-results/parsed/*.json > links.txt

# 3. Fetch discovered links
./llm-web-parser fetch --urls "$(cat links.txt | tr '\n' ',')"
```

### Cache management
```bash
# Force fresh fetch (ignore cache)
./llm-web-parser fetch --urls "https://example.com" --force-fetch

# Set long cache TTL (24 hours)
./llm-web-parser fetch --urls "https://example.com" --max-age "24h"

# Set short cache TTL (5 minutes)
./llm-web-parser fetch --urls "https://example.com" --max-age "5m"

# Clear cache manually
rm -rf llm-web-parser-results/raw/*
```

---

## See Also

- [SCHEMA.md](./SCHEMA.md) - JSON output schema reference
- [LLM-USAGE.md](../LLM-USAGE.md) - LLM integration patterns
- [README.md](../README.md) - Quick start and examples
