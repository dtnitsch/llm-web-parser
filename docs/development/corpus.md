# Corpus Command Reference

## Extract (Keyword Aggregation)

```bash
# Aggregate all URLs in session
lwp corpus extract --session=5

# Control number of keywords
lwp corpus extract --session=5 --top=25    # DEFAULT
lwp corpus extract --session=5 --top=50
lwp corpus extract --session=5 --top=0     # All keywords

# Specific URLs only
lwp corpus extract --url-ids=1,2,3
lwp corpus extract --url-ids=42

# Output formats
lwp corpus extract --session=5 --format=yaml    # DEFAULT
lwp corpus extract --session=5 --format=json
lwp corpus extract --session=5 --format=csv
```

---

## Suggest (Query Suggestions)

```bash
# Get suggested queries for session
lwp corpus suggest --session=5
```

---

## Not Yet Implemented

The following verbs are planned but not yet available:

```bash
# Query (boolean filtering)
lwp corpus query --session=5 --filter="has_code"               # NOT IMPLEMENTED

# Compare (cross-document analysis)
lwp corpus compare --session=5                                 # NOT IMPLEMENTED

# Detect (pattern recognition)
lwp corpus detect --session=5                                  # NOT IMPLEMENTED

# Other verbs
lwp corpus normalize --session=5                               # NOT IMPLEMENTED
lwp corpus trace --session=5                                   # NOT IMPLEMENTED
lwp corpus score --session=5                                   # NOT IMPLEMENTED
lwp corpus delta --session=5                                   # NOT IMPLEMENTED
lwp corpus summarize --session=5                               # NOT IMPLEMENTED
lwp corpus explain-failure --session=5                         # NOT IMPLEMENTED
```

See `docs/CORPUS-API.md` for implementation status and roadmap.

---

## Workflows

### Keyword-Based Triage

```bash
# Step 1: Fetch with keywords
lwp fetch --urls="..."

# Step 2: Extract top keywords
lwp corpus extract --session=1 --top=10

# Step 3: Read URLs about specific topics
# (Once corpus query is implemented:)
# lwp corpus query --session=1 --keyword=error
```
