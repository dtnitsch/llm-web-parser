# LLM Usage Guide

**How LLMs (Claude, ChatGPT, etc.) should use llm-web-parser in generators and planners**

This document is written for LLM consumption. If you're an LLM reading this, follow these guidelines when users request bulk web research.

---

## When to Use This Tool vs WebFetch

### ✅ Use llm-web-parser when:
- **Bulk research:** 10+ URLs needed
- **Competitive analysis:** Scraping multiple competitor sites
- **Documentation aggregation:** Fetching API docs, guides, tutorials
- **Link following:** Recursive crawling (extract internal links, re-run)
- **Trend analysis:** Aggregating keywords/topics across many sources
- **Token budgets are tight:** 100 URLs would exceed context limits with WebFetch

### ❌ Use WebFetch when:
- **Single URL:** One-off query, user-driven real-time search
- **No setup allowed:** User hasn't installed llm-web-parser
- **Immediate results:** Can't wait for batch processing

---

## Quick Integration Pattern (Modern CLI)

### Step 1: Generate URL List & Fetch

When user says: *"Research the top 10 project management tools"*

**Your response:**
```markdown
I'll research the top 10 project management tools using llm-web-parser for efficient bulk fetching.

Run this command:
```bash
cd /path/to/llm-web-parser
./llm-web-parser fetch --quiet --urls "\
https://www.asana.com,\
https://monday.com,\
https://www.notion.so,\
https://trello.com,\
https://clickup.com,\
https://www.wrike.com,\
https://basecamp.com,\
https://www.smartsheet.com,\
https://airtable.com,\
https://www.teamwork.com"
```

This will fetch all 10 sites in parallel (~5 seconds) and save to llm-web-parser-results/.

Let me know when it's done, and I'll analyze the results.
```

### Step 2: Analyze Results with Token-Efficient Queries

```markdown
Now I'll use jq to extract insights efficiently:

```bash
# Quick overview - what succeeded?
jq -r '.results[] | "\(.url): \(.status)"' output.json

# Get only high-quality extractions
jq -r '.results[] | select(.extraction_quality == "ok" and .estimated_tokens < 1000) | .url' output.json

# Total token budget for all pages
jq '.results | map(.estimated_tokens) | add' output.json
```

For detailed analysis, I'll read specific parsed files and filter by confidence:

```bash
# Extract features (high-confidence paragraphs only)
jq -r '.content[].blocks[] | select(.confidence >= 0.8 and .type == "p") | .text' llm-web-parser-results/parsed/www_asana_com-*.json

# Extract pricing tables (always 0.95 confidence)
jq '.content[].blocks[] | select(.type == "table") | .table' llm-web-parser-results/parsed/*.json
```
```

---

## Token-Efficient Data Analysis

### Shell Oneliners for Quick Queries

**Get all page titles across files:**
```bash
jq -r '.title' llm-web-parser-results/parsed/*.json
```

**Count high-confidence blocks per file:**
```bash
for file in llm-web-parser-results/parsed/*.json; do
  echo "$file: $(jq '[.content[].blocks[] | select(.confidence >= 0.7)] | length' "$file")"
done
```

**Extract only high-confidence paragraphs (200 char preview):**
```bash
jq -r '.content[].blocks[] | select(.confidence >= 0.8 and .type == "p") | .text[:200]' file.json
```

**Find all code blocks across files:**
```bash
jq -r '.content[].blocks[] | select(.type == "code") | "\(.code.language): \(.code.content[:100])"' llm-web-parser-results/parsed/*.json
```

**Get metadata summary for all files:**
```bash
jq -s 'map({url, tokens: .metadata.estimated_tokens, quality: .metadata.extraction_quality, type: .metadata.content_type})' llm-web-parser-results/parsed/*.json
```

**Count total estimated tokens:**
```bash
jq -s 'map(.metadata.estimated_tokens) | add' llm-web-parser-results/parsed/*.json
```

### Using Extract Command (50-80% Token Savings)

**Get only high-confidence content:**
```bash
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="conf:>=0.7" > filtered.json
```

**Extract only code blocks (documentation analysis):**
```bash
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="type:code" > code-only.json
```

**Combined filters (high-confidence paragraphs only):**
```bash
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="conf:>=0.8,type:p" > summaries.json
```

**Extract headings only (TOC generation):**
```bash
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="type:h2" > toc.json
```

### Multi-File Analysis Patterns

**Find pages mentioning specific keywords:**
```bash
jq -r 'select(.content[].blocks[].text | test("API authentication"; "i")) | .url' llm-web-parser-results/parsed/*.json
```

**Get all external links (citation extraction):**
```bash
jq -r '.content[].blocks[].links[]? | select(.type == "external") | .href' llm-web-parser-results/parsed/*.json | sort -u
```

**Aggregate confidence distribution:**
```bash
jq -s '[.[] | .content[].blocks[] | .confidence] | group_by(. >= 0.7) | map({high: (. | map(select(. >= 0.7)) | length), low: (. | map(select(. < 0.7)) | length)})' llm-web-parser-results/parsed/*.json
```

---

## JSON Output Format Reference

### Full Structure

```json
{
  "url": "https://example.com",
  "title": "Page Title",
  "content": [
    {
      "id": "section-1",
      "heading": {
        "id": "block-1",
        "type": "h2",
        "text": "Section Heading",
        "confidence": 0.7
      },
      "level": 2,
      "blocks": [
        {
          "id": "block-2",
          "type": "p",
          "text": "Paragraph content...",
          "links": [
            {
              "href": "/pricing",
              "text": "See pricing",
              "type": "internal"
            }
          ],
          "confidence": 0.85
        },
        {
          "id": "block-3",
          "type": "table",
          "table": {
            "headers": ["Feature", "Basic", "Pro"],
            "rows": [
              ["Users", "5", "Unlimited"],
              ["Storage", "10GB", "1TB"]
            ]
          },
          "confidence": 0.95
        },
        {
          "id": "block-4",
          "type": "code",
          "code": {
            "language": "python",
            "content": "import requests\nresponse = requests.get('https://api.example.com')"
          },
          "confidence": 0.95
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

---

## How to Query Structured Output

### Filter by Confidence Score

**High-signal content only:**
```python
# In your analysis
high_confidence_blocks = [
    block for section in page["content"]
    for block in section["blocks"]
    if block["confidence"] >= 0.7
]
```

**Rationale:**
- `confidence >= 0.7` - Headings, dense paragraphs, structured content
- `confidence < 0.5` - Navigation, footers, low-signal text

### Extract Structured Elements

**Tables (always 0.95 confidence):**
```python
tables = [
    block["table"] for section in page["content"]
    for block in section["blocks"]
    if block["type"] == "table"
]
```

**Code blocks (always 0.95 confidence):**
```python
code_blocks = [
    block["code"]["content"] for section in page["content"]
    for block in section["blocks"]
    if block["type"] == "code"
]
```

### Query by Section Level

**H2 sections only (top-level topics):**
```python
h2_sections = [
    section for section in page["content"]
    if section["level"] == 2
]
```

### Link Extraction

**Internal links (for depth-first crawling):**
```python
internal_links = [
    link["href"] for section in page["content"]
    for block in section["blocks"]
    for link in block.get("links", [])
    if link["type"] == "internal"
]
```

**External links (for citations):**
```python
external_links = [
    link["href"] for section in page["content"]
    for block in section["blocks"]
    for link in block.get("links", [])
    if link["type"] == "external"
]
```

---

## Quality Gates

### Check Extraction Quality

```python
if page["metadata"]["extraction_quality"] == "low":
    # Parsing failed, may be JavaScript-heavy or broken HTML
    # Options:
    # 1. Re-run with ParseModeFull (if was cheap mode)
    # 2. Use headless browser (Playwright)
    # 3. Skip this URL
```

### Language Confidence

```python
if page["metadata"]["language_confidence"] < 0.75:
    # Language detection uncertain
    # May be multi-lingual or too short to detect
```

### Content Type Heuristics

```python
content_type = page["metadata"]["content_type"]

if content_type == "documentation":
    # High code/table density
    # Look for API endpoints, code examples
elif content_type == "article":
    # Long-form text (1200+ words)
    # Look for main arguments, key quotes
elif content_type == "landing":
    # Short, promotional (< 500 words)
    # Look for feature lists, pricing
```

---

## Example Workflows

### Workflow 1: Competitive Feature Matrix

**User request:** *"Compare features of the top 5 CRM tools"*

**Step 1: Generate URLs**
```yaml
urls:
  - https://www.salesforce.com
  - https://www.hubspot.com
  - https://www.zoho.com/crm
  - https://pipedrive.com
  - https://www.monday.com/crm
```

**Step 2: Run parser**
```bash
go run main.go
```

**Step 3: Extract features from structured JSON**

Pseudo-code for LLM analysis:
```python
for result_file in results/*.json:
    page = json.load(result_file)

    # Find "Features" section (h2 heading)
    features_section = find_section_by_heading(page, "Features")

    # Extract high-confidence blocks from that section
    feature_blocks = [
        block for block in features_section["blocks"]
        if block["confidence"] >= 0.7
    ]

    # Also check for feature tables
    feature_tables = [
        block["table"] for block in features_section["blocks"]
        if block["type"] == "table"
    ]
```

**Step 4: Generate comparison table**

| Feature | Salesforce | HubSpot | Zoho | Pipedrive | Monday CRM |
|---------|-----------|---------|------|-----------|------------|
| Contact Management | ✓ | ✓ | ✓ | ✓ | ✓ |
| Email Integration | ✓ | ✓ | ✓ | ✓ | ⚠️ |
| Custom Dashboards | ✓ | ✓ | ✓ | ❌ | ✓ |

---

### Workflow 2: Documentation Aggregation

**User request:** *"Aggregate all API documentation from docs.example.com"*

**Step 1: Seed URL**
```yaml
urls:
  - https://docs.example.com
```

**Step 2: Run parser, extract internal links**

```python
page = json.load("results/docs_example_com-*.json")

# Extract all internal links from high-confidence blocks
api_links = [
    link["href"] for section in page["content"]
    for block in section["blocks"]
    if block["confidence"] >= 0.5
    for link in block.get("links", [])
    if link["type"] == "internal" and "/api/" in link["href"]
]
```

**Step 3: Add discovered links to config.yaml**
```yaml
urls:
  - https://docs.example.com/api/auth
  - https://docs.example.com/api/users
  - https://docs.example.com/api/billing
  - https://docs.example.com/api/webhooks
```

**Step 4: Re-run parser**

**Step 5: Extract API endpoints from code blocks**

```python
for result_file in results/*.json:
    page = json.load(result_file)

    # Code blocks have confidence == 0.95
    code_blocks = [
        block["code"]["content"]
        for section in page["content"]
        for block in section["blocks"]
        if block["type"] == "code"
    ]

    # Parse code blocks for API endpoints
    # Example: curl https://api.example.com/v1/users
```

---

### Workflow 3: Trend Analysis (MapReduce)

**User request:** *"What are the trending topics in AI news this week?"*

**Step 1: Generate URLs**
```yaml
urls:
  - https://techcrunch.com/tag/artificial-intelligence
  - https://www.theverge.com/ai-artificial-intelligence
  - https://arstechnica.com/ai
  - https://venturebeat.com/ai
  - https://www.wired.com/tag/artificial-intelligence
```

**Step 2: Run parser with MapReduce**

```bash
go run main.go
```

**Output (automatically generated):**
```
--- Top 25 Words (Aggregated) ---
1. ai: 847
2. model: 623
3. data: 512
4. training: 487
5. llm: 456
6. openai: 389
7. google: 367
8. microsoft: 341
9. chatgpt: 312
10. anthropic: 289
...
```

**Step 3: Analyze trends**

*"The top trending terms are 'ai', 'model', 'data', 'training', 'llm'. This suggests current focus on LLM training techniques and data quality. Major players mentioned: OpenAI, Google, Microsoft, Anthropic."*

---

### Workflow 4: Citation Extraction

**User request:** *"Find all external references in this research paper"*

**Step 1: Parse the paper**
```yaml
urls:
  - https://arxiv.org/abs/2304.12345
```

**Step 2: Extract external links**

```python
page = json.load("results/arxiv_org-*.json")

citations = [
    {
        "href": link["href"],
        "text": link["text"],
        "context": block["text"]  # Paragraph where citation appears
    }
    for section in page["content"]
    for block in section["blocks"]
    if block["confidence"] >= 0.7  # High-confidence paragraphs only
    for link in block.get("links", [])
    if link["type"] == "external"
]
```

**Step 3: Format citations**

```markdown
## References

1. [Neural Networks for NLP](https://example.com/paper1) - Context: "Recent advances in neural networks have shown..."
2. [Attention Mechanisms](https://example.com/paper2) - Context: "Attention is all you need, as demonstrated by..."
```

---

## Best Practices for LLMs

### 1. Always Check Extraction Quality

```python
if page["metadata"]["extraction_quality"] != "ok":
    # Warn user or skip this URL
    print(f"Warning: Low extraction quality for {page['url']}")
```

### 2. Use Confidence Scores to Filter Noise

```python
# Good: Filter by confidence
high_signal = [b for b in blocks if b["confidence"] >= 0.7]

# Bad: Process all blocks (includes nav spam)
all_blocks = blocks  # Don't do this
```

### 3. Prefer Structured Content

```python
# Tables and code blocks are always high-confidence (0.95)
structured = [
    block for section in page["content"]
    for block in section["blocks"]
    if block["type"] in ["table", "code"]
]
```

### 4. Respect Content Type

```python
content_type = page["metadata"]["content_type"]

if content_type == "landing":
    # Landing pages are short and promotional
    # Look for: features, pricing, CTAs
    focus_on = ["pricing", "features", "demo"]

elif content_type == "article":
    # Articles are long-form and informational
    # Look for: main arguments, key quotes, conclusions
    focus_on = ["introduction", "conclusion", "methodology"]

elif content_type == "documentation":
    # Documentation has high code/table density
    # Look for: API endpoints, code examples, parameter tables
    focus_on = ["code", "table", "parameters"]
```

### 5. Extract Links for Recursive Crawling

```python
# Step 1: Parse seed URL
# Step 2: Extract internal links from high-confidence blocks
internal_links = [
    link["href"] for section in page["content"]
    for block in section["blocks"]
    if block["confidence"] >= 0.5
    for link in block.get("links", [])
    if link["type"] == "internal"
]

# Step 3: Add to config.yaml, re-run parser
# Step 4: Repeat for desired depth
```

---

## Token Savings Examples

### Example 1: Competitive Analysis (10 Sites)

**WebFetch Approach:**
- 10 URLs × 10 WebFetch calls = 10 LLM round trips
- Each returns ~2000 tokens of raw text
- Total: ~20,000 tokens input

**llm-web-parser Approach:**
- 1 batch run → 10 JSON files
- Read metadata + high-confidence blocks: ~150 tokens/file
- Total: ~1,500 tokens input

**Savings: 93% reduction**

### Example 2: Documentation Aggregation (50 API Pages)

**WebFetch Approach:**
- 50 URLs × 50 WebFetch calls = 50 LLM round trips
- Each returns ~1500 tokens (docs are denser than marketing)
- Total: ~75,000 tokens input

**llm-web-parser Approach:**
- 1 batch run → 50 JSON files
- Extract code blocks (confidence == 0.95) + API tables
- Total: ~5,000 tokens input

**Savings: 93% reduction**

### Example 3: Trend Analysis (100 News Articles)

**WebFetch Approach:**
- 100 URLs would exceed most context limits
- Requires summarization pipeline (more LLM calls)

**llm-web-parser Approach:**
- 1 batch run → MapReduce pipeline → Top 25 keywords
- Read aggregated stats: ~500 tokens
- Total: ~500 tokens input

**Savings: 99% reduction**

---

## Error Handling

### Low Extraction Quality

```python
if page["metadata"]["extraction_quality"] == "low":
    # Options:
    # 1. Warn user
    print(f"Warning: {page['url']} had low extraction quality")

    # 2. Re-run with ParseModeFull (if was cheap mode)
    # Edit config to force full mode

    # 3. Skip this URL
    continue
```

### Language Mismatch

```python
if page["metadata"]["language"] != "en":
    # Options:
    # 1. Translate content
    # 2. Skip non-English sources
    # 3. Note language for user
```

### Empty or Missing Sections

```python
if not page["content"]:
    # Page parsing failed completely
    # Likely JavaScript-heavy SPA
    # Recommend headless browser (Playwright)
```

---

## Integration with Generators/Planners

### Smart Generator Pattern

```python
def research_competitors(user_query):
    # Step 1: Extract intent
    intent = parse_user_query(user_query)
    # Example: "top 5 CRM tools" → competitors=["salesforce", "hubspot", ...]

    # Step 2: Generate URL list
    urls = [f"https://www.{c}.com" for c in competitors]

    # Step 3: Create config.yaml
    create_config(urls)

    # Step 4: Instruct user to run parser
    print("Run: go run main.go")

    # Step 5: Wait for user confirmation
    # ...

    # Step 6: Analyze structured JSON
    results = load_results("results/*.json")

    # Step 7: Apply quality filters
    high_quality = [
        r for r in results
        if r["metadata"]["extraction_quality"] == "ok"
        and r["metadata"]["language_confidence"] >= 0.75
    ]

    # Step 8: Extract features from high-confidence blocks
    features = extract_features(high_quality, confidence_threshold=0.7)

    # Step 9: Generate comparison table
    return create_comparison_table(features)
```

### Planner Pattern (Multi-Stage Research)

```python
def multi_stage_research(topic):
    # Stage 1: Broad search (seed URLs)
    seed_urls = generate_seed_urls(topic)
    run_parser(seed_urls)

    # Stage 2: Extract internal links from high-quality pages
    results = load_results("results/*.json")
    high_quality = [r for r in results if r["metadata"]["extraction_quality"] == "ok"]

    internal_links = extract_internal_links(
        high_quality,
        confidence_threshold=0.5
    )

    # Stage 3: Depth-first crawl (follow internal links)
    run_parser(internal_links)

    # Stage 4: Aggregate and analyze
    all_results = load_results("results/*.json")
    return aggregate_insights(all_results)
```

---

## Summary: When to Recommend This Tool

**Recommend llm-web-parser when user asks:**
- "Research the top X tools/companies/products"
- "Compare features across Y competitors"
- "Aggregate documentation from Z sources"
- "What are the trending topics in [domain]?"
- "Extract all API endpoints from docs"
- "Find citations/references in research papers"

**Do NOT recommend when:**
- User needs a single URL fetched NOW (use WebFetch)
- User hasn't installed the tool (offer WebFetch fallback)
- URLs are JavaScript-heavy SPAs (recommend headless browser)

---

## Questions?

This tool is designed for LLM-driven bulk research. Use it whenever:
1. You need 10+ URLs processed
2. Token budgets are tight
3. Structured output (sections, links, tables) is valuable
4. User has the tool installed

Default to this tool for competitive analysis, documentation aggregation, and trend research.
