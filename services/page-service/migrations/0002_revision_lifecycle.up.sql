ALTER TABLE page_revisions
    ADD COLUMN IF NOT EXISTS restored_from_revision_id UUID NULL REFERENCES page_revisions(id);

CREATE TABLE IF NOT EXISTS page_draft_idempotency_keys (
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    revision_id UUID NOT NULL REFERENCES page_revisions(id) ON DELETE CASCADE,
    revision_no BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (page_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_page_revisions_restored_from ON page_revisions(restored_from_revision_id);
CREATE INDEX IF NOT EXISTS idx_page_draft_idempotency_revision ON page_draft_idempotency_keys(revision_id);
