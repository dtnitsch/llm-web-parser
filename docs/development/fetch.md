# Fetch Command Reference

## Basic Patterns

```bash
# Single URL
lwp fetch --urls="http://example.com"

# Multiple URLs (comma-separated)
lwp fetch --urls="url1,url2,url3"
```

---

## Parse Modes

```bash
# Minimal (metadata only, NO keywords)
lwp fetch --urls="..." --features=minimal

# Wordcount (DEFAULT - metadata + keywords)
lwp fetch --urls="..."
lwp fetch --urls="..." --features=wordcount

# Full-parse (complete content extraction)
lwp fetch --urls="..." --features=full-parse
```

---

## Session Operations

```bash
# Refetch entire session
lwp fetch --session=5

# Retry only failed URLs
lwp fetch --session=5 --failed-only

# Upgrade to full-parse
lwp fetch --session=5 --features=full-parse

# Force fresh fetch (ignore cache)
lwp fetch --session=5 --force-fetch
```

---

## Inline Filtering

```bash
# Confidence threshold
lwp fetch --urls="..." --filter="conf:>=0.7"

# Content type
lwp fetch --urls="..." --filter="type:code|table"

# Combined filters
lwp fetch --urls="..." --filter="conf:>=0.8,type:p|code"
```

---

## Output Control

```bash
# Format (YAML is DEFAULT - 30% more token efficient)
lwp fetch --urls="..." --format=yaml
lwp fetch --urls="..." --format=json

# Quiet mode (no URL ID display)
lwp fetch --urls="..." --quiet

# Output mode
lwp fetch --urls="..." --output-mode=tier2    # DEFAULT
```

---

## Performance

```bash
# Worker count (DEFAULT: 8)
lwp fetch --urls="..." --workers=8
lwp fetch --urls="..." --workers=16

# Cache freshness
lwp fetch --urls="..." --max-age=1h    # DEFAULT
lwp fetch --urls="..." --max-age=24h
lwp fetch --urls="..." --max-age=0s    # Always fetch fresh
```

---

## Workflows

### Fast Scan → Deep Dive

```bash
# Step 1: Quick metadata scan
lwp fetch --urls="url1,url2,...,url50"

# Step 2: Analyze results
lwp db get --file=details | yq '.[] | select(.confidence >= 7)'

# Step 3: Deep parse selected URLs
lwp fetch --session=1 --features=full-parse
```

### Failed URL Retry

```bash
# Step 1: Initial fetch (some may fail)
lwp fetch --urls="url1,url2,url3"

# Step 2: Retry only failures
lwp fetch --session=2 --failed-only
```
