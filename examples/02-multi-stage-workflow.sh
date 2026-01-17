#!/bin/bash
# 02-multi-stage-workflow.sh - Realistic multi-stage workflow
#
# What this does:
# 1. Fast scan of multiple URLs (wordcount mode)
# 2. Inspect results to find high-confidence URLs
# 3. Deep-dive on selected URLs with full-parse mode
#
# Expected runtime: 10-15 seconds
# Expected output: Two sessions (scan + deep-dive)

set -e  # Exit on error

echo "=== Multi-Stage Workflow Example ==="
echo ""

# Stage 1: Fast scan with keyword extraction
echo "Stage 1: Fast scan of 3 URLs (wordcount mode)..."
./llm-web-parser fetch --urls "https://golang.org,https://www.python.org,https://www.rust-lang.org"

echo ""
echo "Checking session details..."
SESSION_ID=$(./llm-web-parser db sessions | head -2 | tail -1 | awk '{print $1}' | tr -d '#')

echo "Latest session ID: $SESSION_ID"
echo ""

# Stage 2: Analyze which URLs are high-confidence
echo "Stage 2: Checking confidence scores..."
./llm-web-parser db get --file=details | head -20

echo ""
echo "Stage 3: Full-parse on all URLs from the session..."
./llm-web-parser fetch --session "$SESSION_ID" --features full-parse

echo ""
echo "=== Workflow Complete! ==="
echo "You now have:"
echo "  - Session $SESSION_ID: Fast scan with keywords"
echo "  - Session $((SESSION_ID + 1)): Full parse with content blocks"
echo ""
echo "Try these next:"
echo "  ./llm-web-parser corpus extract --session $SESSION_ID --top 20"
echo "  ./llm-web-parser db get --file=details $((SESSION_ID + 1)) | head -50"
