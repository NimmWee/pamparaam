CREATE TABLE IF NOT EXISTS pages (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    current_draft_revision_id UUID NOT NULL,
    current_draft_revision_no BIGINT NOT NULL,
    current_published_revision_id UUID NULL,
    current_published_revision_no BIGINT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (workspace_id, slug)
);

CREATE TABLE IF NOT EXISTS page_revisions (
    id UUID PRIMARY KEY,
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    revision_no BIGINT NOT NULL,
    revision_kind TEXT NOT NULL,
    base_revision_id UUID NULL,
    document_snapshot JSONB NOT NULL,
    extracted_title TEXT NOT NULL,
    created_by UUID NOT NULL,
    created_via TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (page_id, revision_no)
);

CREATE TABLE IF NOT EXISTS embedded_table_refs (
    id UUID PRIMARY KEY,
    page_revision_id UUID NOT NULL REFERENCES page_revisions(id) ON DELETE CASCADE,
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    block_id TEXT NOT NULL,
    mws_table_id TEXT NOT NULL,
    display_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    preview_cache_key TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS attachment_refs (
    id UUID PRIMARY KEY,
    page_revision_id UUID NOT NULL REFERENCES page_revisions(id) ON DELETE CASCADE,
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    block_id TEXT NULL,
    file_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS page_links (
    id UUID PRIMARY KEY,
    page_revision_id UUID NOT NULL REFERENCES page_revisions(id) ON DELETE CASCADE,
    source_page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    target_page_id UUID NOT NULL,
    block_id TEXT NOT NULL,
    link_kind TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS page_outbox (
    id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ NULL,
    available_at TIMESTAMPTZ NOT NULL,
    last_error TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_page_revisions_page_id ON page_revisions(page_id, revision_no DESC);
CREATE INDEX IF NOT EXISTS idx_embedded_table_refs_revision_id ON embedded_table_refs(page_revision_id);
CREATE INDEX IF NOT EXISTS idx_attachment_refs_revision_id ON attachment_refs(page_revision_id);
CREATE INDEX IF NOT EXISTS idx_page_links_revision_id ON page_links(page_revision_id);
CREATE INDEX IF NOT EXISTS idx_page_outbox_status_available ON page_outbox(status, available_at);
