package help

const ColdstartYAML = `# llm-web-parser Quick Start

parse_modes:
  minimal: "Metadata only (default, fastest)"
  full-parse: "Complete content with confidence scores"

output_modes:
  tier2: "Session-based, best for 10+ URLs (default)"
  summary: "JSON/YAML to stdout"
  full: "Complete parsed content to stdout"

commands:
  basic_fetch: |
    llm-web-parser fetch --urls "https://example.com"

  full_parse: |
    llm-web-parser fetch --urls "https://example.com" --features full-parse

  inline_filter: |
    llm-web-parser fetch --urls "https://example.com" --features full-parse --filter "conf:>=0.7"

  list_sessions: |
    llm-web-parser db sessions

  session_details: |
    llm-web-parser db session 5

  get_session_content: |
    llm-web-parser db get --file=details 5

  query_sessions: |
    llm-web-parser db query --today
    llm-web-parser db query --failed
    llm-web-parser db query --url=example.com

  multi_stage: |
    # Step 1: Fast minimal fetch
    llm-web-parser fetch --urls "url1,url2,url3"

    # Step 2: List sessions and get latest ID
    llm-web-parser db sessions

    # Step 3: Get session content
    llm-web-parser db get --file=details <session_id>

    # Step 4: Full parse selected URLs
    llm-web-parser fetch --urls "$(yq -r '.[] | select(.confidence >= 7) | .url' llm-web-parser-results/sessions/2026-01-15-<id>/summary-details.yaml)" --features full-parse

key_files:
  - "llm-web-parser-results/FIELDS.yaml (field reference)"
  - "llm-web-parser-results/index.yaml (all sessions)"
  - "llm-web-parser-results/sessions/2026-01-15-{id}/summary-details.yaml (full metadata)"

session_system:
  - "Sessions tracked in SQLite database"
  - "Auto-incrementing session IDs (1, 2, 3...)"
  - "Session directories: sessions/2026-01-15-1 (date + ID)"
  - "Same URLs = instant cache hit from DB"
  - "Use 'lwp db sessions' to list all sessions"
  - "Use 'lwp db session <id>' for details"
  - "Use 'lwp db get --file=details <id>' to see YAML content"

db_commands:
  sessions: "List all sessions with stats"
  session_id: "Show detailed info for session"
  get_id: "Cat session YAML files (--file=index|details|failed)"
  query: "Filter sessions (--today, --failed, --url=pattern)"
  init: "Initialize database schema"

session_invariants:
  - "Same URLs = same session ID = instant cache hit"
  - "Session dirs: YYYY-MM-DD-{id} for chronological order"
  - "failed-urls.yaml only created if errors occurred"
  - "summary-index.yaml has successful fetches only"
  - "summary-details.yaml has all URLs (success + failed)"

query_examples:
  list_all_sessions: 'lwp db sessions'
  show_session_5: 'lwp db session 5'
  get_details_yaml: 'lwp db get --file=details 5'
  get_index_yaml: 'lwp db get --file=index 5'
  query_today: 'lwp db query --today'
  query_failed: 'lwp db query --failed'
  query_url_pattern: 'lwp db query --url=example.com'
  filter_confidence: 'lwp db get --file=details 5 | yq ".[] | select(.confidence >= 7)"'
  filter_domain: 'lwp db get --file=details 5 | yq ".[] | select(.domain_type == \"academic\")"'

error_behavior:
  - "Malformed URLs: fail fast before fetching"
  - "Runtime errors: logged to failed-urls.yaml"
  - "Exit codes: 0=success, 1=partial failure, 2=complete failure"
`
