package db

const schema = `
-- Performance and reliability settings
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA temp_store = MEMORY;
PRAGMA mmap_size = 30000000000;

-- URLs table: normalized URL components + content type metadata
CREATE TABLE IF NOT EXISTS urls (
    url_id INTEGER PRIMARY KEY AUTOINCREMENT,
    original_url TEXT NOT NULL UNIQUE,
    canonical_url TEXT,
    scheme TEXT NOT NULL,
    domain TEXT NOT NULL,
    path TEXT,
    fragment TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Content type classification
    content_type TEXT,           -- academic, docs, wiki, news, repo, blog, landing, unknown
    content_subtype TEXT,         -- arxiv-paper, api-docs, reference, etc.
    detection_confidence REAL,    -- 0-10 confidence score

    -- Boolean flags for content features
    has_abstract BOOLEAN DEFAULT 0,
    has_infobox BOOLEAN DEFAULT 0,
    has_toc BOOLEAN DEFAULT 0,
    has_code_examples BOOLEAN DEFAULT 0,

    -- Content structure counts
    section_count INTEGER DEFAULT 0,
    citation_count INTEGER DEFAULT 0,
    code_block_count INTEGER DEFAULT 0,

    -- Top keywords as JSON object: {"word1": count1, "word2": count2, ...}
    top_keywords TEXT
);

CREATE INDEX IF NOT EXISTS idx_urls_domain ON urls(domain);
CREATE INDEX IF NOT EXISTS idx_urls_canonical ON urls(canonical_url);

-- Content type indexes for fast queries
CREATE INDEX IF NOT EXISTS idx_urls_content_type ON urls(content_type);
CREATE INDEX IF NOT EXISTS idx_urls_has_abstract ON urls(has_abstract) WHERE has_abstract = 1;
CREATE INDEX IF NOT EXISTS idx_urls_has_code ON urls(has_code_examples) WHERE has_code_examples = 1;
CREATE INDEX IF NOT EXISTS idx_urls_confidence ON urls(detection_confidence);

-- URL query parameters: normalized query strings
CREATE TABLE IF NOT EXISTS url_query_params (
    param_id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL,
    key TEXT NOT NULL,
    value TEXT,
    FOREIGN KEY (url_id) REFERENCES urls(url_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_params_url ON url_query_params(url_id);
CREATE INDEX IF NOT EXISTS idx_params_key ON url_query_params(key);

-- URL metadata: key-value storage for URL-specific metadata
CREATE TABLE IF NOT EXISTS url_metadata (
    metadata_id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL,
    namespace TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    FOREIGN KEY (url_id) REFERENCES urls(url_id) ON DELETE CASCADE,
    UNIQUE(url_id, namespace, key)
);

CREATE INDEX IF NOT EXISTS idx_metadata_url ON url_metadata(url_id);
CREATE INDEX IF NOT EXISTS idx_metadata_namespace ON url_metadata(namespace);
CREATE INDEX IF NOT EXISTS idx_metadata_key ON url_metadata(key);

-- URL accesses: every fetch attempt tracked
CREATE TABLE IF NOT EXISTS url_accesses (
    access_id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL,
    accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status_code INTEGER,
    error_type TEXT,
    success BOOLEAN NOT NULL,
    FOREIGN KEY (url_id) REFERENCES urls(url_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_accesses_url ON url_accesses(url_id);
CREATE INDEX IF NOT EXISTS idx_accesses_time ON url_accesses(accessed_at);
CREATE INDEX IF NOT EXISTS idx_accesses_success ON url_accesses(success);

-- Artifact types: lookup table for normalization
CREATE TABLE IF NOT EXISTS artifact_types (
    type_id INTEGER PRIMARY KEY AUTOINCREMENT,
    type_name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Artifacts: content pointers (DB stores metadata, disk stores content)
CREATE TABLE IF NOT EXISTS artifacts (
    artifact_id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL,
    type_id INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    file_path TEXT NOT NULL,
    size_bytes INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (url_id) REFERENCES urls(url_id) ON DELETE CASCADE,
    FOREIGN KEY (type_id) REFERENCES artifact_types(type_id),
    UNIQUE(url_id, type_id)
);

CREATE INDEX IF NOT EXISTS idx_artifacts_url ON artifacts(url_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_type ON artifacts(type_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_hash ON artifacts(content_hash);

-- Artifact metadata: parsing results, per-artifact properties
CREATE TABLE IF NOT EXISTS artifact_metadata (
    metadata_id INTEGER PRIMARY KEY AUTOINCREMENT,
    artifact_id INTEGER NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    FOREIGN KEY (artifact_id) REFERENCES artifacts(artifact_id) ON DELETE CASCADE,
    UNIQUE(artifact_id, key)
);

CREATE INDEX IF NOT EXISTS idx_artifact_metadata_artifact ON artifact_metadata(artifact_id);
CREATE INDEX IF NOT EXISTS idx_artifact_metadata_key ON artifact_metadata(key);

-- URL redirects: redirect chain tracking
CREATE TABLE IF NOT EXISTS url_redirects (
    redirect_id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_url_id INTEGER NOT NULL,
    target_url_id INTEGER NOT NULL,
    redirect_code INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_url_id) REFERENCES urls(url_id) ON DELETE CASCADE,
    FOREIGN KEY (target_url_id) REFERENCES urls(url_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_redirects_source ON url_redirects(source_url_id);
CREATE INDEX IF NOT EXISTS idx_redirects_target ON url_redirects(target_url_id);

-- Sessions: tracks each fetch operation with auto-incrementing ID
CREATE TABLE IF NOT EXISTS sessions (
    session_id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    url_count INTEGER NOT NULL,
    success_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    features TEXT,
    parse_mode TEXT,
    session_dir TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at DESC);

-- Session URLs: junction table mapping sessions to URLs
CREATE TABLE IF NOT EXISTS session_urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    url_id INTEGER NOT NULL,
    was_sanitized BOOLEAN DEFAULT FALSE,
    original_url TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(session_id) ON DELETE CASCADE,
    FOREIGN KEY (url_id) REFERENCES urls(url_id) ON DELETE CASCADE,
    UNIQUE(session_id, url_id)
);

CREATE INDEX IF NOT EXISTS idx_session_urls_session ON session_urls(session_id);
CREATE INDEX IF NOT EXISTS idx_session_urls_url ON session_urls(url_id);
CREATE INDEX IF NOT EXISTS idx_session_urls_sanitized ON session_urls(was_sanitized);

-- Session results: per-URL results within a session
CREATE TABLE IF NOT EXISTS session_results (
    result_id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    url_id INTEGER NOT NULL,
    status TEXT NOT NULL,
    status_code INTEGER,
    error_type TEXT,
    error_message TEXT,
    file_size_bytes INTEGER,
    estimated_tokens INTEGER,
    FOREIGN KEY (session_id) REFERENCES sessions(session_id) ON DELETE CASCADE,
    FOREIGN KEY (url_id) REFERENCES urls(url_id),
    UNIQUE(session_id, url_id)
);

CREATE INDEX IF NOT EXISTS idx_session_results_session ON session_results(session_id);

-- Seed artifact types
INSERT OR IGNORE INTO artifact_types (type_name, description) VALUES
    ('html_raw', 'Raw HTML content'),
    ('json_parsed', 'Parsed JSON output from parser'),
    ('keywords', 'Extracted keywords'),
    ('wordcount', 'Word frequency analysis'),
    ('links', 'Extracted links'),
    ('images', 'Extracted images'),
    ('metadata', 'Page metadata (title, description, etc.)');
`
