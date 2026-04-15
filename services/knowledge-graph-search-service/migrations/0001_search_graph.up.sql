CREATE TABLE IF NOT EXISTS search_documents (
    page_id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    title TEXT NOT NULL,
    searchable_text TEXT NOT NULL,
    link_titles TEXT[] NOT NULL DEFAULT '{}',
    embed_titles TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    search_vector tsvector NOT NULL DEFAULT ''::tsvector
);

CREATE OR REPLACE FUNCTION set_search_documents_vector()
RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector(
        'simple',
        coalesce(NEW.title, '') || ' ' ||
        coalesce(NEW.searchable_text, '') || ' ' ||
        array_to_string(NEW.link_titles, ' ') || ' ' ||
        array_to_string(NEW.embed_titles, ' ')
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_set_search_documents_vector ON search_documents;
CREATE TRIGGER trg_set_search_documents_vector
BEFORE INSERT OR UPDATE ON search_documents
FOR EACH ROW
EXECUTE FUNCTION set_search_documents_vector();

CREATE TABLE IF NOT EXISTS page_link_edges (
    source_page_id TEXT NOT NULL,
    target_page_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    PRIMARY KEY (source_page_id, target_page_id)
);

CREATE INDEX IF NOT EXISTS idx_search_documents_workspace_updated_at ON search_documents (workspace_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_search_documents_vector ON search_documents USING GIN (search_vector);
CREATE INDEX IF NOT EXISTS idx_page_link_edges_target ON page_link_edges (workspace_id, target_page_id);
