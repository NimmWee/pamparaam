package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/ports"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) UpsertPage(ctx context.Context, projection domain.PageProjection, targetPageIDs []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
INSERT INTO search_documents (page_id, workspace_id, title, searchable_text, link_titles, embed_titles, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (page_id) DO UPDATE SET
	workspace_id = EXCLUDED.workspace_id,
	title = EXCLUDED.title,
	searchable_text = EXCLUDED.searchable_text,
	link_titles = EXCLUDED.link_titles,
	embed_titles = EXCLUDED.embed_titles,
	updated_at = EXCLUDED.updated_at
`,
		projection.PageID,
		projection.WorkspaceID,
		projection.Title,
		projection.SearchableText,
		projection.LinkTitles,
		projection.EmbedTitles,
		projection.UpdatedAt,
	); err != nil {
		return err
	}

	if err := replacePageLinks(ctx, tx, projection.WorkspaceID, projection.PageID, targetPageIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ReplacePageLinks(ctx context.Context, workspaceID, sourcePageID string, targetPageIDs []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := replacePageLinks(ctx, tx, workspaceID, sourcePageID, targetPageIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replacePageLinks(ctx context.Context, tx pgx.Tx, workspaceID, sourcePageID string, targetPageIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM page_link_edges WHERE workspace_id = $1 AND source_page_id = $2`, workspaceID, sourcePageID); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, targetPageID := range targetPageIDs {
		targetPageID = strings.TrimSpace(targetPageID)
		if targetPageID == "" {
			continue
		}
		if _, ok := seen[targetPageID]; ok {
			continue
		}
		seen[targetPageID] = struct{}{}
		if _, err := tx.Exec(ctx, `
INSERT INTO page_link_edges (source_page_id, target_page_id, workspace_id)
VALUES ($1,$2,$3)
`, sourcePageID, targetPageID, workspaceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Search(ctx context.Context, workspaceID, query, sortKey string) ([]domain.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		rows, err := s.pool.Query(ctx, `
SELECT page_id, title, updated_at
FROM search_documents
WHERE workspace_id = $1
ORDER BY updated_at DESC
LIMIT 50
`, workspaceID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		results := make([]domain.SearchResult, 0)
		for rows.Next() {
			var result domain.SearchResult
			if err := rows.Scan(&result.PageID, &result.Title, &result.UpdatedAt); err != nil {
				return nil, err
			}
			result.MatchType = "content"
			result.Snippet = result.Title
			results = append(results, result)
		}
		return results, rows.Err()
	}

	orderBy := "rank DESC, updated_at DESC"
	if sortKey == "updated_at" {
		orderBy = "updated_at DESC, rank DESC"
	}
	rows, err := s.pool.Query(ctx, `
SELECT page_id, title,
       CASE
           WHEN title ILIKE '%' || $2 || '%' THEN 'title'
           WHEN EXISTS (SELECT 1 FROM unnest(embed_titles) AS embed_title WHERE embed_title ILIKE '%' || $2 || '%') THEN 'embed_reference'
           WHEN EXISTS (SELECT 1 FROM unnest(link_titles) AS link_title WHERE link_title ILIKE '%' || $2 || '%') THEN 'link'
           ELSE 'content'
       END AS match_type,
       COALESCE(NULLIF(ts_headline('simple', searchable_text, plainto_tsquery('simple', $2), 'MaxFragments=2,MaxWords=12,MinWords=4'), ''), title) AS snippet,
       updated_at,
       ts_rank_cd(search_vector, plainto_tsquery('simple', $2)) AS rank
FROM search_documents
WHERE workspace_id = $1
  AND (
      search_vector @@ plainto_tsquery('simple', $2)
      OR title ILIKE '%' || $2 || '%'
      OR EXISTS (SELECT 1 FROM unnest(link_titles) AS link_title WHERE link_title ILIKE '%' || $2 || '%')
      OR EXISTS (SELECT 1 FROM unnest(embed_titles) AS embed_title WHERE embed_title ILIKE '%' || $2 || '%')
  )
ORDER BY `+orderBy+`
LIMIT 50
`, workspaceID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]domain.SearchResult, 0)
	for rows.Next() {
		var result domain.SearchResult
		var rank float64
		if err := rows.Scan(&result.PageID, &result.Title, &result.MatchType, &result.Snippet, &result.UpdatedAt, &rank); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func (s *Store) GetBacklinks(ctx context.Context, workspaceID, pageID string) (domain.BacklinksPayload, error) {
	backlinkRows, err := s.pool.Query(ctx, `
SELECT d.page_id, d.title
FROM page_link_edges e
JOIN search_documents d
  ON d.page_id = e.source_page_id
 AND d.workspace_id = e.workspace_id
WHERE e.workspace_id = $1
  AND e.target_page_id = $2
ORDER BY d.updated_at DESC, d.title ASC
`, workspaceID, pageID)
	if err != nil {
		return domain.BacklinksPayload{}, err
	}
	defer backlinkRows.Close()

	backlinks := make([]domain.BacklinkReference, 0)
	for backlinkRows.Next() {
		var reference domain.BacklinkReference
		if err := backlinkRows.Scan(&reference.PageID, &reference.Title); err != nil {
			return domain.BacklinksPayload{}, err
		}
		reference.Relation = "backlink"
		backlinks = append(backlinks, reference)
	}
	if err := backlinkRows.Err(); err != nil {
		return domain.BacklinksPayload{}, err
	}

	relatedRows, err := s.pool.Query(ctx, `
WITH current_doc AS (
    SELECT page_id, title, searchable_text
    FROM search_documents
    WHERE workspace_id = $1 AND page_id = $2
),
current_outgoing AS (
    SELECT target_page_id
    FROM page_link_edges
    WHERE workspace_id = $1 AND source_page_id = $2
),
candidate_stats AS (
    SELECT d.page_id,
           d.title,
           d.updated_at,
           EXISTS (
               SELECT 1
               FROM page_link_edges out_edge
               WHERE out_edge.workspace_id = $1
                 AND out_edge.source_page_id = $2
                 AND out_edge.target_page_id = d.page_id
           ) AS direct_outgoing,
           EXISTS (
               SELECT 1
               FROM page_link_edges in_edge
               WHERE in_edge.workspace_id = $1
                 AND in_edge.source_page_id = d.page_id
                 AND in_edge.target_page_id = $2
           ) AS direct_incoming,
           (
               SELECT COUNT(*)
               FROM page_link_edges shared_edge
               JOIN current_outgoing co ON co.target_page_id = shared_edge.target_page_id
               WHERE shared_edge.workspace_id = $1
                 AND shared_edge.source_page_id = d.page_id
           ) AS shared_targets,
           COALESCE(
               ts_rank_cd(
                   d.search_vector,
                   plainto_tsquery('simple', (SELECT title || ' ' || searchable_text FROM current_doc))
               ),
               0
           ) AS topical_rank
    FROM search_documents d
    WHERE d.workspace_id = $1
      AND d.page_id <> $2
)
SELECT page_id,
       title,
       CASE
           WHEN direct_outgoing THEN 'linked_page'
           WHEN direct_incoming THEN 'linked_from'
           WHEN shared_targets > 0 THEN 'shared_links'
           ELSE 'topical_similarity'
       END AS relation
FROM candidate_stats
WHERE direct_outgoing
   OR direct_incoming
   OR shared_targets > 0
   OR topical_rank > 0
ORDER BY
    CASE WHEN direct_outgoing OR direct_incoming THEN 1 ELSE 0 END DESC,
    shared_targets DESC,
    topical_rank DESC,
    updated_at DESC
LIMIT 10
`, workspaceID, pageID)
	if err != nil {
		return domain.BacklinksPayload{}, err
	}
	defer relatedRows.Close()

	related := make([]domain.BacklinkReference, 0)
	for relatedRows.Next() {
		var reference domain.BacklinkReference
		if err := relatedRows.Scan(&reference.PageID, &reference.Title, &reference.Relation); err != nil {
			return domain.BacklinksPayload{}, err
		}
		related = append(related, reference)
	}
	if err := relatedRows.Err(); err != nil {
		return domain.BacklinksPayload{}, err
	}

	return domain.BacklinksPayload{
		PageID:       pageID,
		Backlinks:    backlinks,
		RelatedPages: related,
	}, nil
}

var _ ports.Store = (*Store)(nil)
