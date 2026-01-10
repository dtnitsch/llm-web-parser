# JSON Schema Reference

**Output format for parsed web pages**

## Page Schema

```jsonc
{
  "url": "string",              // Original URL fetched
  "title": "string",            // Page title (<title> tag)

  // Full parsing mode (hierarchical sections)
  "content": [                  // Array of top-level sections
    {
      "id": "section-N",        // Unique section identifier
      "heading": {              // Optional section heading
        "id": "block-N",
        "type": "h1|h2|h3|h4|h5|h6",
        "text": "string",
        "confidence": 0.0-1.0   // Heading confidence (typically 0.7)
      },
      "level": 1-6,             // Heading level (h1=1, h2=2, etc.)
      "blocks": [               // Content blocks in this section
        {
          "id": "block-N",
          "type": "p|li|code|table|...",
          "text": "string",     // Plain text (for p, li)

          // Structured content (type-specific)
          "table": {            // Only present if type == "table"
            "headers": ["string"],
            "rows": [["string"]]
          },
          "code": {             // Only present if type == "code"
            "language": "string",  // Detected language (e.g., "python")
            "content": "string"    // Raw code content
          },

          // Links found in this block
          "links": [            // Optional
            {
              "href": "string",
              "text": "string",
              "type": "internal|external"
            }
          ],

          "confidence": 0.0-1.0  // Block confidence score
        }
      ],
      "children": [             // Nested subsections (recursive)
        // ... same Section structure
      ]
    }
  ],

  // Cheap parsing mode (flat structure, omitted if using full mode)
  "flat_content": [             // Array of blocks without hierarchy
    // ... same ContentBlock structure as above
  ],

  // Metadata about the page
  "metadata": {
    "content_type": "documentation|article|landing|unknown",
    "language": "en|es|fr|...|unknown",
    "language_confidence": 0.0-1.0,
    "word_count": 1234,
    "estimated_read_min": 5.5,
    "section_count": 8,
    "block_count": 42,
    "computed": true,
    "extraction_mode": "cheap|full",
    "extraction_quality": "ok|low|degraded"
  }
}
```

---

## Confidence Scores

Confidence scores indicate content quality and signal strength (0.0-1.0 scale).

| Confidence | Meaning | Typical Content |
|------------|---------|-----------------|
| **0.95** | Very high | Tables, code blocks (structured data) |
| **0.7** | High | Headings (h1-h6), dense paragraphs |
| **0.5-0.6** | Medium | Regular paragraphs, list items |
| **0.3-0.4** | Low | Navigation, footers, UI chrome |
| **<0.3** | Very low | Likely spam or boilerplate |

**Best Practice:** Filter by `confidence >= 0.7` for high-signal content.

---

## Block Types

| Type | Description | Typical Confidence | Structured Field |
|------|-------------|-------------------|------------------|
| `p` | Paragraph | 0.5-0.7 | `.text` |
| `li` | List item | 0.5-0.6 | `.text` |
| `code` | Code block | 0.95 | `.code.content`, `.code.language` |
| `table` | Table | 0.95 | `.table.headers`, `.table.rows` |
| `h1`-`h6` | Headings | 0.7 | `.text` |

**Note:** Structured types (`code`, `table`) always have confidence 0.95.

---

## Content Type Heuristics

Automatically detected based on page structure:

| Content Type | Characteristics | Use For |
|-------------|-----------------|---------|
| `documentation` | High code/table density (5+ code blocks) | API docs, tutorials, guides |
| `article` | Long-form text (1200+ words), 8+ sections | Blog posts, news articles, essays |
| `landing` | Short text (<500 words), ≤2 sections | Marketing pages, product homepages |
| `unknown` | Doesn't match patterns | Fallback |

---

## Extraction Quality

| Quality | Meaning | Action |
|---------|---------|--------|
| `ok` | Extraction succeeded, content is clean | Use normally |
| `low` | Partial failure, may be missing sections | Re-run with full mode or skip |
| `degraded` | Significant parsing issues | Consider using headless browser |

**Check quality before analysis:**
```bash
jq -r '.[] | select(.metadata.extraction_quality != "ok") | .url' llm-web-parser-results/parsed/*.json
```

---

## Link Types

| Type | Meaning | Example |
|------|---------|---------|
| `internal` | Same-domain link | `/docs/api` (for depth-first crawling) |
| `external` | Different-domain link | `https://other-site.com` (for citations) |

**Extract internal links for recursive crawling:**
```bash
jq -r '.content[].blocks[].links[]? | select(.type == "internal") | .href' file.json
```

---

## Metadata Fields

| Field | Type | Description |
|-------|------|-------------|
| `content_type` | string | Detected page type (see Content Type Heuristics) |
| `language` | string | ISO 639-1 language code (`en`, `es`, etc.) |
| `language_confidence` | float | Detection confidence (0.75+ is reliable) |
| `word_count` | int | Total words in all text blocks |
| `estimated_read_min` | float | Reading time (word_count / 225) |
| `section_count` | int | Total sections (including nested) |
| `block_count` | int | Total content blocks |
| `computed` | bool | Whether metadata has been computed |
| `extraction_mode` | string | Parser mode used (`cheap` or `full`) |
| `extraction_quality` | string | Quality assessment (see above) |

---

## Example Queries

### Extract high-confidence paragraphs
```bash
jq -r '.content[].blocks[] | select(.confidence >= 0.8 and .type == "p") | .text' file.json
```

### Count code blocks
```bash
jq '[.content[].blocks[] | select(.type == "code")] | length' file.json
```

### Get all tables
```bash
jq '.content[].blocks[] | select(.type == "table") | .table' file.json
```

### Find sections by heading text
```bash
jq '.content[] | select(.heading.text | test("API Reference"; "i"))' file.json
```

### Calculate token budget
```bash
jq '.metadata.estimated_tokens' file.json
# Or: word_count / 2.5
```

### Extract external citations
```bash
jq -r '.content[].blocks[].links[]? | select(.type == "external") | .href' file.json
```

---

## Hierarchical Structure Example

```json
{
  "content": [
    {
      "id": "section-1",
      "level": 0,
      "blocks": [/* intro paragraphs */],
      "children": [
        {
          "id": "section-2",
          "heading": {"type": "h2", "text": "Getting Started"},
          "level": 2,
          "blocks": [/* getting started content */],
          "children": [
            {
              "id": "section-3",
              "heading": {"type": "h3", "text": "Installation"},
              "level": 3,
              "blocks": [/* installation steps */]
            }
          ]
        }
      ]
    }
  ]
}
```

**Navigation pattern:**
- Top-level sections: `page.content`
- H2 sections: `page.content[].children` where `level == 2`
- H3 subsections: `page.content[].children[].children` where `level == 3`

---

## Token Estimation

```javascript
estimated_tokens = word_count / 2.5
```

**Use this to:**
- Budget LLM API costs before reading
- Filter pages by size
- Decide whether to use `extract` command for filtering

```bash
# Find pages under 500 tokens
jq -r 'select(.metadata.estimated_tokens < 500) | .url' llm-web-parser-results/parsed/*.json
```

---

## See Also

- [CLI-REFERENCE.md](./CLI-REFERENCE.md) - Command-line interface
- [LLM-USAGE.md](../LLM-USAGE.md) - LLM integration patterns
- [README.md](../README.md) - Quick start and examples
