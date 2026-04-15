DROP TABLE IF EXISTS page_draft_idempotency_keys;

ALTER TABLE page_revisions
    DROP COLUMN IF EXISTS restored_from_revision_id;
