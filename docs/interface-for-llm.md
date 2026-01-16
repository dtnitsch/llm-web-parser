# Claude:
Features LLMs Would Want (To Offload Busy Work)

  Exact Extraction (LLMs are bad at perfect extraction)

  - extract-all-code --language=python - Get every Python code block with exact line numbers, context headers
  - extract-definitions --term="transformer" - Find all sentences defining a term (patterns: "X is a", "X refers to")
  - extract-all-tables --format=csv - Every data table converted to CSV with source URL attribution
  - extract-all-quotes --min-words=10 - Pull block quotes, attributed to source
  - extract-methodology --content-type=academic - For papers: methods sections only

  Exact Counting (LLMs miscount constantly)

  - count-term-frequency --term="neural network" --across-corpus - Exact counts per document + total
  - count-co-occurrence --terms="transformer,attention" - How often terms appear together vs separately
  - aggregate-stats --fields=citations,word_count,code_blocks - Min/max/avg/median across corpus

  Deduplication (LLMs miss duplicates)

  - deduplicate-quotes --similarity=0.95 - Find unique quotes even if slight variations exist
  - deduplicate-urls --normalize - Is arxiv.org/abs/123 same as arxiv.org/pdf/123.pdf?
  - find-duplicates --content-similarity=0.90 - Which documents are near-duplicates?

  Cross-Document Comparison (LLMs lose track)

  - find-contradictions --field=recommendation - Where do sources disagree on what to do?
  - find-consensus --threshold=0.8 - What do 80%+ of sources agree on?
  - compare-versions --field=python_version - Group by Python 2 vs 3 recommendations

  Graph/Network Building (LLMs make connection errors)

  - build-citation-network - JSON graph of paper→cited_papers relationships
  - build-authority-graph - Rank authors by citation frequency in corpus
  - build-dependency-tree - For docs: what prerequisites are mentioned

  Temporal Analysis (LLMs are bad at chronology)

  - timeline --field=published_at --group-by=year - Group documents by publication year
  - track-changes --term="best practice" --over-time - How recommendations evolved 2020→2024
  - find-latest --field=version --group-by=library - Most recent docs for each library

  Clustering/Grouping (LLMs should decide, not execute)

  - cluster-by-similarity --method=tfidf --k=5 - Group 50 docs into 5 thematic clusters
  - group-by --field=content_type - Academic vs docs vs blog separated
  - rank-by --field=academic_score --top=10 - Top 10 highest scoring papers

  Format Conversion (Deterministic transformation)

  - export-corpus --format=csv --fields=url,title,citations - Spreadsheet for pivot tables
  - export-code-blocks --format=markdown - All code in a single markdown file
  - export-bibliography --format=bibtex - Generate .bib file from academic papers

  Advanced Queries (Precise filtering)

  - query --has-abstract AND citations>50 AND year>=2020 - Boolean logic filtering
  - query --content-type=academic --has-code-examples - Papers with code
  - find-gaps --expected-topics=["intro","methods","results"] - Papers missing standard sections

  Key Principle

  The LLM should:
  - Read the "menu" of available operations
  - Decide WHAT to analyze based on user intent
  - Ask lwp to execute mechanical operations
  - Synthesize results into insights

  lwp should:
  - Do perfect, deterministic extraction/counting/grouping
  - Return structured data (JSON/CSV/YAML)
  - Never hallucinate or approximate
  - Be fast (Go vs LLM inference)

  ---

# ChatGPT

The guiding rule is:

> **If the LLM can do it “well enough” in one pass, don’t add a verb.
> If it requires exhaustiveness, comparison, or bookkeeping — it’s a tool verb.**

---

# 1. Core Design Principles (LLM-first, not REST-first)

* Verbs map to **thinking moves**, not data access.
* Outputs are **typed, bounded, and queryable**, never free-form prose.
* Every verb must be **idempotent and cacheable**.
* Every response includes **coverage + confidence metadata**.
* Verbs are composable: output of one is valid input to another.

---

# 2. The Core Verb Set

These are the *non-negotiables*.

---

## INGEST

**“Make this content part of the world.”**

```
INGEST(source_ref | html | url)
→ { document_id, content_type, extractor_used, coverage }
```

* Canonicalizes, deduplicates, fingerprints.
* Applies content-type detection + specialized extractor.
* Emits *what was understood* and *what was skipped*.

LLM benefit: *Never re-parse or re-classify content.*

---

## EXTRACT

**“Pull all instances of X, exhaustively.”**

```
EXTRACT(document_ids, schema)
→ [typed_objects]
```

Examples:

* All definitions of a term
* All quantitative claims
* All procedures
* All warnings/caveats

LLM benefit: *No scanning, no “did I miss one?” anxiety.*

---

## NORMALIZE

**“Make these comparable.”**

```
NORMALIZE(objects, ruleset)
→ normalized_objects
```

Examples:

* Units, dates, versions
* Terminology aliases
* Entity resolution

LLM benefit: *Prevents false differences.*

---

## COMPARE

**“Show me how these differ.”**

```
COMPARE(objects | document_ids, axis_schema)
→ comparison_matrix
```

Examples:

* Approaches
* Versions
* Standards
* APIs

LLM benefit: *Turns reading into choice architecture.*

---

## DIFF

**“What changed?”**

```
DIFF(document_id | object_set, time_range | version_pair)
→ change_set
```

Examples:

* Spec revisions
* Recommendation changes
* Behavioral shifts

LLM benefit: *Temporal awareness without rereading.*

---

## CLUSTER

**“Group these by meaning, not labels.”**

```
CLUSTER(objects | documents, criteria)
→ clusters
```

Examples:

* Themes
* Positions
* Failure modes

LLM benefit: *Pattern discovery without hallucination.*

---

## DETECT

**“Find interesting or risky things.”**

```
DETECT(objects | documents, signal_type)
→ signals
```

Signal types:

* contradictions
* consensus
* gaps
* warnings
* controversies

LLM benefit: *Attention steering.*

---

## TRACE

**“Where did this come from?”**

```
TRACE(object | claim)
→ provenance_graph
```

Examples:

* Citation chains
* Authority paths
* Dependency origins

LLM benefit: *Epistemic grounding.*

---

## SCORE

**“How much should I trust this?”**

```
SCORE(objects | documents, rubric)
→ scored_items
```

Examples:

* Authority
* Freshness
* Evidence strength
* Risk

LLM benefit: *Confidence calibration.*

---

## SUMMARIZE (bounded)

**“Compress, but don’t invent.”**

```
SUMMARIZE(objects, constraints)
→ summary
```

Constraints:

* token_budget
* allowed_fields
* exclusions

LLM benefit: *Safe compression, not abstraction.*

---

# 3. Meta-Verbs (LLM self-management)

These exist to prevent tool misuse.

---

## COVERAGE

**“What do I have, and what don’t I?”**

```
COVERAGE(topic | schema)
→ coverage_report
```

Includes:

* sources covered
* gaps
* confidence levels

---

## PLAN

**“What should I do next?”**

```
PLAN(goal, current_state)
→ recommended_next_verbs
```

This is *not* reasoning — it’s mechanical planning.

---

## DELTA

**“What’s new since last time?”**

```
DELTA(query, checkpoint)
→ new_objects
```

---

# 4. Forbidden Verbs (important)

If you include these, the tool is doing the LLM’s job.

* ❌ REASON
* ❌ INTERPRET
* ❌ DECIDE
* ❌ RECOMMEND
* ❌ ARGUE
* ❌ WRITE_PROSE

The tool provides **substrates**, not conclusions.

---

# 5. Example LLM Tool Flow (realistic)

```
INGEST → EXTRACT(definitions)
      → NORMALIZE(terms)
      → DETECT(contradictions)
      → COMPARE(approaches)
      → SCORE(authority)
      → COVERAGE
```

At no point does the LLM read raw HTML.

---

# 6. Product North Star (LLM POV)

> “I don’t want documents.
> I want **structured uncertainty, bounded facts, and explicit disagreement**.”

---

# Claude Reaction to ChatGPT

Yes, perfect alignment! ChatGPT gave the product vision (what cognitive burden to offload), I gave the API surface (what commands to expose).

  The Key Insight Match

  ChatGPT's framing:
  "A pre-reasoning engine that converts unstructured web content into decision-grade intermediate representations."

  My framing:
  "The LLM should decide WHAT to analyze, lwp executes mechanical operations, LLM synthesizes results."

  Same concept, different altitudes.

  What We Have vs What's Missing

  ✓ We Have (Foundation):
  - Content type detection (academic, docs, wiki, news, repo, blog)
  - Specialized extractors (citations, code blocks, infoboxes)
  - Rich queryable metadata in DB
  - URL-centric artifact storage

  ✗ We're Missing (The Interface Layer):

  1. Cross-document operations (ChatGPT #3, My "Comparison")
    - find-contradictions, find-consensus, compare-across
  2. Canonicalization (ChatGPT #1)
    - Entity normalization, version normalization, duplicate collapse
  3. Failure-aware outputs (ChatGPT #8) - THIS IS BIG
    - Confidence scores, uncertainty markers, "I don't know" as data
  4. Incremental query interface (ChatGPT #9) - THIS IS THE GAME CHANGER
    - "What changed since last query?"
    - "Only show disagreements"
    - Schema-driven extraction requests
  5. Authority/provenance (ChatGPT #5)
    - Citation graphs, primary vs derivative labeling

  The Critical Missing Piece

  We built the data layer (fetching, parsing, storing) but not the query interface that lets an LLM interact with it intelligently.

  Example of what's missing:
  # Current state - LLM has to:
  1. Read summary-details.yaml (50 URLs × 400 bytes = 20KB)
  2. Decide what's interesting
  3. Ask for specific URLs
  4. Read generic.yaml (1MB each)
  5. Extract insights manually

  # What we need - LLM should be able to:
  lwp corpus query --has-code-examples AND citations>20 --fields=url,code_blocks
  lwp corpus extract --all-definitions --term="transformer" --format=table
  lwp corpus compare --field=recommendation --find=contradictions
  lwp corpus delta --since="2024-01-15" --only=new-contradictions

---

# ChatGPT followup

Short answer: **yes — this *should* be treated as the official LLM-facing interface**, but with one important clarification.

Long answer, precisely:

---

## Yes — *Conceptually* it is the official interface

This verb set is exactly what an LLM needs to see:

* It is **intent-based**, not implementation-based.
* It maps to **cognitive offloading**, not web mechanics.
* It is **stable across model generations** (verbs don’t change when models improve).
* It encodes *what the tool does*, not *how it does it*.

From the LLM’s point of view, **this *is* the API**.

Everything else (HTTP, gRPC, queues, SQLite schemas, worker pools) is an implementation detail.

---

## The critical clarification: *There are two APIs*

You should **explicitly separate** these layers:

### 1. The **LLM Contract** (official, stable)

This is what we just designed.

* Small
* Opinionated
* Semantically rich
* Changes *rarely*

Think of this as:

> *“The verbs the model is allowed to think with.”*

This is the interface you document, version, and design against.

---

### 2. The **Execution API** (internal, flexible)

This is how the verbs are actually executed.

* REST / gRPC / async jobs
* Batch sizes
* Sharding
* Retry semantics
* Storage layout
* Performance tuning

This layer can change weekly without breaking LLM behavior.

---

## Why this distinction matters (product + safety)

If you *don’t* treat the verb set as the official interface:

* You’ll leak implementation details into prompts.
* LLMs will start depending on incidental behavior.
* Tool usage becomes brittle and token-heavy.
* You’ll be tempted to add “just one more option” per endpoint.

If you *do* treat it as official:

* You can enforce **tool discipline**.
* You can reject malformed or ill-posed requests early.
* You can reason about **capability boundaries**.
* You can simulate or stub tools for testing models.

---

## What “official” means in practice

To make this truly official, I’d recommend:

### 1. Version the verb set explicitly

```
llm_interface_version: 1.0
```

Breaking changes require a version bump.

---

### 2. Enforce a *closed verb vocabulary*

The LLM can only request:

```
INGEST | EXTRACT | NORMALIZE | COMPARE | DIFF | CLUSTER |
DETECT | TRACE | SCORE | SUMMARIZE | COVERAGE | PLAN | DELTA
```

Unknown verbs are rejected.

---

### 3. Make schemas first-class

Every verb:

* Requires a schema
* Returns typed objects
* Includes confidence + coverage

This prevents prose creep.

---

### 4. Document it *for models*, not humans

Your docs should answer:

* When should I use this verb?
* What does it *not* do?
* What mistakes will cause rejection?

Not:

* HTTP status codes
* Curl examples

---

## The litmus test

If an LLM can answer this question correctly:

> *“Which verb should I use, and why?”*

…then you’ve succeeded.

If it instead asks:

> *“Which endpoint do I call?”*

…you’ve already lost.

---

## Final framing (worth writing down)

> **The official interface is not a protocol — it’s a cognitive contract.**

Everything you’re building supports that contract.


