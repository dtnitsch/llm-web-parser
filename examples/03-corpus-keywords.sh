#!/bin/bash
# 03-corpus-keywords.sh - Using corpus extract for keyword analysis
#
# What this does:
# 1. Fetches URLs with wordcount mode
# 2. Extracts and aggregates keywords across all URLs
# 3. Shows top keywords to understand corpus themes
#
# Expected runtime: 10-15 seconds
# Expected output: Aggregated keyword frequencies in JSON

set -e  # Exit on error

echo "=== Corpus Keyword Analysis Example ==="
echo ""

# Step 1: Fetch URLs with keyword extraction
echo "Step 1: Fetching 4 tech documentation sites..."
./llm-web-parser fetch --urls "https://go.dev/doc/,https://docs.python.org/3/,https://doc.rust-lang.org/book/,https://nodejs.org/docs"

echo ""
SESSION_ID=$(./llm-web-parser db sessions | head -2 | tail -1 | awk '{print $1}' | tr -d '#')
echo "Session created: #$SESSION_ID"

# Step 2: Extract keywords across the corpus
echo ""
echo "Step 2: Extracting top 15 keywords across all URLs..."
./llm-web-parser corpus extract --session "$SESSION_ID" --top 15 --format json

echo ""
echo ""
echo "=== Analysis Complete! ==="
echo ""
echo "The keywords show common themes across these documentation sites."
echo ""
echo "Try these next:"
echo "  ./llm-web-parser corpus extract --session $SESSION_ID --top 50  # More keywords"
echo "  ./llm-web-parser corpus suggest --session $SESSION_ID           # Get suggestions"
