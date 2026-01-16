# LLM Cognitive Contract - The Official Interface for lwp

## Problem Statement

**Current state:**
- We have rich data (content type detection, specialized extractors, queryable DB)
- We have implementation primitives (fetch, parse, store)
- **We DON'T have an LLM-facing cognitive interface**

**The gap:**
LLMs currently have to:
1. Read raw summary files (20KB+ of YAML)
2. Manually extract insights from generic.yaml (1MB each)
3. Re-discover duplicates, contradictions, patterns
4. Waste tokens on mechanical work

**What we need:**
> **An official LLM Contract that defines stable, semantic verbs for offloading mechanical reasoning to deterministic tools.**

This is NOT a REST API design. This is a **cognitive contract** between LLM intelligence and deterministic execution.

---

## Design Philosophy (From ChatGPT + Claude)

### The Two-Layer Architecture

**Layer 1: LLM Contract (Official, Stable)**
- Small, opinionated verb set
- Semantically rich (intent-based, not implementation-based)
- Changes rarely (versioned explicitly)
- What the LLM "thinks with"

**Layer 2: Execution API (Internal, Flexible)**
- REST/gRPC/async jobs
- Batch sizes, sharding, retry semantics
- Can change weekly without breaking LLM behavior

### Key Principle
> **The official interface is not a protocol — it's a cognitive contract.**

If an LLM asks "Which verb should I use?" → Success
If an LLM asks "Which endpoint do I call?" → Failure

---

## The Verb Set (Closed Vocabulary)

### 1. INGEST Family (Data Acquisition)
**Purpose:** Get data into the system

```
lwp ingest --urls <list> --mode=<minimal|full>
lwp ingest --session <id> --refetch-failed
```

**Contract:**
- Returns: Session ID + summary stats
- Guarantees: Deduplicated, normalized storage
- Failures: Explicit (failed-urls.yaml)

---

### 2. EXTRACT Family (Schema-Driven Retrieval)
**Purpose:** Pull exact data matching a schema without LLM interpretation

```
lwp extract --session <id> --schema=Code --filters='{"language":"python"}'
lwp extract --session <id> --schema=Definition --filters='{"term":"transformer"}'
lwp extract --session <id> --schema=Table --format=csv
lwp extract --session <id> --schema=Quote --filters='{"min_words":10}'
lwp extract --session <id> --schema=Methodology --filters='{"content_type":"academic"}'
```

**Contract:**
- Returns: Typed objects matching schema (JSON/CSV/YAML)
- Guarantees: Exact, deterministic (no approximation)
- Includes: Source attribution, confidence scores, coverage metrics
- Schema-driven: Content shape is defined by schema, not verb name

**Available Schemas v1.0:**
- Code (language, context, line_numbers)
- Definition (term, definition_text, source_context)
- Table (headers, rows, caption)
- Quote (text, author, context, word_count)
- Methodology (section_text, approach, tools_mentioned)
- Citation (number, text, context)
- Reference (index, full_text, doi, arxiv_id)

**Why:** LLMs hallucinate during extraction; this doesn't. Schema-driven prevents feature explosion.

---

### 3. NORMALIZE Family (Canonicalization)
**Purpose:** Reduce entropy before LLM reasoning

```
lwp normalize entities --session <id> --concept="neural network"
lwp normalize dates --session <id> --format=iso8601
lwp normalize versions --session <id> --package=python
lwp normalize code --session <id> --language=python --style=black
```

**Contract:**
- Returns: Mapping tables (original → canonical)
- Guarantees: Deterministic equivalence rules
- Idempotent: Same input = same output

**Why:** Prevents LLM from rediscovering "these three things are the same."

---

### 4. COMPARE Family (Cross-Document Analysis)
**Purpose:** Structural comparison without interpretation

```
lwp compare versions --session <id> --doc-a=<url_id> --doc-b=<url_id>
lwp compare consensus --session <id> --field=recommendation --threshold=0.8
lwp compare contradictions --session <id> --field=best-practice
lwp compare approaches --session <id> --schema=pros-cons
```

**Contract:**
- Returns: Structured diff objects
- Guarantees: Enumeration of all differences
- Includes: Agreement/disagreement counts

**Why:** Decisions live in differences, not summaries.

---

### 5. DETECT Family (Pattern Recognition)
**Purpose:** Surface signals for LLM attention

```
lwp detect clusters --session <id> --objective="group by topical similarity" --constraints='{"max_clusters":5}'
lwp detect warnings --session <id> --objective="flag cautionary language" --constraints='{"min_strength":"high"}'
lwp detect gaps --session <id> --objective="find missing expected content" --constraints='{"expected_topics":["intro","methods","results"]}'
lwp detect anomalies --session <id> --objective="outlier detection" --constraints='{"field":"citation_count"}'
lwp detect trends --session <id> --objective="temporal pattern analysis" --constraints='{"field":"recommendation"}'
```

**Contract:**
- Returns: Ranked patterns with confidence scores + algorithm used (transparent)
- Guarantees: Statistical summaries, reproducible with same constraints
- Includes: Evidence pointers (which docs contribute), method explanation
- Algorithm-agnostic: Objective defines intent, system chooses best method
- Transparency: Always reports which algorithm was used (tfidf, embeddings, etc.)

**Why:** Tells LLM where to look harder. Intent-based prevents algorithm lock-in.

---

### 6. TRACE Family (Provenance & Authority)
**Purpose:** Build citation/influence graphs

```
lwp trace citations --session <id> --format=graph
lwp trace authority --session <id> --rank-by=citations
lwp trace influence --session <id> --source=<url_id>
lwp trace freshness --session <id> --baseline-date=2024-01-01
```

**Contract:**
- Returns: Graph structures (nodes + edges)
- Guarantees: Complete citation chains
- Includes: Authority scores, freshness metrics

**Why:** LLM shouldn't guess what to trust.

---

### 7. SCORE Family (Epistemic Hygiene)
**Purpose:** Attach confidence/quality metrics

```
lwp score authority --session <id> --criteria=citations
lwp score freshness --session <id>
lwp score coverage --session <id> --topic="transformers"
lwp score confidence --session <id> --extraction=<extraction_id>
```

**Contract:**
- Returns: Numeric scores + reasoning
- Guarantees: Transparent scoring criteria
- Includes: Score distribution across corpus

**Why:** Prevents false confidence.

---

### 8. QUERY Family (Targeted Retrieval)
**Purpose:** Boolean logic filtering over metadata (v1: metadata-only)

```
lwp query --session <id> --filter="has_abstract AND citations>50 AND year>=2020"
lwp query --session <id> --filter="content_type=academic AND has_code_examples"
lwp query --session <id> --filter="domain_type=.edu AND word_count>5000"
```

**Contract v1.0:**
- Returns: List of matching URL IDs + metadata
- Guarantees: Exact boolean evaluation
- Supports: AND, OR, NOT, comparison operators (=, !=, >, <, >=, <=)
- **Scope:** Metadata fields only (structural properties, counts, flags)
- **Not supported in v1:** Semantic/conceptual filters ("argues against X", "introduces Y")

**Reserved for v2.0:**
- Semantic predicates over content
- Conceptual similarity matching
- Argument stance detection
- Prerequisite vs introduction distinction

**Why:** Token-efficient filtering before deep read. Explicitly metadata-only to set expectations.

---

### 9. DELTA Family (Incremental Updates)
**Purpose:** Only show what changed

```
lwp delta --session <id> --since=<timestamp>
lwp delta --session <id> --baseline=<session_id> --only=new-contradictions
lwp delta --session <id> --watch=<url_id> --field=content
```

**Contract:**
- Returns: Diff objects (added/removed/modified)
- Guarantees: Deterministic change detection
- Idempotent: Same baseline = same delta

**Why:** Prevents reloading the world when one fact changed.

---

### 10. SUMMARIZE Family (Structured Synthesis)
**Purpose:** Emit thinking substrates, not prose (NO JUDGMENT)

```
lwp summarize coverage --session <id> --topic="transformers"
lwp summarize decision-inputs --session <id> --question="Which library?"
lwp summarize timeline --session <id> --field=published_at
lwp summarize comparison-matrix --session <id> --items=<url_ids> --schema=<fields>
```

**Contract:**
- Returns: Structured objects (tables, graphs, timelines) - NEVER prose
- Guarantees: Data-driven (no hallucination), no recommendations
- Includes: Confidence scores, unknowns as first-class data
- **Forbidden outputs:** "best choice", "recommendation", "conclusion", "you should"
- **Required fields for decision-inputs:** options, evidence, disagreements, unknowns

**Output Structure for decision-inputs:**
```json
{
  "options": [...],
  "evidence_for": {...},
  "evidence_against": {...},
  "disagreements": [...],
  "consensus_points": [...],
  "unknowns": [...],
  "coverage": 0.85,
  "confidence": 0.72
}
```

**Why:** These are thinking substrates, not summaries. Tool provides inputs, LLM makes decision.

---

### 11. EXPLAIN_FAILURE Family (Diagnostic Transparency)
**Purpose:** Explain why verbs returned low confidence or partial coverage

```
lwp explain-failure --verb=extract --extraction-id=<id>
lwp explain-failure --verb=detect --detection-id=<id>
lwp explain-failure --session <id> --url-id=<id>
```

**Contract:**
- Returns: Structured diagnostic information
- Includes: Missing data, ambiguous patterns, parsing errors, coverage gaps
- Guarantees: Actionable feedback (what to fix, what to provide)
- Format: Always structured, never vague ("low quality" → specific issues)

**Output Structure:**
```json
{
  "verb": "extract",
  "target": "Code",
  "confidence": 0.42,
  "issues": [
    {
      "type": "ambiguous_language_detection",
      "affected_blocks": [12, 15, 23],
      "recommendation": "Specify language explicitly or improve code fence markers"
    },
    {
      "type": "incomplete_coverage",
      "missing_from": ["url_id:5", "url_id:12"],
      "reason": "No code blocks found in these documents"
    }
  ],
  "fixable": true,
  "suggested_actions": ["Add --filters language=python", "Check content_type metadata"]
}
```

**Why:** Helps LLM avoid false confidence and understand tool limitations.

---

## Corpus Views (Not Just Sessions)

**Problem:** Sessions are convenient early but LLMs need flexible corpus reuse.

**Solution:** Make "corpus views" first-class:

### View Types
1. **Session view** (current default)
   - All URLs from a session
   - Immutable snapshot

2. **Query view** (saved filters)
   - `lwp view create --name="high-quality-papers" --filter="content_type=academic AND citations>50"`
   - Reusable across sessions

3. **Explicit set view** (manual curation)
   - `lwp view create --name="comparison-set" --urls=1,5,12,23`
   - Cross-session subset

4. **Evolving view** (dynamic)
   - `lwp view create --name="recent-docs" --filter="content_type=docs" --since="2024-01-01" --auto-update`
   - Changes as new docs ingested

### Benefits
- No re-ingesting for partial corpus analysis
- Cross-session reuse
- Evolving world models
- DELTA and QUERY naturally work with views

---

## Contract Enforcement

### 1. Versioning
```yaml
lwp_contract_version: 1.0
```
Breaking changes require version bump.

### 2. Schema Validation
Every verb:
- Requires explicit schema
- Returns typed objects
- Includes confidence + coverage metadata

### 3. Rejection Semantics
Unknown verbs → Hard fail with suggestion
Malformed requests → Schema validation error
Ambiguous requests → Require disambiguation

### 4. Documentation Target
**Write for models, not humans:**
- When should I use this verb?
- What does it NOT do?
- What mistakes will cause rejection?

**Do NOT write:**
- HTTP status codes
- Curl examples
- Implementation details

### 5. Token Limit Rule
**Critical contract constraint:**
> The tool must never return prose longer than 500 tokens unless explicitly requested via --verbose flag.

**Rationale:** Prevents slow drift into "helpful summaries" that waste LLM context.

**Enforcement:**
- All responses are structured (JSON/YAML/CSV)
- Error messages: max 100 tokens
- Help text: max 200 tokens
- Verbose mode must be explicit opt-in

---

## ChatGPT Feedback Integration

### Non-Negotiable Changes (v1 Blockers)
✅ 1. **Remove algorithm names from LLM-facing contract** - Use objective + constraints instead
✅ 2. **Collapse EXTRACT variants into schema-driven extraction** - Prevents feature explosion
✅ 3. **Rename decision-leaning summarize verbs** - "decision-brief" → "decision-inputs"
✅ 4. **Explicitly mark QUERY as metadata-only** - Reserve semantic filters for v2

### Nice-to-Have (High Value)
✅ 5. **Add EXPLAIN_FAILURE verb** - Diagnostic transparency for low confidence
✅ 6. **Add token limit rule** - Prevent prose creep
✅ 7. **Add corpus views** - Session + query + explicit + evolving views

### ChatGPT's Verdict
> "This is one of the best LLM-tool interface designs I've seen. Not 'good for a side project' — **good enough to ossify and build a platform on.**"

**Key insight:** We're 80-85% done with the design, these changes bring us to production-ready v1.0.

---

## Implementation Phases

### Phase 1: Foundation (Current State)
✅ Content type detection
✅ Specialized extractors (academic, docs, wiki)
✅ Rich metadata in DB
✅ URL-centric artifact storage

### Phase 2: Contract Foundation (Week 1)
**Goal:** Establish contract infrastructure

**Deliverables:**
1. Schema registry system (`pkg/contract/schemas.go`)
2. Request/response validation (`pkg/contract/validator.go`)
3. Corpus views infrastructure (`pkg/corpus/views.go`)
4. Contract versioning mechanism
5. Error message token limiting

**Files to create:**
- `pkg/contract/schemas.go` - Schema definitions for all extract types
- `pkg/contract/validator.go` - Request validation + rejection semantics
- `pkg/corpus/views.go` - Session, query, explicit, evolving views
- `models/contract.go` - Request/response type definitions

### Phase 3: Core Verbs (Week 2-3)
**Priority 1: EXTRACT (schema-driven)**
- Implement schema registry (Code, Definition, Table, Quote, etc.)
- Build extraction engine that routes to specialized extractors
- Add source attribution + confidence scoring

**Priority 2: QUERY (metadata-only v1)**
- Boolean filter parser (AND, OR, NOT, comparison ops)
- Metadata field enumeration
- Explicit "not supported" messaging for semantic filters

**Priority 3: EXPLAIN_FAILURE**
- Diagnostic framework for all verbs
- Structured issue reporting
- Actionable recommendations

### Phase 4: Analysis Verbs (Week 4-5)
**Priority 1: COMPARE**
- Cross-document diff engine
- Contradiction detection
- Consensus calculation

**Priority 2: DETECT (intent-based)**
- Objective → algorithm mapping
- Pattern recognition with transparent methods
- Confidence scoring

### Phase 5: Advanced Features (Week 6+)
- NORMALIZE family (entity canonicalization)
- TRACE family (citation graphs)
- DELTA family (incremental updates)
- SUMMARIZE family (structured synthesis)

---

## Success Metrics

**For LLMs:**
1. Can select correct verb from description
2. Can construct valid request without example
3. Reduced token usage (10x less than manual extraction)
4. Increased accuracy (deterministic vs hallucinated)

**For System:**
1. All verbs complete in <2s for 50-document corpus
2. Schema validation catches 95%+ of errors
3. Idempotent operations (same input = same output)
4. Versioned contract with migration path

---

## Critical Files

### Phase 2: Contract Foundation
**To Create:**
- `docs/LLM-CONTRACT.md` - Official verb set (for models, not humans)
- `docs/COGNITIVE-API-DESIGN.md` - This plan file, frozen as design doc
- `pkg/contract/schemas.go` - Extract schema registry
- `pkg/contract/validator.go` - Request validation + token limits
- `pkg/corpus/views.go` - View types (session, query, explicit, evolving)
- `models/contract.go` - Request/response types for all verbs

**To Modify:**
- `main.go` - Add corpus commands (extract, query, compare, detect, etc.)
- `pkg/db/schema.go` - Add views table
- `pkg/db/operations.go` - View CRUD operations

### Phase 3: Core Verbs
**To Create:**
- `internal/corpus/extract.go` - Schema-driven extraction engine
- `internal/corpus/query.go` - Boolean filter parser + evaluator
- `internal/corpus/explain.go` - Diagnostic framework
- `pkg/contract/filters.go` - Filter syntax parser

**To Modify:**
- `pkg/extractors/` - Add confidence scoring to all extractors
- `internal/fetch/summary.go` - Add coverage metrics

### Phase 4: Analysis Verbs
**To Create:**
- `internal/corpus/compare.go` - Cross-document comparison
- `internal/corpus/detect.go` - Pattern recognition with transparent methods
- `pkg/analytics/` - Statistical analysis utilities

---

## Immediate Next Steps (Before Implementation)

### 1. Freeze This Design as Official Doc (TODAY)
```bash
cp ~/.claude/plans/twinkly-plotting-locket.md docs/COGNITIVE-API-DESIGN.md
```
This becomes the source of truth for v1.0 contract.

### 2. Create LLM-Facing Documentation (Week 1, Day 1-2)
Write `docs/LLM-CONTRACT.md` following these rules:
- Answer: "When should I use this verb?"
- Answer: "What does it NOT do?"
- Answer: "What mistakes will cause rejection?"
- NO HTTP status codes, curl examples, implementation details
- Include example request/response for each verb family

### 3. Schema Design Session (Week 1, Day 3)
Design concrete JSON schemas for:
- Extract schemas (Code, Definition, Table, etc.)
- Request format (verb, target, constraints, filters)
- Response format (data, confidence, coverage, unknowns)
- Error format (type, message, suggested_actions)

### 4. Implement Token Limiting First (Week 1, Day 4)
Before any verb implementation:
- Add response size validation
- Enforce 500 token limit on all outputs
- Test with deliberately verbose responses
- **This prevents prose creep from day one**

### 5. Build Contract Validator (Week 1, Day 5)
Before implementing verbs:
- Verb registry with closed vocabulary
- Schema validator for all request types
- Rejection semantics with helpful errors
- **This prevents malformed requests early**

---

## Design Decisions (Resolved)

### 1. Sessions vs URL sets?
**Decision:** Corpus views as first-class abstraction
- Sessions remain for historical compatibility
- Views enable flexible corpus composition
- All verbs accept `--view` or `--session`

### 2. Sync vs async execution?
**Decision:** Hybrid approach
- Sync for <100 documents (most use cases)
- Async with job IDs for >100 documents
- `--async` flag for explicit async opt-in

### 3. Output format: JSON vs YAML?
**Decision:** Content-type negotiation
- JSON default (LLM parsing optimized)
- YAML available via `--format=yaml` (human debugging)
- CSV for tabular data (export compatibility)

### 4. Unknown unknowns?
**Decision:** Explicit metadata fields
- All responses include: `confidence`, `coverage`, `unknowns`
- EXPLAIN_FAILURE verb for deep diagnostics
- No "success" response without these fields

---

## Final Note on Contract Stability

**This v1.0 contract should ossify.**

Changes allowed:
- New schemas in EXTRACT family
- New objectives in DETECT family
- Performance improvements (invisible to LLM)

Changes forbidden without v2.0:
- Changing verb names
- Changing request format
- Removing response fields
- Breaking determinism guarantees

**The LLM contract is a cognitive API, not a REST API.**
