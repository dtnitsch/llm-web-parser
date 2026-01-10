`llm-web-parser` Example Commands for Documentation

. Fetching URLs & Initial Summaries (`fetch` command)

* Fetch a single URL:
./llm-web-parser fetch --urls "https://example.com/some-article"

* Fetch multiple URLs:
./llm-web-parser fetch -u "https://site1.com,https://site2.org/page"

* Fetch and force fresh data (ignore local storage):
./llm-web-parser fetch --urls "https://example.com/news" --force-fetch

* Fetch with a short freshness window (e.g., 5 minutes):
./llm-web-parser fetch --urls "https://example.com/live-blog" --max-age 5m

* Fetch and store results in a custom directory:
./llm-web-parser fetch --urls "https://example.com/data" --output-dir "my_project_results"

* Fetch and output detailed JSON:
./llm-web-parser fetch --urls "https://example.com/details" --output-mode full
(Illustrates future planned capability)

* Fetch and output YAML summaries:
./llm-web-parser fetch --urls "https://example.com/metrics" --format yaml

2. Extracting Filtered Content (`extract` command)

(Assumes you have previously run `fetch` and have `.json` files in `llm-web-parser-results/parsed/`)

* Extract high-confidence content (>= 0.7) from all parsed files:
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy="conf:>=0.7"

* Extract only code blocks from a specific file:
./llm-web-parser extract --from 'llm-web-parser-results/parsed/docs_api_com-a1b2c3d4e5f6.json' --strategy
"type:code"

* Extract paragraphs and list items with medium-high confidence (>= 0.5):
./llm-web-parser extract --from 'llm-web-parser-results/parsed/blog_post-xzy123.json' --strategy=
"conf:>=0.5,type:p|li"

* Extract content from specific sections (by heading text) from multiple files:
./llm-web-parser extract --from 'llm-web-parser-results/parsed/*.json' --strategy=
"section:'Installation|API Reference'"
(Illustrates future planned capability)
