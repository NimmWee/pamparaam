package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

type projectionWriter struct {
	queryer interface {
		Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	}
}

func (w projectionWriter) ReplaceEmbeddedTableRefs(ctx context.Context, revisionID string, refs []domain.EmbeddedTableReference) error {
	if _, err := w.queryer.Exec(ctx, `DELETE FROM embedded_table_refs WHERE page_revision_id = $1`, revisionID); err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := w.queryer.Exec(ctx, `
INSERT INTO embedded_table_refs (id, page_revision_id, page_id, block_id, mws_table_id, display_config, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
`, ref.ID, ref.PageRevisionID, ref.PageID, ref.BlockID, ref.MWSTableID, ref.DisplayConfig, ref.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (w projectionWriter) ReplaceAttachmentRefs(ctx context.Context, revisionID string, refs []domain.AttachmentReferenceRecord) error {
	if _, err := w.queryer.Exec(ctx, `DELETE FROM attachment_refs WHERE page_revision_id = $1`, revisionID); err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := w.queryer.Exec(ctx, `
INSERT INTO attachment_refs (id, page_revision_id, page_id, block_id, file_id, created_at)
VALUES ($1,$2,$3,$4,$5,$6)
`, ref.ID, ref.PageRevisionID, ref.PageID, ref.BlockID, ref.FileID, ref.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (w projectionWriter) ReplacePageLinks(ctx context.Context, revisionID string, refs []domain.PageLinkRecord) error {
	if _, err := w.queryer.Exec(ctx, `DELETE FROM page_links WHERE page_revision_id = $1`, revisionID); err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := w.queryer.Exec(ctx, `
INSERT INTO page_links (id, page_revision_id, source_page_id, target_page_id, block_id, link_kind, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
`, ref.ID, ref.PageRevisionID, ref.SourcePageID, ref.TargetPageID, ref.BlockID, ref.LinkKind, ref.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListEmbeddedTableRefs(ctx context.Context, revisionID string) ([]domain.EmbeddedTableReference, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, page_revision_id, page_id, block_id, mws_table_id, display_config, created_at
FROM embedded_table_refs
WHERE page_revision_id = $1
ORDER BY created_at ASC
`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []domain.EmbeddedTableReference
	for rows.Next() {
		var ref domain.EmbeddedTableReference
		if err := rows.Scan(&ref.ID, &ref.PageRevisionID, &ref.PageID, &ref.BlockID, &ref.MWSTableID, &ref.DisplayConfig, &ref.CreatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (s *Store) ListAttachmentRefs(ctx context.Context, revisionID string) ([]domain.AttachmentReferenceRecord, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, page_revision_id, page_id, block_id, file_id, created_at
FROM attachment_refs
WHERE page_revision_id = $1
ORDER BY created_at ASC
`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []domain.AttachmentReferenceRecord
	for rows.Next() {
		var ref domain.AttachmentReferenceRecord
		if err := rows.Scan(&ref.ID, &ref.PageRevisionID, &ref.PageID, &ref.BlockID, &ref.FileID, &ref.CreatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (s *Store) ListPageLinks(ctx context.Context, revisionID string) ([]domain.PageLinkRecord, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, page_revision_id, source_page_id, target_page_id, block_id, link_kind, created_at
FROM page_links
WHERE page_revision_id = $1
ORDER BY created_at ASC
`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []domain.PageLinkRecord
	for rows.Next() {
		var ref domain.PageLinkRecord
		if err := rows.Scan(&ref.ID, &ref.PageRevisionID, &ref.SourcePageID, &ref.TargetPageID, &ref.BlockID, &ref.LinkKind, &ref.CreatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}
