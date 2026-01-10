# LLM Integration Guide

**llm-web-parser** - Session-based web scraper optimized for LLM consumption with enriched metadata and intelligent filtering.

## Quick Start

```bash
# Default: quiet=true, format=yaml, output-mode=tier2
./llm-web-parser --urls "https://www.cdc.gov,https://arxiv.org,https://docs.python.org"

# Output:
# Parsed 3 URLs - 3 success, 0 failed. Summary files in llm-web-parser-results/sessions/2026-01-10T14-30-abc123. Features enabled:
```

**What happens:**
1. Fetches 3 URLs in parallel (~2-3 seconds)
2. Creates session: `llm-web-parser-results/sessions/{timestamp-hash}/`
3. Writes `summary-index.yaml` (minimal, ~150 bytes/URL)
4. Writes `summary-details.yaml` (full enriched metadata, ~400 bytes/URL)
5. Updates `index.yaml` with session info
6. Prints concise stats to stdout

---

## Understanding Sessions

### Session Structure

```
llm-web-parser-results/
├── FIELDS.yaml                    ← Field reference (read this first!)
├── index.yaml                     ← All sessions registry
├── sessions/
│   ├── 2026-01-10T14-30-abc123/  ← Timestamp-first for discovery
│   │   ├── summary-index.yaml    ← Minimal scannable data
│   │   ├── summary-details.yaml  ← Full enriched metadata
│   │   └── failed-urls.yaml      ← Failed URLs (only if errors occurred)
│   └── 2026-01-10T15-45-def456/
│       ├── summary-index.yaml
│       ├── summary-details.yaml
│       └── failed-urls.yaml
├── raw/                          ← Shared HTML cache
└── parsed/                       ← Shared JSON cache
```

### Finding Sessions

```bash
# Get latest session ID
SESSION=$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)

# List all sessions
yq '.sessions[] | .session_id + " (" + (.url_count | tostring) + " URLs)"' llm-web-parser-results/index.yaml

# Find sessions by date
yq '.sessions[] | select(.session_id | test("2026-01-10"))' llm-web-parser-results/index.yaml
```

### Session Cache (Instant Retrieval)

```bash
# First run: fetches and parses
./llm-web-parser --urls "https://www.cdc.gov"
# Parsed 1 URLs - 1 success, 0 failed...

# Second run: instant cache hit!
./llm-web-parser --urls "https://www.cdc.gov"
# Session cache hit! Results from: llm-web-parser-results/sessions/2026-01-10T14-30-f9dc16d6d37a
```

**Same URLs = Same session hash = Instant retrieval (no re-fetching)**

### Error Handling & Failed URLs

#### URL Pre-Validation (Fail Fast)

All URLs are validated **before** any fetching begins. If any URL is malformed, the tool exits immediately with all invalid URLs listed:

```bash
./llm-web-parser --urls "not-a-url,ftp://badscheme.com,https://good.com"

# Output:
# Error: 2 URL(s) are malformed:
#   - not-a-url
#   - ftp://badscheme.com
```

**Validation checks:**
- Valid HTTP/HTTPS scheme
- Proper domain format
- No invalid characters
- URL structure integrity

#### Runtime Failures (failed-urls.yaml)

If URLs fail during fetch/parse (404, timeout, network error), they're logged to `failed-urls.yaml`:

```yaml
failed_urls:
  - url: https://example.com/404
    status_code: 404
    error_type: http_error
    error_message: "Not Found"

  - url: https://timeout.example.com
    status_code: 0
    error_type: network_error
    error_message: "connection timeout after 30s"
```

**Error types:**
- `http_error` - 4xx/5xx status codes
- `network_error` - Connection failures
- `timeout` - Request timeouts
- `parse_error` - HTML parsing failed
- `fetch_error` - Generic fetch failure

#### Querying Failed URLs

```bash
# Get latest session
SESSION=$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)

# View all failures
yq '.failed_urls[]' llm-web-parser-results/sessions/$SESSION/failed-urls.yaml

# Filter by error type
yq '.failed_urls[] | select(.error_type == "http_error")' \
  llm-web-parser-results/sessions/$SESSION/failed-urls.yaml

# Count failures by type
yq '.failed_urls | group_by(.error_type) | .[] | {
  "type": .[0].error_type,
  "count": length
}' llm-web-parser-results/sessions/$SESSION/failed-urls.yaml
```

#### Token-Efficient Error Output

**Before:** Errors printed to stderr (thousands of tokens with many failures)
```
ERROR: failed to fetch https://site1.com: timeout
ERROR: failed to fetch https://site2.com: 404 not found
ERROR: failed to fetch https://site3.com: connection refused
...
```

**After:** Concise summary + structured YAML file
```
Parsed 50 URLs - 45 success, 5 failed (see sessions/2026-01-10T14-30-abc123/failed-urls.yaml).
```

**Token savings:** ~95% reduction for error-heavy batches

---

## Field Reference

**Read this first:** `llm-web-parser-results/FIELDS.yaml`

This auto-generated file documents:
- All available fields and their types
- Valid values for enums (domain_type, domain_category, etc.)
- Query examples using yq
- Difference between summary-index and summary-details

### Key Fields for Filtering

```yaml
# Domain Classification (FREE metadata - no extra API calls!)
domain_type: [gov, edu, academic, commercial, mobile, unknown]
domain_category: [gov/health, academic/ai, academic/general, news/tech, docs/api, blog, general]
country: us  # 2-letter code or "unknown"
confidence: 8.0  # 0-10 quality/credibility score

# Academic Signals (boolean, only present if true)
has_doi: true
has_arxiv: true
has_latex: true
has_citations: true
has_references: true
has_abstract: true
academic_score: 7.5  # 0-10 composite score

# Visual Metadata (token-efficient!)
has_favicon: true  # Boolean (3 tokens vs 30 for URL)
image_count: 23    # Count (4 tokens vs 600 for URLs)

# Content Metrics
word_count: 1245
estimated_tokens: 498
section_count: 8
language: en
language_confidence: 0.95
```

---

## Query Patterns (yq)

### Basic Session Access

```bash
# Set session directory for queries
SESSION_DIR="llm-web-parser-results/sessions/$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)"

# Quick scan - what did we fetch?
yq '.[] | .url + ": " + .title' "$SESSION_DIR/summary-index.yaml"

# Full metadata for analysis
yq '.' "$SESSION_DIR/summary-details.yaml"
```

### Filter by Domain Type

```bash
# Government sites only
yq '.[] | select(.domain_type == "gov")' "$SESSION_DIR/summary-details.yaml"

# Academic sites with high confidence
yq '.[] | select(.domain_type == "academic" and .confidence >= 7)' "$SESSION_DIR/summary-details.yaml"

# Government health sites (specific category)
yq '.[] | select(.domain_category == "gov/health")' "$SESSION_DIR/summary-details.yaml"
```

### Filter by Academic Signals

```bash
# Papers with citations
yq '.[] | select(.has_citations)' "$SESSION_DIR/summary-details.yaml"

# High-quality academic papers
yq '.[] | select(.has_doi and .academic_score >= 7)' "$SESSION_DIR/summary-details.yaml"

# ArXiv papers with abstracts
yq '.[] | select(.has_arxiv and .has_abstract)' "$SESSION_DIR/summary-details.yaml"
```

### Filter by Content Type

```bash
# Documentation sites
yq '.[] | select(.content_type == "documentation" or .domain_category == "docs/api")' "$SESSION_DIR/summary-details.yaml"

# Visual-heavy content (landing pages, galleries)
yq '.[] | select(.image_count > 10)' "$SESSION_DIR/summary-details.yaml"

# Text-heavy technical content (low images, high structure)
yq '.[] | select(.image_count < 3 and .section_count > 5)' "$SESSION_DIR/summary-details.yaml"
```

### Token Budget Analysis

```bash
# Total tokens for all pages
yq '[.[] | .estimated_tokens] | add' "$SESSION_DIR/summary-details.yaml"

# Pages under 500 tokens
yq '.[] | select(.estimated_tokens < 500) | {url, tokens: .estimated_tokens}' "$SESSION_DIR/summary-details.yaml"

# Long-form content worth deep reading
yq '.[] | select(.word_count > 1000 and .read_time_min > 5 and .confidence >= 6)' "$SESSION_DIR/summary-details.yaml"
```

### Multi-Session Queries

```bash
# Query across ALL sessions
yq '.[]' llm-web-parser-results/sessions/*/summary-details.yaml | \
  yq '[.] | group_by(.domain_category) | map({category: .[0].domain_category, count: length})'

# Find all government sites across all sessions
yq '.[] | select(.domain_type == "gov")' llm-web-parser-results/sessions/*/summary-details.yaml
```

---

## Common Workflows

### 1. Competitive Analysis

**Goal:** Compare feature pages from 3 competitors

```bash
# Step 1: Fetch competitor pages
./llm-web-parser --urls "https://competitor1.com/features,https://competitor2.com/features,https://competitor3.com/features"

# Step 2: Get session and extract high-confidence content
SESSION_DIR="llm-web-parser-results/sessions/$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)"

# Step 3: Filter by confidence and domain
yq '.[] | select(.confidence >= 7 and .domain_type == "commercial") | {url, title, tokens: .estimated_tokens}' "$SESSION_DIR/summary-details.yaml"

# Step 4: Use extract command for detailed analysis
./llm-web-parser extract --from "llm-web-parser-results/parsed/*.json" --strategy="conf:>=0.7"
```

**Token savings:** Minimal index (~450 bytes) instead of full content (~15KB) for initial scan.

---

### 2. Research Paper Filtering

**Goal:** From 100 ArXiv URLs, find papers with citations worth deep reading

```bash
# Step 1: Fetch 100 papers in minimal mode (fast!)
./llm-web-parser --urls "$(cat arxiv_urls.txt | tr '\n' ',')"
# Output: ~2-3 seconds for metadata extraction

# Step 2: Get session
SESSION_DIR="llm-web-parser-results/sessions/$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)"

# Step 3: Filter by academic signals
yq '.[] | select(.has_citations and .academic_score >= 7 and .word_count > 2000)' "$SESSION_DIR/summary-details.yaml" > relevant_papers.yaml

# Step 4: Extract URLs for deep analysis
yq '.[].url' relevant_papers.yaml > urls_for_analysis.txt

# Step 5: Analyze selected papers with full parsing
./llm-web-parser analyze --urls "$(cat urls_for_analysis.txt | tr '\n' ',')" --features full-parse
```

**Performance:** 3.5s (100 minimal + 5 full-parse) vs 15s (100 full-parse upfront)

---

### 3. Documentation Aggregation

**Goal:** Collect API docs from multiple sources, ensure quality

```bash
# Step 1: Fetch documentation pages
./llm-web-parser --urls "https://docs.example.com/api,https://api-docs.another.com,https://third.com/reference"

# Step 2: Get session
SESSION_DIR="llm-web-parser-results/sessions/$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)"

# Step 3: Quality gate - only high-confidence docs
yq '.[] | select(.domain_category == "docs/api" and .confidence >= 7 and .extraction_mode != "degraded")' "$SESSION_DIR/summary-details.yaml"

# Step 4: Extract code blocks only
./llm-web-parser extract --from "llm-web-parser-results/parsed/*.json" --strategy="type:code"
```

---

### 4. Content Discovery by Language

**Goal:** Find non-English high-quality content

```bash
# Get session
SESSION_DIR="llm-web-parser-results/sessions/$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)"

# Spanish content with high confidence
yq '.[] | select(.language == "es" and .language_confidence > 0.8 and .confidence >= 6)' "$SESSION_DIR/summary-details.yaml"

# Multi-language breakdown
yq '[.[] | .language] | group_by(.) | map({lang: .[0], count: length})' "$SESSION_DIR/summary-details.yaml"
```

---

## Integration Examples

### Python with subprocess

```python
import subprocess
import yaml

# Run parser
subprocess.run(["./llm-web-parser", "--urls", "https://example.com,https://example.org"])

# Get latest session
with open("llm-web-parser-results/index.yaml") as f:
    index = yaml.safe_load(f)
    session_id = index['sessions'][0]['session_id']

# Load summary details
with open(f"llm-web-parser-results/sessions/{session_id}/summary-details.yaml") as f:
    results = yaml.safe_load(f)

# Filter high-confidence government sites
gov_sites = [r for r in results if r.get('domain_type') == 'gov' and r.get('confidence', 0) >= 7]

print(f"Found {len(gov_sites)} high-confidence government sites")
for site in gov_sites:
    print(f"  {site['url']}: {site['title']} (confidence: {site['confidence']})")
```

### Shell Script (Quality-Gated Pipeline)

```bash
#!/bin/bash
set -e

# Fetch URLs from config
./llm-web-parser --config config.yaml

# Get session
SESSION_DIR="llm-web-parser-results/sessions/$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)"

# Quality gates
MIN_CONFIDENCE=7
MAX_TOKENS=1000

# Filter and extract
yq ".[] | select(.confidence >= $MIN_CONFIDENCE and .estimated_tokens < $MAX_TOKENS)" \
  "$SESSION_DIR/summary-details.yaml" > filtered.yaml

# Count results
RESULT_COUNT=$(yq '. | length' filtered.yaml)

if [ "$RESULT_COUNT" -eq 0 ]; then
  echo "No pages passed quality gates (confidence >= $MIN_CONFIDENCE, tokens < $MAX_TOKENS)"
  exit 1
fi

echo "✅ $RESULT_COUNT pages passed quality gates"

# Extract URLs for next stage
yq '.[].url' filtered.yaml
```

---

## Output Modes

### tier2 (default)

**Best for:** Scanning 100-1000 URLs efficiently

```bash
./llm-web-parser --urls "..."
```

**Output:**
- Creates session directory
- Writes `summary-index.yaml` (minimal, ~150 bytes/URL)
- Writes `summary-details.yaml` (full metadata, ~400 bytes/URL)
- Updates `index.yaml`
- Prints concise stats to stdout

**Token efficiency:** 100 URLs = ~15KB index + ~40KB details (vs ~470KB full parse)

---

### summary

**Best for:** Single report to stdout (JSON or YAML)

```bash
./llm-web-parser --urls "..." --output-mode summary
```

**Output:** Prints full summary to stdout (no session directory)

---

### full

**Best for:** Small batches (<5 URLs) needing immediate full content

```bash
./llm-web-parser --urls "https://example.com" --output-mode full
```

**Output:** Prints complete parsed content to stdout (very verbose)

---

### minimal

**Best for:** Metadata-only extraction

```bash
./llm-web-parser --urls "..." --output-mode minimal
```

**Output:** Basic metadata without content parsing

---

## Format Options

### YAML (default)

```bash
# Default - more token-efficient (10-15% smaller)
./llm-web-parser --urls "..."

# Query with yq
yq '.[] | .url' sessions/*/summary-index.yaml
```

### JSON

```bash
# Use JSON for traditional tooling
./llm-web-parser --urls "..." --format json

# Query with jq
jq -r '.[].url' sessions/*/summary-index.json
```

---

## Best Practices

### 1. Start with Minimal Mode (Default)

```bash
# Fast metadata scan (2-3x faster than full-parse)
./llm-web-parser --urls "url1,url2,...,url100"

# Then selectively analyze interesting URLs
./llm-web-parser analyze --urls "url5,url12,url47" --features full-parse
```

### 2. Use Confidence Scores for Filtering

```bash
# Only high-quality extractions
yq '.[] | select(.confidence >= 7)' summary-details.yaml
```

### 3. Leverage Domain Classification

```bash
# Government health sources only
yq '.[] | select(.domain_category == "gov/health")' summary-details.yaml

# Academic papers with citations
yq '.[] | select(.domain_type == "academic" and .has_citations)' summary-details.yaml
```

### 4. Check FIELDS.yaml First

Before querying, always reference `llm-web-parser-results/FIELDS.yaml` for:
- Available fields
- Valid values
- Query examples

### 5. Use Session Cache

Same URL list = instant cache hit. No re-fetching!

```bash
# First run: fetches
./llm-web-parser --urls "url1,url2,url3"

# Second run: instant (same URLs = same session hash)
./llm-web-parser --urls "url1,url2,url3"
# Session cache hit! ...
```

### 6. Multi-Stage Workflows

```bash
# Stage 1: Fetch minimal (100 URLs, 2-3s)
./llm-web-parser --urls "..." > /dev/null

# Stage 2: Scan + filter
SESSION_DIR="llm-web-parser-results/sessions/$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)"
URLS=$(yq '.[] | select(.confidence >= 7) | .url' "$SESSION_DIR/summary-details.yaml" | tr '\n' ',')

# Stage 3: Deep analysis (5 URLs, 0.5s)
./llm-web-parser analyze --urls "$URLS" --features full-parse
```

**Total:** 3.5s vs 15s full-parse upfront

---

## Error Handling

```bash
# Check for failures in session
SESSION_DIR="llm-web-parser-results/sessions/$(yq '.sessions[0].session_id' llm-web-parser-results/index.yaml)"

# List failed URLs
yq '.[] | select(.status == "failed") | {url, error}' "$SESSION_DIR/summary-details.yaml"

# Retry failed URLs with --force-fetch
FAILED_URLS=$(yq '.[] | select(.status == "failed") | .url' "$SESSION_DIR/summary-details.yaml" | tr '\n' ',')
./llm-web-parser --urls "$FAILED_URLS" --force-fetch
```

---

## Token Optimization Examples

### Scenario: 100 Competitor Landing Pages

**Naive approach:** Parse all 100 pages fully
- Output: ~470KB (188,000 tokens)
- Cost: ~$0.56 @ $3/M tokens

**Smart approach:** tier2 mode + filtering
1. Fetch minimal (100 URLs): ~15KB index + ~40KB details = 55KB (22,000 tokens)
2. Filter by `confidence >= 7` and `domain_type == "commercial"`: 20 URLs
3. Analyze selected: ~94KB (37,600 tokens)
4. **Total:** 59,600 tokens vs 188,000
5. **Savings:** 68% ($0.18 vs $0.56)

### Scenario: Academic Paper Discovery

**Goal:** Find 5 relevant papers from 100 ArXiv URLs

**Smart approach:**
1. Minimal mode: 22,000 tokens (metadata only)
2. Filter: `has_citations and academic_score >= 7`: 12 papers
3. Deep analysis: 12 papers × 3,140 tokens = 37,680 tokens
4. **Total:** 59,680 tokens
5. **vs Full parse:** 188,000 tokens
6. **Savings:** 68%

---

## Advanced: Field-Level Filtering

```bash
# Get only specific fields (ultra-minimal)
yq '.[] | {url, confidence, tokens: .estimated_tokens}' summary-details.yaml

# Complex filtering logic
yq '.[] | select(
  (.domain_type == "academic" and .academic_score >= 7) or
  (.domain_category == "gov/health" and .confidence >= 8)
) | {url, type: .domain_type, score: (.academic_score // .confidence)}' summary-details.yaml
```

---

## Troubleshooting

### Session not found

```bash
# List all sessions to find the right one
yq '.sessions[]' llm-web-parser-results/index.yaml

# Use specific session ID
SESSION_ID="2026-01-10T14-30-abc123"
SESSION_DIR="llm-web-parser-results/sessions/$SESSION_ID"
```

### Cache issues

```bash
# Force refetch (ignore cache)
./llm-web-parser --urls "..." --force-fetch

# Adjust cache TTL
./llm-web-parser --urls "..." --max-age "24h"

# Clear cache manually
rm -rf llm-web-parser-results/raw/*
rm -rf llm-web-parser-results/parsed/*
```

### Empty results

```bash
# Check extraction quality
yq '.[] | {url, quality: .extraction_quality, mode: .extraction_mode}' summary-details.yaml

# If quality is "degraded", try full-parse mode
./llm-web-parser analyze --urls "problematic_url" --features full-parse
```

---

## Summary: Why This Approach Wins

✅ **Session-based:** Organized, discoverable, no overwrites
✅ **Token-efficient:** YAML default, minimal index, selective deep-dive
✅ **Enriched metadata:** Domain classification, academic signals (FREE)
✅ **Smart filtering:** Confidence scores, content type, visual metadata
✅ **Cache-aware:** Same URLs = instant retrieval
✅ **Multi-stage:** Scan → filter → analyze (3-5x faster)
✅ **Self-documenting:** FIELDS.yaml always up-to-date

**Result:** 68% token savings, 3-5x faster, zero wasted computation.
