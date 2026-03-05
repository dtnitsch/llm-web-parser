# LLM Research Workflow Guide

**Purpose:** Step-by-step guide for LLMs to research topics by fetching 30+ websites, analyzing results, and extracting specific data.

**Use case:** Extract pricing plans from Netlify, compare API features across docs, analyze academic papers, etc.

---

## TL;DR - Recommended Workflow

**For LLMs doing research with 30+ URLs:**

1. **Verify:** `which llm-web-parser`
2. **Fetch (fast):** `llm-web-parserfetch --urls "url1,url2,...,url30"` → Session ID
3. **Explore:** `llm-web-parser corpus suggest --session <id>` → See what's available
4. **Filter:** `llm-web-parser corpus query --session <id> --filter="has_code AND keyword:api"` → Find relevant URLs
5. **Deep parse:** `llm-web-parserfetch --session <id> --features full-parse` → Get full content
6. **Extract:** `llm-web-parser db show <url_id> --only=h2,code` → Get specific data

**Key insight:** Use corpus API to filter BEFORE deep parsing. This saves time and tokens.

---

## Step 0: Verify Tool Availability

```bash
which llm-web-parser
# Output: /Users/username/go/bin/llm-web-parser (or similar)

# Verify it works
llm-web-parser --coldstart
```

**If not found:** Install with `go install github.com/dtnitsch/llm-web-parser@latest`

---

## Step 1: Fetch URLs (Two-Stage Workflow)

### Stage 1: Fast Metadata Scan

Fetch 30-50 URLs quickly to understand what you have:

```bash
# Default mode: metadata + keywords (recommended for triage)
llm-web-parser fetch --urls "url1,url2,...,url30"

# Output shows:
# Session 1: 28/30 URLs successful
# Results: llm-web-parser-results/sessions/2026-02-11-1
```

**What this gives you:**
- Session ID (auto-incrementing: 1, 2, 3...)
- Success/failure counts
- Location where results are stored
- Keywords for each URL (helps you decide what to deep-dive)

**Example:**
```bash
llm-web-parser fetch --urls "https://www.netlify.com/pricing/,https://vercel.com/pricing,https://render.com/pricing"
# Session 3: 3/3 URLs successful
```

### Stage 2: Deep Parse Selected URLs

Once you know which URLs are interesting (based on keywords, confidence, etc.):

```bash
# Refetch the same session with full-parse mode
llm-web-parser fetch --session 3 --features full-parse

# Or fetch specific URLs with full-parse
llm-web-parser fetch --urls "https://www.netlify.com/pricing/" --features full-parse
```

**Why two stages?**
- Stage 1 is fast (~2-5 seconds for 50 URLs)
- Stage 2 is thorough but slower (~1-2 seconds per URL)
- You avoid wasting time parsing irrelevant content

---

## Step 2: Understand Where Data Lives

### Database Location

```bash
# Show database path
llm-web-parser db path
# Output: /path/to/current/directory/llm-web-parser.db
```

**Important:** Database and results are created in your **current working directory** (where you run the command), not where the binary is installed.

**Example:**
```bash
# Running from /Users/daniel/project1
cd /Users/daniel/project1
llm-web-parser fetch --urls "..."
# Creates: /Users/daniel/project1/llm-web-parser.db
#          /Users/daniel/project1/lwp-sessions/
#          /Users/daniel/project1/lwp-results/

# Running from /Users/daniel/project2
cd /Users/daniel/project2
llm-web-parser fetch --urls "..."
# Creates: /Users/daniel/project2/llm-web-parser.db
#          /Users/daniel/project2/lwp-sessions/
#          /Users/daniel/project2/lwp-results/
```

Each directory gets its own isolated database and results.

### Directory Structure

```
llm-web-parser-results/
├── llm-web-parser.db              # SQLite database (sessions, URLs, metadata)
├── index.yaml                     # Registry of all sessions
├── FIELDS.yaml                    # Field reference with query examples
├── sessions/
│   └── 2026-02-11-3/              # Session directory (date + ID)
│       ├── summary-index.yaml     # Minimal data (~150 bytes/URL)
│       ├── summary-details.yaml   # Full metadata (~400 bytes/URL)
│       └── failed-urls.yaml       # Failed URLs (if any)
├── raw/                           # Shared HTML cache (by URL hash)
└── parsed/                        # Shared JSON cache (full-parse mode)
```

### Key Files

| File | Purpose | When to Use |
|------|---------|-------------|
| `summary-index.yaml` | Quick overview (title, confidence, keywords) | Fast triage |
| `summary-details.yaml` | Full metadata (word_count, domain_type, scores) | Filtering, analysis |
| `parsed/<hash>.json` | Full parsed content (blocks, types, confidence) | Data extraction |
| `raw/<hash>.html` | Original HTML | Debugging, custom parsing |

---

## Step 3: List and Explore Results

### List All Sessions

```bash
llm-web-parser db sessions
# Shows: ID, Created, URLs, Success/Failed, Session Dir
```

### Get Session Overview

```bash
# Latest session (auto-detected)
llm-web-parser db get --file=details

# Specific session
llm-web-parser db get --file=details 3
```

**Output:** YAML with metadata for all URLs in the session.

### Show URL IDs

```bash
# Latest session
llm-web-parser db urls

# Specific session
llm-web-parser db urls 3

# Output:
# Session: 3
#  1. [#5] https://www.netlify.com/pricing/
#     commercial | no_code | conf:5.0
#     Keywords: netlify, credits, team, plan, web
#  2. [#6] https://vercel.com/pricing
#     commercial | no_code | conf:6.5
#     Keywords: vercel, pro, hobby, enterprise, pricing
```

**Use URL IDs to save tokens:** `#5` instead of the full URL.

---

## Step 4: Corpus API - Query and Analyze Collections

The **Corpus API** provides powerful operations for analyzing collections of web content. It's designed specifically for LLM workflows.

### Available Commands

| Command | Status | Purpose |
|---------|--------|---------|
| **extract** | ✅ Working | Aggregate keywords across URLs |
| **query** | ✅ Working | Boolean filtering over metadata |
| **suggest** | ✅ Working | Get query suggestions for session |
| compare | ⏳ Planned | Cross-document analysis |
| detect | ⏳ Planned | Pattern recognition |
| normalize | ⏳ Planned | Canonicalize entities |
| trace | ⏳ Planned | Citation graphs |
| score | ⏳ Planned | Confidence metrics |
| delta | ⏳ Planned | Incremental updates |
| summarize | ⏳ Planned | Structured synthesis |

### 4.1: Extract Keywords

Extract and aggregate keywords across URLs to understand what topics are covered.

```bash
# Extract top 25 keywords (default)
llm-web-parser corpus extract --session 3

# Get more keywords
llm-web-parser corpus extract --session 3 --top 50

# Get ALL keywords (warning: can be large)
llm-web-parser corpus extract --session 3 --top 0

# Extract from specific URLs only
llm-web-parser corpus extract --url-ids=5,6,7

# Output formats
llm-web-parser corpus extract --session 3 --format json  # JSON (default)
llm-web-parser corpus extract --session 3 --format yaml  # YAML
llm-web-parser corpus extract --session 3 --format csv   # CSV (spreadsheet-friendly)
```

**Output (JSON):**
```yaml
verb: extract
data:
  urlcount: 3
  keywords:
    - word: netlify
      count: 45
    - word: pricing
      count: 38
    - word: plan
      count: 32
    - word: enterprise
      count: 28
confidence: 0.95
coverage: 1.0
unknowns: []
error: null
```

**Use case:** Quickly see what topics 30+ URLs cover without reading them all.

### 4.2: Query - Boolean Filtering

Filter URLs by metadata using boolean expressions. **This is incredibly powerful for narrowing down results.**

```bash
# Basic filters
llm-web-parser corpus query --session 3 --filter="has_code"
llm-web-parser corpus query --session 3 --filter="content_type=academic"
llm-web-parser corpus query --session 3 --filter="has_abstract"

# Comparison operators
llm-web-parser corpus query --session 3 --filter="citations>50"
llm-web-parser corpus query --session 3 --filter="section_count>=10"
llm-web-parser corpus query --session 3 --filter="detection_confidence>=7"

# Boolean logic (AND/OR)
llm-web-parser corpus query --session 3 --filter="has_code AND citations>20"
llm-web-parser corpus query --session 3 --filter="content_type=academic OR has_doi"
llm-web-parser corpus query --session 3 --filter="has_code AND section_count>=5 AND citations>10"

# Keyword search (searches within top_keywords JSON)
llm-web-parser corpus query --session 3 --filter="keyword:pricing"
llm-web-parser corpus query --session 3 --filter="keyword:api"

# Combine with other filters
llm-web-parser corpus query --session 3 --filter="keyword:pricing AND has_code"
```

**Available Fields:**

| Field | Type | Example |
|-------|------|---------|
| `content_type` | string | academic, docs, wiki, news, blog, repo |
| `content_subtype` | string | research, tutorial, reference |
| `detection_confidence` | float | 0-10 (higher = more confident) |
| `has_abstract` | bool | true/false |
| `has_infobox` | bool | true/false |
| `has_toc` | bool | true/false (table of contents) |
| `has_code_examples` | bool | true/false |
| `section_count` | int | Number of sections |
| `citation_count` | int | Number of citations |
| `code_block_count` | int | Number of code blocks |
| `keyword:term` | special | Search top keywords |

**Output:**
```yaml
verb: query
data:
  filter: has_code AND citations>20
  matchcount: 5
  totalcount: 30
  matches:
    - urlid: 12
      originalurl: https://arxiv.org/abs/1234
      domain: arxiv.org
      contenttype: academic
      detectionconfidence: 9.5
      hasabstract: true
      citationcount: 45
      codeblockcount: 8
    - urlid: 18
      originalurl: https://example.com/tutorial
      ...
confidence: 0.95
coverage: 1.0
```

**Use case:** Filter 30 URLs down to the 5 academic papers with code examples.

### 4.3: Suggest - Get Query Ideas

Get suggested queries based on what's actually in your session.

```bash
llm-web-parser corpus suggest --session 3
```

**Output:**
```
📊 Session 3 Analysis:
  30 URLs parsed
  12 academic (40%)
  8 docs (27%)
  10 unknown (33%)

💡 Suggested queries:
  llm-web-parser corpus extract --session=3
  llm-web-parser corpus query --session=3 --filter="keyword:transformer"
  llm-web-parser corpus query --session=3 --filter="content_type=academic"
  llm-web-parser corpus query --session=3 --filter="has_code"
  llm-web-parser corpus query --session=3 --filter="citations>20"

Advanced: llm-web-parser corpus --help
```

**Use case:** Not sure what to query? Run this first to see what metadata is available.

### 4.4: Corpus Workflow Example

**Goal:** Research deep learning frameworks, find tutorials with code.

```bash
# Step 1: Fetch 30 framework documentation URLs
llm-web-parserfetch --urls "pytorch.org/docs,tensorflow.org/guide,..."
# Session 10: 28/30 successful

# Step 2: Get suggestions
llm-web-parser corpus suggest --session 10
# Suggests: filter="has_code", filter="keyword:tutorial"

# Step 3: Query for tutorials with code
llm-web-parser corpus query --session 10 --filter="has_code AND keyword:tutorial"
# Returns 8 matching URLs

# Step 4: Extract keywords from those 8 URLs
llm-web-parser corpus extract --url-ids=42,43,44,45,46,47,48,49
# Keywords: training, model, dataset, loss, optimizer

# Step 5: Deep parse those 8 URLs
llm-web-parser db show 42,43,44,45,46,47,48,49

# Step 6: Extract code blocks only
llm-web-parser db show 42 --only=code,pre
```

**Result:** Found 8 high-quality tutorials with code examples, extracted keywords, and got the code blocks.

---

## Step 5: Extract Specific Data from Parsed Content

### Use Case: Extract Netlify Pricing Tiers

**Goal:** Extract the 4 pricing packages (Free, Personal, Pro, Enterprise) from https://www.netlify.com/pricing/

### Step 5.1: Get Parsed Content

```bash
# Get full parsed content by URL ID
llm-web-parser db show 5

# Or by URL
llm-web-parser db show https://www.netlify.com/pricing/
```

**Output:** JSON with structured content blocks:
```json
{
  "url": "https://www.netlify.com/pricing/",
  "blocks": [
    {
      "type": "h2",
      "content": "Free",
      "confidence": 0.95,
      "position": 12
    },
    {
      "type": "p",
      "content": "Perfect for personal projects...",
      "confidence": 0.85,
      "position": 13
    },
    {
      "type": "ul",
      "content": "- 100GB bandwidth\n- 300 build minutes\n- ...",
      "confidence": 0.90,
      "position": 14
    }
  ]
}
```

### Step 5.2: Filter by Block Type

```bash
# Get only headings (outline)
llm-web-parser db show 5 --outline

# Output:
# h1: Pricing and Plans
# h2: Free
# h2: Personal
# h2: Pro
# h2: Enterprise

# Get specific block types (headings + lists)
llm-web-parser db show 5 --only=h2,ul,p
```

### Step 5.3: Search for Patterns

```bash
# Search for "Free" with context
llm-web-parser db show 5 --grep="Free|Personal|Pro|Enterprise" --context=3

# This shows matching blocks plus 3 blocks before/after
```

### Step 5.4: Process with yq/jq

```bash
# Get all h2 headings (pricing tier names)
llm-web-parser db show 5 | jq '.blocks[] | select(.type == "h2") | .content'

# Output:
# "Free"
# "Personal"
# "Pro"
# "Enterprise"

# Get pricing tiers with their feature lists
llm-web-parser db show 5 | jq '
  .blocks[] |
  select(.type == "h2" or .type == "ul") |
  {type, content}
'
```

### Step 5.5: Extract Structured Data (Full Example)

**Goal:** Extract pricing tier name, description, and features.

```bash
# Approach 1: Use the parsed JSON directly
llm-web-parser db show 5 > pricing.json

# Then use jq to extract tiers
cat pricing.json | jq '
  .blocks |
  to_entries |
  map(
    select(.value.type == "h2") |
    {
      tier: .value.content,
      index: .key,
      features: []
    }
  ) |
  map(
    . + {
      features: (
        [.index + 1, .index + 2, .index + 3] |
        map(. as $i | $blocks[$i] | select(.type == "ul" or .type == "p") | .content)
      )
    }
  )
' --argjson blocks "$(cat pricing.json | jq '.blocks')"
```

**Output:**
```json
[
  {
    "tier": "Free",
    "features": [
      "Perfect for personal projects",
      "- 100GB bandwidth\n- 300 build minutes\n- Unlimited sites"
    ]
  },
  {
    "tier": "Personal",
    "features": [...]
  }
]
```

**Approach 2: Use yq for YAML processing**

```bash
# Get session details
llm-web-parser db get --file=details 3 | yq '.[] | select(.url_id == 5)'

# Filter by confidence score
llm-web-parser db get --file=details | yq '.[] | select(.confidence >= 7)'

# Filter by domain type
llm-web-parser db get --file=details | yq '.[] | select(.domain_type == "commercial")'
```

---

## Step 6: Common Research Patterns

### Pattern 1: Corpus-First Workflow (Recommended)

Use corpus API to filter before deep parsing - saves time and tokens.

```bash
# 1. Fetch 30 URLs (fast scan with keywords)
llm-web-parserfetch --urls "url1,url2,...,url30"
# Session 1: 28/30 successful

# 2. Get query suggestions
llm-web-parser corpus suggest --session 1
# Suggests: filter="has_code", filter="keyword:tutorial"

# 3. Query for relevant URLs only
llm-web-parser corpus query --session 1 --filter="has_code AND keyword:api"
# Returns 8 matching URL IDs

# 4. Deep parse ONLY those 8 URLs
llm-web-parser db show 12,15,18,22,25,28,31,34

# 5. Extract specific data
llm-web-parser db show 12 --only=code,pre
```

**Why this works:** You triage 30 URLs → filter to 8 relevant → deep parse only those 8.

### Pattern 2: Batch Fetch and Filter (YAML-based)

Alternative approach using yq for filtering.

```bash
# Fetch 30 URLs
llm-web-parserfetch --urls "url1,url2,...,url30"

# Filter high-confidence URLs with YAML
llm-web-parser db get --file=details | yq '.[] | select(.confidence >= 7)' > high_conf.yaml

# Extract just the URL IDs for deep parsing
cat high_conf.yaml | yq '.[] | .url_id' > url_ids.txt

# Deep parse those URLs
llm-web-parser db show $(cat url_ids.txt | tr '\n' ',' | sed 's/,$//')
```

### Pattern 3: Multi-Session Research

```bash
# Session 1: Fetch initial URLs
llm-web-parserfetch --urls "..."
# Session 1: 25/30 successful

# Session 2: Retry failures
llm-web-parserfetch --session 1 --failed-only
# Session 2: 3/5 successful

# Session 3: Deep parse successful URLs from session 1
llm-web-parserfetch --session 1 --features full-parse
# Reuses same session ID if URLs are identical
```

### Pattern 4: Cross-Document Keyword Analysis

```bash
# 1. Fetch academic papers
llm-web-parserfetch --urls "arxiv.org/abs/123,arxiv.org/abs/456,..."
# Session 5: 20/20 successful

# 2. Extract keywords across all papers
llm-web-parser corpus extract --session 5 --top 50 --format json > keywords.json

# 3. Query for papers mentioning specific technique
llm-web-parser corpus query --session 5 --filter="keyword:transformer"
# Returns 12 papers

# 4. Get citations for those papers
llm-web-parser corpus query --session 5 --filter="keyword:transformer AND citations>20"
# Returns 5 highly-cited papers

# 5. Deep dive those 5 papers
llm-web-parser db show 42,45,48,51,54 --only=h2,p
```

### Pattern 5: Comparative Analysis

```bash
# Research question: "How do 3 frameworks handle authentication?"

# 1. Fetch docs
llm-web-parserfetch --urls "django.com/auth,flask.com/auth,fastapi.com/auth"
# Session 8: 3/3 successful

# 2. Check keywords
llm-web-parser corpus extract --session 8 --top 20
# Keywords: auth, token, session, oauth, jwt

# 3. Query for URLs mentioning JWT
llm-web-parser corpus query --session 8 --filter="keyword:jwt"
# Returns 2/3 URLs

# 4. Deep parse all 3
llm-web-parserfetch --session 8 --features full-parse

# 5. Extract code examples only
llm-web-parser db show 20,21,22 --only=code,pre | jq '.[] | {url, code: [.blocks[] | select(.type == "code") | .content]}'
# Structured comparison of authentication code across frameworks
```

### Pattern 6: Query by Metadata (Alternative to Corpus)

If you prefer YAML/yq instead of corpus API:

```bash
# Get all academic papers with citations
llm-web-parser db get --file=details | yq '.[] | select(.has_doi and .academic_score >= 7)'

# Get all docs with code examples
llm-web-parser db get --file=details | yq '.[] | select(.domain_category == "docs/api")'

# Get all recent articles (requires published_at)
llm-web-parser db get --file=details | yq '.[] | select(.published_at >= "2024-01-01")'
```

**Note:** Corpus API (`llm-web-parser corpus query`) is more powerful and token-efficient than yq filtering.

---

## Step 7: Token Efficiency Tips

### Use URL IDs Instead of Full URLs

```bash
# Bad (wastes tokens)
llm-web-parser db show https://www.netlify.com/pricing/

# Good (saves 90% tokens)
llm-web-parser db show 5
```

### Batch Retrieve

```bash
# Get multiple URLs at once
llm-web-parser db show 5,6,7,8
```

### Use Filters Early

```bash
# Fetch with inline filtering (only show high-confidence blocks)
llm-web-parser fetch --urls "..." --features full-parse --filter "conf:>=0.7"

# Filter by block type
llm-web-parser fetch --urls "..." --features full-parse --filter "type:code|pre"
```

### Use Summary Files for Triage

```bash
# summary-index.yaml: ~150 bytes/URL (title, keywords, confidence)
llm-web-parser db get --file=index

# summary-details.yaml: ~400 bytes/URL (full metadata)
llm-web-parser db get --file=details

# Only deep-dive after triage
llm-web-parser db show 5  # Full parsed content (~1-5KB per URL)
```

---

## Quick Reference: Essential Commands

### Fetching & Sessions

| Task | Command |
|------|---------|
| **Verify tool** | `which llm-web-parser` |
| **Fetch URLs** | `llm-web-parserfetch --urls "url1,url2,url3"` |
| **Deep parse** | `llm-web-parserfetch --session 1 --features full-parse` |
| **List sessions** | `llm-web-parser db sessions` |
| **Get session YAML** | `llm-web-parser db get --file=details` |
| **Show URL IDs** | `llm-web-parser db urls` |

### Corpus API (Query & Analyze)

| Task | Command |
|------|---------|
| **Extract keywords** | `llm-web-parser corpus extract --session 1` |
| **Query by metadata** | `llm-web-parser corpus query --session 1 --filter="has_code"` |
| **Boolean filters** | `llm-web-parser corpus query --session 1 --filter="has_code AND citations>20"` |
| **Keyword search** | `llm-web-parser corpus query --session 1 --filter="keyword:api"` |
| **Get suggestions** | `llm-web-parser corpus suggest --session 1` |

### Content Extraction

| Task | Command |
|------|---------|
| **Get parsed content** | `llm-web-parser db show <id>` |
| **Get outline** | `llm-web-parser db show <id> --outline` |
| **Filter blocks** | `llm-web-parser db show <id> --only=h2,ul` |
| **Search content** | `llm-web-parser db show <id> --grep="pattern"` |
| **Get raw HTML** | `llm-web-parser db raw <id>` |
| **Find URL ID** | `llm-web-parser db find-url https://example.com` |

**Alias:** `lwp` = `llm-web-parser` (use shorter version to save tokens)

---

## Complete Example: Netlify Pricing Research

**Goal:** Extract pricing package details (Free, Personal, Pro, Enterprise) from Netlify and compare with competitors.

```bash
# 0. Verify
which llm-web-parser

# 1. Fetch pricing pages from 3 providers (fast scan)
llm-web-parserfetch --urls "https://www.netlify.com/pricing/,https://vercel.com/pricing,https://render.com/pricing"
# Session 5: 3/3 URLs successful

# 2. Check what we got
llm-web-parser db urls 5
# Session: 5
#  1. [#12] https://www.netlify.com/pricing/
#     commercial | no_code | conf:5.0
#     Keywords: netlify, pricing, plan, team, enterprise
#  2. [#13] https://vercel.com/pricing
#     commercial | no_code | conf:6.5
#     Keywords: vercel, pro, hobby, enterprise, pricing
#  3. [#14] https://render.com/pricing
#     commercial | no_code | conf:5.5
#     Keywords: render, free, starter, team, pricing

# 3. Use corpus API to explore
llm-web-parser corpus suggest --session 5
# Suggests: filter="keyword:pricing", filter="keyword:enterprise"

llm-web-parser corpus extract --session 5 --top 20
# verb: extract
# keywords:
#   - word: pricing (count: 45)
#   - word: plan (count: 38)
#   - word: enterprise (count: 28)
#   - word: free (count: 25)

# 4. Query for URLs with pricing keywords
llm-web-parser corpus query --session 5 --filter="keyword:pricing"
# All 3 URLs match

# 5. Deep parse to get full content
llm-web-parserfetch --session 5 --features full-parse
# Session 5 refetched with full-parse

# 6. Get pricing tier structure for Netlify (outline view)
llm-web-parser db show 12 --outline
# h1: Pricing and Plans
# h2: Free
# h2: Personal
# h2: Pro
# h2: Enterprise

# 7. Extract tier names + features (headings + lists)
llm-web-parser db show 12 --only=h2,ul,p | jq '.blocks[] | select(.type == "h2" or .type == "ul")'
# {
#   "type": "h2",
#   "content": "Free",
#   "confidence": 0.95
# },
# {
#   "type": "ul",
#   "content": "- 100GB bandwidth\n- 300 build minutes\n- Unlimited sites",
#   "confidence": 0.90
# },
# ...

# 8. Compare tier names across all providers
llm-web-parser db show 12,13,14 --only=h2 | jq -s '
  map({
    url: .url,
    tiers: [.blocks[] | select(.type == "h2") | .content]
  })
'
# [
#   {
#     "url": "https://www.netlify.com/pricing/",
#     "tiers": ["Free", "Personal", "Pro", "Enterprise"]
#   },
#   {
#     "url": "https://vercel.com/pricing",
#     "tiers": ["Hobby", "Pro", "Enterprise"]
#   },
#   {
#     "url": "https://render.com/pricing",
#     "tiers": ["Free", "Starter", "Team", "Business", "Enterprise"]
#   }
# ]

# 9. Extract Netlify tier details with structured data
llm-web-parser db show 12 | jq '
  .blocks |
  reduce .[] as $block (
    {tiers: [], current: null};
    if $block.type == "h2" then
      .current = {name: $block.content, features: []} |
      .tiers += [.current]
    elif $block.type == "ul" or $block.type == "p" then
      if .current then
        .current.features += [$block.content]
      else . end
    else . end
  ) |
  .tiers
'
# [
#   {
#     "name": "Free",
#     "features": [
#       "Perfect for personal projects",
#       "- 100GB bandwidth\n- 300 build minutes\n- Unlimited sites"
#     ]
#   },
#   {
#     "name": "Personal",
#     "features": [...]
#   },
#   {
#     "name": "Pro",
#     "features": [...]
#   },
#   {
#     "name": "Enterprise",
#     "features": [...]
#   }
# ]
```

**Result:**
- Extracted all 4 pricing tiers from Netlify with structured features
- Compared tier names across 3 providers
- Used corpus API to explore keywords and filter results
- Total time: ~10 seconds (including fetch + parse + extraction)

---

## Troubleshooting

### "No sessions found"
```bash
# Run fetch first
llm-web-parserfetch --urls "https://example.com"
```

### "Parsed JSON not found"
```bash
# URL was fetched with wordcount mode, not full-parse
llm-web-parserfetch --session 1 --features full-parse
```

### "Malformed URLs"
```bash
# URLs are auto-cleaned, but spaces must be %20
# Check sanitized URLs
llm-web-parser db urls --sanitized
```

### "File not found: summary-details.yaml"
```bash
# Working directory mismatch - check database path
llm-web-parser db path
# Database and results must be in same directory
```

### Permission Issues

If you get permission errors when creating the database or results directories:

```bash
# Check current directory permissions
ls -la

# Try running in a directory where you have write access
cd ~/Documents
llm-web-parser fetch --urls "..."
```

**For LLMs:** If running from a read-only directory, `cd` to a writable location first (e.g., `/tmp`, `~/Documents`, or the user's project directory).

---

## Advanced: Programmatic Usage (Go)

If you need to use llm-web-parser as a library:

```go
import (
    "github.com/dtnitsch/llm-web-parser/pkg/parser"
    "github.com/dtnitsch/llm-web-parser/pkg/fetcher"
)

// Fetch and parse
result, err := fetcher.Fetch("https://example.com")
parsed, err := parser.Parse(result.HTML)
```

Add to `go.mod`:
```bash
go get github.com/dtnitsch/llm-web-parser@latest
```

---

## See Also

- **Complete command reference:** `docs/development/index.yaml`
- **Field reference:** `llm-web-parser-results/FIELDS.yaml` (auto-generated)
- **Quick start:** `llm-web-parser --coldstart`
