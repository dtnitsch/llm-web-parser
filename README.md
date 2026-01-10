# LLM Web Parser

**Fast, parallel web scraper that returns structured JSON optimized for LLM consumption**

Parse 40 URLs in 4 seconds. Get hierarchical sections, confidence scores, and keyword extraction—not raw HTML bloat.

```bash
# Fetch 100 competitor pages in parallel, extract structured content
go run main.go
# → results/competitor_com-features-2025-12-30.json (1.2k tokens vs 15k raw HTML)
```

**Performance:** 40 URLs in 4.6s (vs 140s serial) | **Token Savings:** 97% (summary mode) | **Quality:** Auto-escalates cheap → full parsing

---

## Why Use This?

LLM web scraping is broken:

| Problem | LLM Web Parser Solution |
|---------|------------------------|
| **Serial fetching:** 100 URLs = 100 round trips | **Parallel workers:** 40 URLs in 4.6s (8 workers) |
| **HTML bloat:** 2000 tokens/page of markup | **Structured JSON:** Hierarchical sections, typed blocks |
| **No quality signals:** Can't filter nav spam | **Confidence scores:** 0.95 for tables/code, 0.3 for nav |
| **Manual parsing:** LLMs re-parse structure each time | **Smart modes:** Auto-escalates cheap → full when needed |

**Result:** 97% token savings, 30x faster, zero prompt engineering needed for structure extraction

---

## Quick Start

```bash
# 1. Clone and build
git clone https://github.com/dtnitsch/llm-web-parser.git
cd llm-web-parser
go build .

# 2. Fetch and parse URLs
./llm-web-parser fetch --urls "https://docs.python.org/3/library/asyncio.html,https://fastapi.tiangolo.com" --quiet

# 3. Check results
ls llm-web-parser-results/parsed/
# docs_python_org-3-library-asyncio_html-abc123.json
# fastapi_tiangolo_com-def456.json
# summary-2026-01-10.json (← start here!)
```

**Pro tip:** Read `summary-*.json` first (3-5k tokens) to see what's available, then deep-dive into specific files.

---

## Common & Useful Commands

### Working with Parsed JSON

```bash
# Get all page titles
jq -r '.title' llm-web-parser-results/parsed/*.json

# Count high-confidence blocks per file
jq '[.content[].blocks[] | select(.confidence >= 0.7)] | length' file.json

# Extract high-confidence paragraphs (200 char preview)
jq -r '.content[].blocks[] | select(.confidence >= 0.8 and .type == "p") | .text[:200]' file.json

# Find all code blocks across files
jq -r '.content[].blocks[] | select(.type == "code") | .code.content' llm-web-parser-results/parsed/*.json

# Get metadata summary across all files
jq -s 'map({url, tokens: .metadata.estimated_tokens, quality: .metadata.extraction_quality})' llm-web-parser-results/parsed/*.json
```

### Using the Extract Command (Token Savings!)

```bash
# Get only high-confidence content (save 50-80% tokens)
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="conf:>=0.7"

# Get only code blocks
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="type:code"

# Combined: high-confidence paragraphs only
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="conf:>=0.8,type:p"
```

### Fetch Command Options

```bash
# Quiet mode (suppress logs)
./llm-web-parser fetch --urls "https://example.com" --quiet

# Force refetch (ignore cache)
./llm-web-parser fetch --urls "https://example.com" --force-fetch

# Adjust cache age
./llm-web-parser fetch --urls "https://example.com" --max-age "24h"

# Summary output (default, most token-efficient)
./llm-web-parser fetch --urls "https://example.com" --output-mode summary
```

---

## Performance Benchmarks

**Test:** 40 ML research URLs (Wikipedia, Keras, PyTorch, HuggingFace, etc.)
**Hardware:** MacBook M4, 24GB RAM (with Ollama + Docker running)

| Metric | 4 Workers (default) | 8 Workers | Serial (baseline) |
|--------|---------------------|-----------|-------------------|
| **Total time** | 5.053s | 4.594s | 140s |
| **Avg/URL** | 0.136s | 0.123s | 3.5s |
| **Speedup** | 27.7x | 30.5x | 1x |
| **Success rate** | 37/40 (92.5%) | 37/40 (92.5%) | N/A |

**Token savings:** 100x with summary mode (7.4k tokens vs 740k for full files)

**Keywords accuracy:** 99.8% match between aggregate counts (MapReduce working correctly across workers)

**Real-world use case:** Analyzed 38 GitHub repo READMEs in 4.6 seconds vs ~2 minutes serial fetch + manual parsing

---

## Output Format

### Summary Manifest (Start Here)

```json
{
  "generated_at": "2025-12-30T12:01:01-05:00",
  "total_urls": 40,
  "successful": 37,
  "failed": 3,
  "aggregate_keywords": ["learning:1153", "ai:571", "neural:542"],
  "results": [
    {
      "url": "https://example.com",
      "file_path": "results/example_com-2025-12-30.json",
      "status": "success",
      "size_bytes": 29765,
      "word_count": 819,
      "estimated_tokens": 327,
      "extraction_quality": "ok",
      "top_keywords": ["neural:23", "networks:16", "learning:7"]
    }
  ]
}
```

**Use summary for:**
- Quick scan of what succeeded/failed
- Token cost estimation (word_count / 2.5)
- Keyword-based filtering ("which pages mention 'API authentication'?")
- Selective deep-dive (only read high-value pages)

### Hierarchical Page Structure

```json
{
  "url": "https://example.com",
  "title": "Example - The Best Product",
  "content": [
    {
      "id": "section-1",
      "heading": {
        "type": "h2",
        "text": "Features",
        "confidence": 0.7
      },
      "level": 2,
      "blocks": [
        {
          "id": "block-1",
          "type": "p",
          "text": "Our product offers 99.9% uptime SLA...",
          "links": [
            {
              "href": "/pricing",
              "text": "See pricing",
              "type": "internal"
            }
          ],
          "confidence": 0.85
        }
      ],
      "children": []
    }
  ],
  "metadata": {
    "content_type": "landing",
    "language": "en",
    "language_confidence": 0.9,
    "word_count": 1245,
    "estimated_read_min": 5.5,
    "section_count": 8,
    "block_count": 42,
    "extraction_mode": "full",
    "extraction_quality": "ok"
  }
}
```

### What You Get

| Feature | Value | Use Case |
|---------|-------|----------|
| **Hierarchical sections** | H1 → H2 → H3 nesting | Query "all H2 sections" without re-parsing |
| **Confidence scores** | 0.95 (code/tables) → 0.30 (nav spam) | Filter low-signal content |
| **Link classification** | `internal` vs `external` | Depth-first crawling, citation extraction |
| **Content type** | `documentation`, `article`, `landing` | Adjust prompts per page type |
| **Language detection** | Language + confidence score | Skip non-English content |
| **Extraction quality** | `ok` / `low` / `degraded` | Auto-retry with full mode |
| **Token estimation** | `word_count / 2.5` | Budget LLM costs before reading |
| **Top keywords** | Per-URL + aggregate | Filter pages by topic |

---

## Architecture

### How It Works

```
URLs → Worker Pool (8 parallel) → Smart Parser → Structured JSON + Summary
                                       ↓
                                Auto-escalates
                                cheap → full
                                       ↓
                                  MapReduce
                                  Keywords
```

**Three-phase pipeline:**

1. **Parallel Fetch** (4-8 workers)
   - Concurrent HTTP requests with timeouts
   - Failed URLs don't block batch
   - File size caching to avoid redundant I/O

2. **Smart Parsing** (two-tier strategy)
   - **Cheap mode** (default): Fast, flat structure, good for text-heavy pages
   - **Full mode** (auto-escalates): Rich hierarchy, tables, code blocks, citations
   - Auto-detects quality issues and re-parses when needed

3. **MapReduce Analytics**
   - Map: Extract word frequencies per page
   - Reduce: Aggregate keywords across all pages
   - Output: Top 25 keywords with counts

**Performance characteristics:**
- **Scaling:** 8 workers = 1.37x speedup (non-linear due to I/O)
- **Quality:** 92.5% success rate on real-world URLs
- **Efficiency:** Summary manifest eliminates 38 redundant os.Stat() calls (saves ~2.5s)

---

## Project Structure

```
llm-web-parser/
├── main.go              # Worker pool orchestration
├── config.yaml          # URLs + worker count
├── models/              # Page, Section, Metadata types
├── pkg/
│   ├── fetcher/        # Parallel HTTP client
│   ├── parser/         # HTML → JSON (cheap/full modes)
│   ├── storage/        # File I/O + caching
│   ├── manifest/       # Summary generation
│   ├── mapreduce/      # Keyword aggregation + filtering
│   └── analytics/      # Word frequency stats
└── results/            # Generated JSON + summary
```

**Key design decisions:**
- Separation of concerns (orchestration in main.go, logic in packages)
- Caching layer to avoid redundant I/O
- Conservative keyword filtering (removes malformed tokens, keeps technical terms)

---

## Use Cases

### 1. Competitive Analysis
```yaml
urls:
  - https://competitor1.com/features
  - https://competitor2.com/features
  - https://competitor3.com/features
```
**→** Structured comparison in 5 seconds. LLM prompt: "Compare features where confidence > 0.7"

### 2. Documentation Aggregation
```yaml
urls:
  - https://docs.example.com/api/auth
  - https://docs.example.com/api/users
  - https://docs.example.com/sdk/python
```
**→** Unified API reference. LLM prompt: "Extract all code blocks (confidence == 0.95)"

### 3. Recursive Crawling
```bash
# 1. Fetch root page
# 2. Extract internal links with confidence > 0.5
# 3. Add to config.yaml, re-run
```
**→** Depth-first blog archive crawl with preserved structure

### 4. Trend Analysis
40 news articles → MapReduce → Top 25 keywords across all content

---

## Advanced Configuration

**Custom worker count** (edit `config.yaml`):
```yaml
worker_count: 8  # Default: 4
```

**Force full parsing** (edit `main.go:72`):
```go
Mode: models.ParseModeFull,  // Skip cheap mode
```

**Quality filtering** (post-process summary):
```bash
jq '.results[] | select(.extraction_quality == "ok" and .word_count > 100)' summary-*.json
```

---

## Design Principles

1. **LLM-First:** Confidence scores guide attention, hierarchical structure enables selective querying
2. **Fail Gracefully:** Auto-escalation, per-URL errors don't block batch, quality signals prevent silent failures
3. **Production-Grade:** Timeouts, caching, separation of concerns, 92.5% success rate on real-world URLs

---

## Comparison to Alternatives

| Feature | LLM Web Parser | BeautifulSoup | Jina Reader | Firecrawl |
|---------|---------------|---------------|-------------|-----------|
| **Parallel fetching** | ✅ (4 workers) | ❌ | ✅ | ✅ |
| **Confidence scoring** | ✅ | ❌ | ❌ | ❌ |
| **Hierarchical sections** | ✅ | ❌ | ❌ | ⚠️ (flat markdown) |
| **Auto quality detection** | ✅ | ❌ | ⚠️ | ⚠️ |
| **Content type detection** | ✅ | ❌ | ❌ | ❌ |
| **Link classification** | ✅ (internal/external) | ❌ | ❌ | ❌ |
| **Self-hosted** | ✅ | ✅ | ❌ (API) | ⚠️ |
| **LLM-optimized output** | ✅ | ❌ | ⚠️ | ⚠️ |
| **MapReduce analytics** | ✅ | ❌ | ❌ | ❌ |

---

## Limitations & Roadmap

**Current limitations:**
- ❌ JavaScript-heavy SPAs (use Playwright/Puppeteer instead)
- ❌ No robots.txt respect
- ❌ No per-domain rate limiting
- ❌ No URL deduplication

**Planned (see `todos.yaml`):**
- ⏳ CLI args + stdout output (P0 - eliminates config.yaml friction)
- ⏳ Extract subcommand for selective filtering (P0)
- ⏳ Retry logic with exponential backoff (P0-post-launch)
- ⏳ 65 golangci-lint fixes (errcheck, gosec, revive, etc.)

---

## Dependencies

- `goquery` - HTML parsing | `go-readability` - Article extraction | `lingua-go` - Language detection

All production-grade, battle-tested libraries.

---

## Contributing

Pull requests welcome! See `todos.yaml` for prioritized tasks.

**High-value areas:**
- CLI argument parsing (P0)
- Summary output mode (P0)
- Retry logic and rate limiting (P1)

---

## License

MIT - See LICENSE file

---

**Questions?** See `LLM-USAGE.md` for LLM integration patterns | `todos.yaml` for roadmap
