package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mtc/wiki-editor-backend/pkg/messaging"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

var ErrNotFound = errors.New("page not found")
var ErrStaleRevision = errors.New("stale revision")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Execute(ctx context.Context, fn func(ports.PageWriter, ports.ProjectionWriter, ports.OutboxWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(pageWriter{queryer: tx}, projectionWriter{queryer: tx}, outboxWriter{queryer: tx}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) GetPage(ctx context.Context, pageID string) (domain.Page, error) {
	const query = `
SELECT id, workspace_id, slug, title, status, created_by, updated_by,
       current_draft_revision_id, current_draft_revision_no,
       COALESCE(current_published_revision_id::text, ''), COALESCE(current_published_revision_no, 0),
       created_at, updated_at
FROM pages
WHERE id = $1
`
	var page domain.Page
	err := s.pool.QueryRow(ctx, query, pageID).Scan(
		&page.ID,
		&page.WorkspaceID,
		&page.Slug,
		&page.Title,
		&page.Status,
		&page.CreatedBy,
		&page.UpdatedBy,
		&page.CurrentDraftRevisionID,
		&page.CurrentDraftRevisionNo,
		&page.CurrentPublishedRevisionID,
		&page.CurrentPublishedRevisionNo,
		&page.CreatedAt,
		&page.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Page{}, ErrNotFound
	}
	return page, err
}

func (s *Store) GetRevision(ctx context.Context, pageID string, view domain.RevisionView) (domain.PageRevision, error) {
	page, err := s.GetPage(ctx, pageID)
	if err != nil {
		return domain.PageRevision{}, err
	}

	revisionID := page.CurrentDraftRevisionID
	if view == domain.RevisionViewPublished && page.CurrentPublishedRevisionID != "" {
		revisionID = page.CurrentPublishedRevisionID
	}

	const query = `
SELECT id, page_id, revision_no, revision_kind, COALESCE(base_revision_id::text, ''), COALESCE(restored_from_revision_id::text, ''),
       document_snapshot, extracted_title, created_by, created_via, created_at
FROM page_revisions
WHERE id = $1
`
	var revision domain.PageRevision
	var documentJSON []byte
	err = s.pool.QueryRow(ctx, query, revisionID).Scan(
		&revision.ID,
		&revision.PageID,
		&revision.RevisionNo,
		&revision.RevisionKind,
		&revision.BaseRevisionID,
		&revision.RestoredFromRevisionID,
		&documentJSON,
		&revision.ExtractedTitle,
		&revision.CreatedBy,
		&revision.CreatedVia,
		&revision.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PageRevision{}, ErrNotFound
	}
	if err != nil {
		return domain.PageRevision{}, err
	}
	if err := json.Unmarshal(documentJSON, &revision.Document); err != nil {
		return domain.PageRevision{}, err
	}
	return revision, nil
}

func (s *Store) GetRevisionByID(ctx context.Context, pageID string, revisionID string) (domain.PageRevision, error) {
	const query = `
SELECT id, page_id, revision_no, revision_kind, COALESCE(base_revision_id::text, ''), COALESCE(restored_from_revision_id::text, ''),
       document_snapshot, extracted_title, created_by, created_via, created_at
FROM page_revisions
WHERE id = $1 AND page_id = $2
`
	var revision domain.PageRevision
	var documentJSON []byte
	err := s.pool.QueryRow(ctx, query, revisionID, pageID).Scan(
		&revision.ID,
		&revision.PageID,
		&revision.RevisionNo,
		&revision.RevisionKind,
		&revision.BaseRevisionID,
		&revision.RestoredFromRevisionID,
		&documentJSON,
		&revision.ExtractedTitle,
		&revision.CreatedBy,
		&revision.CreatedVia,
		&revision.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PageRevision{}, ErrNotFound
	}
	if err != nil {
		return domain.PageRevision{}, err
	}
	if err := json.Unmarshal(documentJSON, &revision.Document); err != nil {
		return domain.PageRevision{}, err
	}
	return revision, nil
}

func (s *Store) ListRevisions(ctx context.Context, pageID string) ([]domain.PageRevision, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, page_id, revision_no, revision_kind, COALESCE(base_revision_id::text, ''), COALESCE(restored_from_revision_id::text, ''),
       document_snapshot, extracted_title, created_by, created_via, created_at
FROM page_revisions
WHERE page_id = $1
ORDER BY revision_no DESC
`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []domain.PageRevision
	for rows.Next() {
		var revision domain.PageRevision
		var documentJSON []byte
		if err := rows.Scan(
			&revision.ID,
			&revision.PageID,
			&revision.RevisionNo,
			&revision.RevisionKind,
			&revision.BaseRevisionID,
			&revision.RestoredFromRevisionID,
			&documentJSON,
			&revision.ExtractedTitle,
			&revision.CreatedBy,
			&revision.CreatedVia,
			&revision.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(documentJSON, &revision.Document); err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

func (s *Store) ClaimPending(ctx context.Context, batchSize int) ([]messaging.OutboxMessage, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, event_type, payload, created_at, available_at
FROM page_outbox
WHERE status = 'pending' AND available_at <= now()
ORDER BY created_at ASC
LIMIT $1
`, batchSize)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var messages []messaging.OutboxMessage
	for rows.Next() {
		var message messaging.OutboxMessage
		if err := rows.Scan(&message.ID, &message.Subject, &message.Payload, &message.OccurredAt, &message.AvailableAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *Store) MarkPublished(ctx context.Context, id string, publishedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE page_outbox SET status = 'published', published_at = $2 WHERE id = $1`, id, publishedAt)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, id string, lastError string, nextAttemptAt time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE page_outbox SET status = 'failed', last_error = $2, available_at = $3 WHERE id = $1`, id, lastError, nextAttemptAt)
	return err
}

func (s *Store) GetDraftIdempotency(ctx context.Context, pageID, idempotencyKey string) (domain.DraftIdempotencyRecord, bool, error) {
	var record domain.DraftIdempotencyRecord
	err := s.pool.QueryRow(ctx, `
SELECT page_id, idempotency_key, revision_id, revision_no, created_at
FROM page_draft_idempotency_keys
WHERE page_id = $1 AND idempotency_key = $2
`, pageID, idempotencyKey).Scan(
		&record.PageID,
		&record.IdempotencyKey,
		&record.RevisionID,
		&record.RevisionNo,
		&record.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DraftIdempotencyRecord{}, false, nil
	}
	return record, err == nil, err
}

type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type pageWriter struct {
	queryer execer
}

func (w pageWriter) CreatePage(ctx context.Context, page domain.Page, revision domain.PageRevision) error {
	documentJSON, err := json.Marshal(revision.Document)
	if err != nil {
		return err
	}

	_, err = w.queryer.Exec(ctx, `
INSERT INTO pages (
	id, workspace_id, slug, title, status, created_by, updated_by,
	current_draft_revision_id, current_draft_revision_no,
	current_published_revision_id, current_published_revision_no,
	created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,NULL,$10,$11)
`,
		page.ID,
		page.WorkspaceID,
		page.Slug,
		page.Title,
		page.Status,
		page.CreatedBy,
		page.UpdatedBy,
		page.CurrentDraftRevisionID,
		page.CurrentDraftRevisionNo,
		page.CreatedAt,
		page.UpdatedAt,
	)
	if err != nil {
		return err
	}

	_, err = w.queryer.Exec(ctx, `
INSERT INTO page_revisions (
	id, page_id, revision_no, revision_kind, base_revision_id,
	document_snapshot, extracted_title, created_by, created_via, created_at
) VALUES ($1,$2,$3,$4,NULL,$5,$6,$7,$8,$9)
`,
		revision.ID,
		revision.PageID,
		revision.RevisionNo,
		revision.RevisionKind,
		documentJSON,
		revision.ExtractedTitle,
		revision.CreatedBy,
		revision.CreatedVia,
		revision.CreatedAt,
	)
	return err
}

func (w pageWriter) SaveDraftRevision(ctx context.Context, expectedBaseRevisionNo int64, page domain.Page, revision domain.PageRevision, idempotency *domain.DraftIdempotencyRecord) error {
	tag, err := w.queryer.Exec(ctx, `
UPDATE pages
SET title = $2, status = $3, updated_by = $4, current_draft_revision_id = $5, current_draft_revision_no = $6, updated_at = $7
WHERE id = $1 AND current_draft_revision_no = $8
`,
		page.ID,
		page.Title,
		page.Status,
		page.UpdatedBy,
		page.CurrentDraftRevisionID,
		page.CurrentDraftRevisionNo,
		page.UpdatedAt,
		expectedBaseRevisionNo,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleRevision
	}

	return insertRevisionAndIdempotency(ctx, w.queryer, revision, idempotency)
}

func (w pageWriter) SavePublishedRevision(ctx context.Context, expectedBaseRevisionNo int64, page domain.Page, revision domain.PageRevision) error {
	tag, err := w.queryer.Exec(ctx, `
UPDATE pages
SET title = $2, status = $3, updated_by = $4, current_published_revision_id = $5, current_published_revision_no = $6, updated_at = $7
WHERE id = $1 AND current_draft_revision_no = $8
`,
		page.ID,
		page.Title,
		page.Status,
		page.UpdatedBy,
		page.CurrentPublishedRevisionID,
		page.CurrentPublishedRevisionNo,
		page.UpdatedAt,
		expectedBaseRevisionNo,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleRevision
	}

	return insertRevisionAndIdempotency(ctx, w.queryer, revision, nil)
}

func (w pageWriter) ArchivePage(ctx context.Context, expectedBaseRevisionNo int64, page domain.Page) error {
	tag, err := w.queryer.Exec(ctx, `
UPDATE pages
SET status = $2, updated_by = $3, updated_at = $4
WHERE id = $1 AND current_draft_revision_no = $5
`,
		page.ID,
		page.Status,
		page.UpdatedBy,
		page.UpdatedAt,
		expectedBaseRevisionNo,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleRevision
	}
	return nil
}

type outboxWriter struct {
	queryer execer
}

func (w outboxWriter) Add(ctx context.Context, records []domain.OutboxRecord) error {
	for _, record := range records {
		if _, err := w.queryer.Exec(ctx, `
INSERT INTO page_outbox (
	id, aggregate_type, aggregate_id, event_type, payload, status, created_at, available_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
`,
			record.ID,
			record.AggregateType,
			record.AggregateID,
			record.EventType,
			record.Payload,
			record.Status,
			record.CreatedAt,
			record.AvailableAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func insertRevisionAndIdempotency(ctx context.Context, queryer execer, revision domain.PageRevision, idempotency *domain.DraftIdempotencyRecord) error {
	documentJSON, err := json.Marshal(revision.Document)
	if err != nil {
		return err
	}

	_, err = queryer.Exec(ctx, `
INSERT INTO page_revisions (
	id, page_id, revision_no, revision_kind, base_revision_id, restored_from_revision_id,
	document_snapshot, extracted_title, created_by, created_via, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
`,
		revision.ID,
		revision.PageID,
		revision.RevisionNo,
		revision.RevisionKind,
		nullableUUID(revision.BaseRevisionID),
		nullableUUID(revision.RestoredFromRevisionID),
		documentJSON,
		revision.ExtractedTitle,
		revision.CreatedBy,
		revision.CreatedVia,
		revision.CreatedAt,
	)
	if err != nil {
		return err
	}

	if idempotency != nil && idempotency.IdempotencyKey != "" {
		_, err = queryer.Exec(ctx, `
INSERT INTO page_draft_idempotency_keys (page_id, idempotency_key, revision_id, revision_no, created_at)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (page_id, idempotency_key) DO NOTHING
`,
			idempotency.PageID,
			idempotency.IdempotencyKey,
			idempotency.RevisionID,
			idempotency.RevisionNo,
			idempotency.CreatedAt,
		)
	}
	return err
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
