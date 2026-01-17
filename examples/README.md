# llm-web-parser Examples

Ready-to-run examples showing common workflows.

## Prerequisites

Build the binary first:
```bash
cd ..
go build -o llm-web-parser .
```

## Examples

### 01-quick-start.sh
**The simplest possible example** - Fetch one URL and see results

```bash
./01-quick-start.sh
```

**What it demonstrates:**
- Basic fetch with default settings (wordcount mode)
- Finding the database location
- Listing sessions

**Runtime:** 2-5 seconds
**Best for:** First-time users, verifying installation

---

### 02-multi-stage-workflow.sh
**Realistic workflow** - Fast scan → analyze → deep dive

```bash
./02-multi-stage-workflow.sh
```

**What it demonstrates:**
- Fetching multiple URLs with wordcount mode
- Inspecting session details
- Re-fetching the same session with full-parse mode
- Session ID management

**Runtime:** 10-15 seconds
**Best for:** Understanding how to use sessions effectively

---

### 03-corpus-keywords.sh
**Keyword analysis** - Extract themes across multiple documents

```bash
./03-corpus-keywords.sh
```

**What it demonstrates:**
- Fetching multiple documentation sites
- Using `corpus extract` to aggregate keywords
- Understanding corpus themes at a glance

**Runtime:** 10-15 seconds
**Best for:** Learning the corpus API (extract verb)

---

## What Works vs What's Planned

**Currently Working:**
- ✅ `fetch` command (minimal, wordcount, full-parse modes)
- ✅ `db` commands (sessions, get, urls, show, raw, find-url, path)
- ✅ `corpus extract` - Keyword aggregation across URLs
- ✅ `corpus suggest` - Get query suggestions for a session

**Not Yet Implemented:**
- ⏳ `corpus query` - Boolean filtering (use `fetch --filter` instead)
- ⏳ `corpus compare`, `detect`, `normalize`, `trace`, `score`, `delta`, `summarize`, `explain-failure`

See `docs/CORPUS-API.md` for the full roadmap.

---

## Troubleshooting

**"command not found"**
- Make sure you built the binary: `go build -o llm-web-parser .`
- Run examples from the `examples/` directory: `cd examples && ./01-quick-start.sh`

**"Database already exists"**
- The examples reuse the same database file
- To start fresh: `rm ../llm-web-parser.db ../llm-web-parser-results/ -rf`

**Slow fetches**
- Some URLs may take 5+ seconds to respond
- Use `--workers` flag to adjust concurrency (default: 8)

---

## Next Steps

After running these examples:
1. Read the [main README](../README.md) for complete documentation
2. Check `docs/CORPUS-API.md` for advanced corpus features
3. Try `llm-web-parser --coldstart` for a quick reference guide
