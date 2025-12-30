# LLM Web Parser

**A production-grade web scraper optimized for LLM consumption**

Fetch and parse dozens of URLs in parallel, returning structured JSON with hierarchical sections, confidence scores, and rich metadata—saving 70-80% of LLM context tokens compared to raw HTML or flat text approaches.

---

## Why This Exists

LLMs waste massive context on unstructured web scraping:

- **Serial fetching:** 100 URLs = 100 WebFetch calls = 100 round trips
- **Raw HTML bloat:** (estimates from 3 LLMs) 2000 tokens per page of irrelevant markup
- **No structure:** LLMs re-parse headings, links, tables every time
- **No quality signals:** Can't distinguish high-signal content from navigation spam

**LLM Web Parser solves this:**

- **Parallel fetching:** 40 URLs in 3.7 seconds (8 workers) or 5 seconds (4 workers default)
- **Structured output:** Hierarchical sections, typed blocks, extracted links
- **Confidence scoring:** 0.95 for tables/code, 0.3 for nav spam
- **Smart parsing:** Auto-escalates from cheap → full mode when extraction quality is low

---

## Quick Start

### Installation

```bash
git clone https://github.com/dtnitsch/llm-web-parser.git
cd llm-web-parser
go mod download
```

### Basic Usage

1. Create `config.yaml`:
2. 
```yaml
urls:
  - https://example.com
  - https://docs.example.com/api
  - https://competitor.com/features
```

2. Run the parser:
```bash
go run main.go
```

3. Find structured JSON in `results/`:
4. 
```bash
ls results/
# example_com-2025-12-27.json
# docs_example_com-2025-12-27.json
# competitor_com-2025-12-27.json
```

---

## Output Format

### Hierarchical Sections

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

### Key Features of Output

**Hierarchical Structure:**
- Sections nest based on heading levels (H1 → H2 → H3)
- Query "all H2 sections" or "links in code blocks" without re-parsing

**Confidence Scores:**
- `0.95` - Tables, code blocks (high-signal structured content)
- `0.85` - Dense paragraphs (40+ words, few links)
- `0.50` - Medium paragraphs (15-40 words)
- `0.30` - Link-heavy text (navigation, footers)

**Link Classification:**
- `internal` - Same domain links (for depth-first crawling)
- `external` - Cross-domain links (for citation extraction)

**Content Type Detection:**
- `documentation` - High code/table density
- `article` - Long-form text (1200+ words, 8+ sections)
- `landing` - Short, promotional (< 500 words)

**Metadata:**
- Word count, estimated reading time
- Language detection with confidence
- Extraction quality signals (ok / low / degraded)

---

## Architecture

### Two-Tier Parsing Strategy

```
┌─────────────────────────────────────┐
│  ParseModeCheap (default)           │
│  - Fast, minimal structure          │
│  - Flat content blocks              │
│  - Good for text-heavy pages        │
└─────────────────────────────────────┘
                 │
                 │ Auto-escalates if
                 │ extraction_quality == "low"
                 ▼
┌─────────────────────────────────────┐
│  ParseModeFull                      │
│  - Rich hierarchy (nested sections) │
│  - Tables, code blocks, citations   │
│  - Link extraction per block        │
└─────────────────────────────────────┘
```

**Why this matters:**
- Start cheap, escalate only when needed
- 80% of pages parse clean in cheap mode
- 20% with complex structure get full treatment
- Saves compute without sacrificing quality

### Worker Pool Architecture

```
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│ Worker 1 │    │ Worker 2 │    │ Worker 3 │    │ Worker 4 │
└────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘
     │               │               │               │
     └───────────────┴───────────────┴───────────────┘
                      │
                 Jobs Channel
              (100 URLs queued)
                      │
                 Results Channel
              (Structured Pages)
```

**Concurrency:**
- 4 workers by default (configurable in `main.go`)
- Parallel fetching + parsing
- Graceful error handling (failed URLs don't block others)
- **Scaling:** 8 workers = 1.37x faster (non-linear due to I/O overhead)
  - 4 workers: 40 URLs in 5.053s
  - 8 workers: 40 URLs in 3.685s

### MapReduce Analytics Pipeline

```
┌────────────────┐
│  Fetched Pages │
└───────┬────────┘
        │
        ▼
┌────────────────┐
│  Map Stage     │  ← Extract word frequencies per page
└───────┬────────┘
        │
        ▼
┌────────────────┐
│ Shuffle Stage  │  ← Group by word
└───────┬────────┘
        │
        ▼
┌────────────────┐
│ Reduce Stage   │  ← Aggregate totals
└───────┬────────┘
        │
        ▼
 Top 25 Words (across all pages)
```

**Use cases:**
- Trend analysis across competitor sites
- Keyword extraction from documentation sets
- Content quality signals (jargon density, readability)

---

## Project Structure

```
llm-web-parser/
├── main.go                 # Entry point, worker pool, MapReduce orchestration
├── config.yaml             # List of URLs to fetch
├── models/                 # Data structures
│   ├── page.go            # Page, Section, ContentBlock, Link types
│   ├── page_meta.go       # PageMetadata with quality signals
│   ├── parse_mode.go      # ParseModeCheap, ParseModeFull
│   ├── parse_request.go   # ParseRequest input type
│   └── config.go          # Config file loader
├── pkg/
│   ├── fetcher/           # HTTP client with timeouts
│   ├── parser/            # HTML → Structured Page conversion
│   │   └── parser.go      # Two-tier parsing logic
│   ├── storage/           # Filesystem artifact storage
│   ├── analytics/         # Word frequency, stats
│   └── mapreduce/         # Map/Reduce framework
└── results/               # Generated JSON outputs
    └── example_com-2025-12-27.json
```

---

## Advanced Usage

### Parse Modes

**Explicit mode override:**
```go
page, err := parser.Parse(models.ParseRequest{
    URL:  "https://docs.example.com",
    HTML: htmlString,
    Mode: models.ParseModeFull, // Force full parsing
})
```

**Citation requirements:**
```go
page, err := parser.Parse(models.ParseRequest{
    URL:              "https://research.example.com",
    HTML:             htmlString,
    RequireCitations: true, // Auto-escalates to full mode
})
```

### Custom Worker Count

Edit `main.go`:
```go
const numWorkers = 8 // Increase for more parallelism
```

### Quality Signals

```go
page.ComputeMetadata()

if page.Metadata.ExtractionQuality == "low" {
    // Re-fetch with full mode or skip this URL
}

if page.Metadata.LanguageConfidence < 0.75 {
    // Language detection uncertain
}

if page.Metadata.ContentType == "landing" {
    // Marketing page, may have low information density
}
```

---

## Use Cases

### 1. Competitive Analysis
```yaml
urls:
  - https://competitor1.com/features
  - https://competitor1.com/pricing
  - https://competitor2.com/features
  - https://competitor2.com/pricing
  - https://competitor3.com/features
  - https://competitor3.com/pricing
```

**Output:** Structured comparison of features, pricing, messaging across competitors.

**LLM prompt:**
```
Analyze results/*.json and create a competitive comparison table.
Focus on content blocks with confidence > 0.7.
```

### 2. Documentation Aggregation
```yaml
urls:
  - https://docs.example.com/api/auth
  - https://docs.example.com/api/users
  - https://docs.example.com/api/billing
  - https://docs.example.com/sdk/python
  - https://docs.example.com/sdk/javascript
```

**Output:** Hierarchical API docs with code blocks, tables, internal links preserved.

**LLM prompt:**
```
Create a unified API reference from results/*.json.
Extract all code blocks (confidence == 0.95) and API endpoints.
```

### 3. Link Following (Depth-First Crawling)
```yaml
urls:
  - https://blog.example.com
```

**After initial fetch:**
```
Read results/blog_example_com-*.json
Extract all internal links from blocks with confidence > 0.5
Add to config.yaml
Re-run parser
```

**Result:** Recursive crawl of blog archive, preserving structure.

### 4. Trend Analysis
Run MapReduce pipeline on 100 news articles → Top 25 trending keywords.

---

## Design Philosophy

### 1. **Better, Faster, Cheaper**
- **Better:** Structured output beats flat text
- **Faster:** Parallel > Serial
- **Cheaper:** 10x token savings

### 2. **LLM-First Design**
- Confidence scores guide LLM attention
- Hierarchical sections enable selective querying
- Content type hints adjust LLM prompts

### 3. **Fail Gracefully**
- Auto-escalation when cheap mode fails
- Per-URL error handling (don't block batch)
- Quality signals prevent silent failures

### 4. **Production-Ready**
- Timeouts on HTTP requests
- Idempotent file naming (enables caching)
- Separation of concerns (models, fetcher, parser, storage)

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

## Limitations

**Not designed for:**
- JavaScript-heavy SPAs (use headless browser like Playwright)
- Rate-limited APIs (add retry/backoff logic)
- Real-time streaming (batch-oriented design)

**Known issues:**
- No robots.txt respect (add if crawling large sites)
- No deduplication (same URL from different sources)
- No per-domain rate limiting

**See `todos.yaml` for roadmap.**

---

## Dependencies

- **`github.com/PuerkitoBio/goquery`** - HTML parsing
- **`github.com/go-shiori/go-readability`** - Article extraction
- **`github.com/pemistahl/lingua-go`** - Language detection

All battle-tested, production-grade libraries.

---

## License

MIT

---

## Contributing

See `todos.yaml` for prioritized enhancements. Pull requests welcome!

**High-value contributions:**
- P1: HTML-to-text ratio, block type distribution, link density metrics
- P0 (done): Extraction quality signals, language detection, link classification

---

## Questions?

This tool is designed for LLM-driven research workflows. See `LLM-USAGE.md` for detailed integration guidance.
