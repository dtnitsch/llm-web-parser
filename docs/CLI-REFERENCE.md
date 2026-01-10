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
| `--workers` | `-w` | int | 8 | Number of concurrent workers |
| `--format` | `-f` | string | `json` | Output format: `json` or `yaml` |
| `--output-mode` | | string | `summary` | Output mode: `summary`, `full`, or `minimal` |
| `--max-age` | | duration | `1h` | Maximum age for cached artifacts (e.g., `24h`, `30m`) |
| `--force-fetch` | | bool | `false` | Force refetch, ignore cache |
| `--config` | `-c` | string | `config.yaml` | Path to config file (alternative to `--urls`) |
| `--output-dir` | | string | `llm-web-parser-results` | Base directory for artifacts |
| `--summary-version` | | string | `v1` | Summary format: `v1` (verbose) or `v2` (terse, 40% smaller) |
| `--quiet` | | bool | `false` | Suppress log output (only errors) |

**Examples:**

```bash
# Fetch single URL
./llm-web-parser fetch --urls "https://example.com" --quiet

# Fetch multiple URLs with 4 workers
./llm-web-parser fetch --urls "https://a.com,https://b.com" --workers 4

# Force refetch ignoring cache
./llm-web-parser fetch --urls "https://example.com" --force-fetch

# Use config file instead of --urls
./llm-web-parser fetch --config my-urls.yaml

# Adjust cache TTL to 24 hours
./llm-web-parser fetch --urls "https://example.com" --max-age "24h"

# Use terse v2 format (40% smaller summary output)
./llm-web-parser fetch --urls "https://example.com" --summary-version v2
```

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

## Configuration File (config.yaml)

Alternative to `--urls` flag:

```yaml
urls:
  - https://example.com
  - https://another-example.com
worker_count: 8
```

```bash
./llm-web-parser fetch --config config.yaml
```

---

## Common Workflows

### Batch fetch + filter
```bash
# 1. Fetch all URLs
./llm-web-parser fetch --urls "https://a.com,https://b.com" --quiet

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
