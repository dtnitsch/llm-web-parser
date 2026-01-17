#!/bin/bash
# 01-quick-start.sh - Simplest possible llm-web-parser example
#
# What this does:
# 1. Fetches a single URL with default settings (wordcount mode)
# 2. Shows the database path
# 3. Lists sessions
#
# Expected runtime: 2-5 seconds
# Expected output: Session summary + keywords extracted

set -e  # Exit on error

echo "=== Quick Start Example ==="
echo ""

# 1. Fetch a single URL (wordcount mode is default)
echo "Step 1: Fetching golang.org with keyword extraction..."
./llm-web-parser fetch --urls "https://golang.org"

echo ""
echo "Step 2: Check database location..."
./llm-web-parser db path

echo ""
echo "Step 3: List sessions..."
./llm-web-parser db sessions

echo ""
echo "=== Success! ==="
echo "Try these next:"
echo "  ./llm-web-parser db get --file=details    # See full metadata"
echo "  ./llm-web-parser corpus extract --session 1 --top 10  # Extract top keywords"
