# Database Command Reference

## Session Management

```bash
# List all sessions
lwp db sessions
lwp db sessions --limit=20    # DEFAULT

# Show session details
lwp db session              # Latest
lwp db session 5            # Specific session

# Database location
lwp db path
```

---

## Querying

```bash
# Sessions created today
lwp db query --today

# Sessions with failures
lwp db query --failed

# Sessions containing specific URL
lwp db query --url=example.com
```

---

## Session Content

```bash
# Get session YAML
lwp db get --file=details              # Latest, full YAML (DEFAULT)
lwp db get --file=index                # Latest, index only
lwp db get --file=failed               # Latest, failed URLs
lwp db get --file=details 5            # Specific session
```

---

## URL Operations

```bash
# List URL IDs
lwp db urls                 # Latest session
lwp db urls 5               # Specific session
lwp db urls --sanitized     # Only cleaned URLs

# Show parsed content
lwp db show 42                          # By ID
lwp db show https://golang.org          # By URL
lwp db show 42,43,44                    # Batch retrieve

# Show raw HTML
lwp db raw 42
lwp db raw https://golang.org

# Find URL ID
lwp db find-url https://golang.org
# Output: [#42] https://golang.org
```

---

## Workflows

### Session Exploration

```bash
# Step 1: Find interesting session
lwp db sessions

# Step 2: See what's in it
lwp db urls 5

# Step 3: Check keywords
lwp corpus extract --session=5 --top=10

# Step 4: Read relevant URLs
lwp db show 2,5,8
```
