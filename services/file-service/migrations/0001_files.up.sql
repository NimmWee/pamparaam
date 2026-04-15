CREATE TABLE IF NOT EXISTS file_objects (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    page_id TEXT,
    object_key TEXT NOT NULL UNIQUE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum TEXT,
    status TEXT NOT NULL,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS upload_sessions (
    id TEXT PRIMARY KEY,
    file_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    page_id TEXT,
    object_key TEXT NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_file_objects_workspace_updated_at ON file_objects (workspace_id, updated_at DESC);
