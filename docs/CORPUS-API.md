# Corpus API Reference

**Version:** 1.0
**Purpose:** API for LLMs to query and analyze web content corpora

## Overview

The Corpus API provides 11 semantic verbs for operating on collections of parsed web pages. Each verb returns structured JSON with confidence scores, coverage metrics, and explicit unknowns.

**Design principles:**
- Deterministic (same input = same output)
- Structured responses (no prose)
- Confidence + coverage metadata on all responses
- Token-limited (500 tokens max, use `--verbose` for more)

## Response Format (All Verbs)

```json
{
  "verb": "extract",
  "data": { ... },
  "confidence": 0.85,
  "coverage": 0.92,
  "unknowns": ["missing field X", "ambiguous Y"],
  "error": {
    "error_type": "not_implemented",
    "message": "EXTRACT not implemented yet",
    "suggested_actions": ["wait for implementation"]
  }
}
```

## The 11 Verbs

### 1. INGEST
**Purpose:** Fetch and parse URLs into corpus
**Status:** Not implemented
**Example:** `lwp corpus ingest --urls="url1,url2"`

### 2. EXTRACT
**Purpose:** Schema-driven extraction (code, tables, definitions, etc.)
**Status:** Not implemented
**Example:** `lwp corpus extract --schema=Code --session=1`

**Schemas (v1.0):**
- Code (language, content, line_numbers)
- Definition (term, definition_text, source_context)
- Table (headers, rows, caption)
- Quote (text, author, context)
- Methodology (section_text, approach, tools)
- Citation (number, text, context)
- Reference (index, full_text, doi, arxiv_id)

### 3. NORMALIZE
**Purpose:** Canonicalize entities, dates, versions, code
**Status:** Not implemented
**Example:** `lwp corpus normalize entities --concept="neural network" --session=1`

### 4. COMPARE
**Purpose:** Cross-document analysis (consensus, contradictions, approaches)
**Status:** Not implemented
**Example:** `lwp corpus compare consensus --field=recommendation --threshold=0.8 --session=1`

### 5. DETECT
**Purpose:** Pattern recognition (clusters, warnings, gaps, anomalies, trends)
**Status:** Not implemented
**Example:** `lwp corpus detect clusters --objective="group by topic" --session=1`

### 6. TRACE
**Purpose:** Citation graphs, authority scoring, provenance
**Status:** Not implemented
**Example:** `lwp corpus trace citations --format=graph --session=1`

### 7. SCORE
**Purpose:** Confidence and quality metrics
**Status:** Not implemented
**Example:** `lwp corpus score authority --criteria=citations --session=1`

### 8. QUERY
**Purpose:** Boolean filtering over metadata (v1: metadata-only, no semantic)
**Status:** Not implemented
**Example:** `lwp corpus query --filter="has_code AND citations>50" --session=1`

**Supported filters (v1.0):**
- Boolean: AND, OR, NOT
- Comparison: =, !=, >, <, >=, <=
- Fields: content_type, has_abstract, citations, word_count, etc.

**Not supported (v2.0):**
- Semantic queries ("argues against X", "introduces Y")

### 9. DELTA
**Purpose:** Incremental updates (what changed since baseline)
**Status:** Not implemented
**Example:** `lwp corpus delta --since=2024-01-15T10:00:00Z --session=1`

### 10. SUMMARIZE
**Purpose:** Structured synthesis (decision-inputs, timelines, matrices)
**Status:** Not implemented
**Example:** `lwp corpus summarize decision-inputs --question="Which library?" --session=1`

**Important:** No judgment - provides inputs, NOT recommendations

### 11. EXPLAIN_FAILURE
**Purpose:** Diagnostic transparency for low confidence / failures
**Status:** Not implemented
**Example:** `lwp corpus explain-failure --verb=extract --extraction-id=123`

## Error Handling

**Unknown verb:**
```json
{
  "error": {
    "error_type": "unknown_verb",
    "message": "Verb 'search' not recognized. Did you mean 'query'?",
    "suggested_actions": ["Use 'query' verb", "See docs/CORPUS-API.md for verb list"]
  }
}
```

**Not implemented:**
```json
{
  "error": {
    "error_type": "not_implemented",
    "message": "EXTRACT verb not implemented yet",
    "suggested_actions": ["Check implementation status", "Wait for v1.0 release"]
  }
}
```

## Implementation Status

| Verb | Status | ETA |
|------|--------|-----|
| INGEST | Placeholder | TBD |
| EXTRACT | Placeholder | Next |
| NORMALIZE | Placeholder | TBD |
| COMPARE | Placeholder | TBD |
| DETECT | Placeholder | TBD |
| TRACE | Placeholder | TBD |
| SCORE | Placeholder | TBD |
| QUERY | Placeholder | After EXTRACT |
| DELTA | Placeholder | TBD |
| SUMMARIZE | Placeholder | TBD |
| EXPLAIN_FAILURE | Placeholder | TBD |

## Next Steps

**Current:** Skeleton with all verbs returning "NOT IMPLEMENTED YET"
**Next:** Implement QUERY verb (simplest - metadata filtering)
**Then:** One verb at a time, tested before moving on
