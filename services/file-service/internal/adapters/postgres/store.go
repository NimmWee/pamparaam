package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
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

func (s *Store) CreateUploadSession(ctx context.Context, session domain.UploadSession, file domain.FileObject) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
INSERT INTO file_objects (
	id, workspace_id, page_id, object_key, filename, content_type, size_bytes,
	checksum, status, created_by, created_at, updated_at, deleted_at
) VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,NULLIF($10,''),$11,$12,NULL)
`,
		file.ID,
		file.WorkspaceID,
		file.PageID,
		file.ObjectKey,
		file.Filename,
		file.ContentType,
		file.SizeBytes,
		file.Checksum,
		file.Status,
		file.CreatedBy,
		file.CreatedAt,
		file.UpdatedAt,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO upload_sessions (
	id, file_id, workspace_id, page_id, object_key, filename, content_type,
	size_bytes, checksum, expires_at, completed_at
) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10,NULL)
`,
		session.ID,
		session.FileID,
		session.WorkspaceID,
		session.PageID,
		session.ObjectKey,
		session.Filename,
		session.ContentType,
		session.SizeBytes,
		session.Checksum,
		session.ExpiresAt,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) GetUploadSession(ctx context.Context, uploadID string) (domain.UploadSession, error) {
	var session domain.UploadSession
	err := s.pool.QueryRow(ctx, `
SELECT id, file_id, workspace_id, COALESCE(page_id, ''), object_key, filename,
       content_type, size_bytes, COALESCE(checksum, ''), expires_at, completed_at
FROM upload_sessions
WHERE id = $1
`, uploadID).Scan(
		&session.ID,
		&session.FileID,
		&session.WorkspaceID,
		&session.PageID,
		&session.ObjectKey,
		&session.Filename,
		&session.ContentType,
		&session.SizeBytes,
		&session.Checksum,
		&session.ExpiresAt,
		&session.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.UploadSession{}, domain.ErrNotFound
	}
	return session, err
}

func (s *Store) CompleteUpload(ctx context.Context, uploadID string, file domain.FileObject) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
UPDATE upload_sessions
SET completed_at = $2
WHERE id = $1
`, uploadID, file.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	tag, err = tx.Exec(ctx, `
UPDATE file_objects
SET page_id = NULLIF($2,''), checksum = NULLIF($3,''), status = $4, updated_at = $5
WHERE id = $1 AND deleted_at IS NULL
`,
		file.ID,
		file.PageID,
		file.Checksum,
		file.Status,
		file.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return tx.Commit(ctx)
}

func (s *Store) GetFile(ctx context.Context, fileID string) (domain.FileObject, error) {
	var file domain.FileObject
	err := s.pool.QueryRow(ctx, `
SELECT id, workspace_id, COALESCE(page_id, ''), object_key, filename, content_type,
       size_bytes, COALESCE(checksum, ''), status, COALESCE(created_by, ''),
       created_at, updated_at, deleted_at
FROM file_objects
WHERE id = $1 AND deleted_at IS NULL
`, fileID).Scan(
		&file.ID,
		&file.WorkspaceID,
		&file.PageID,
		&file.ObjectKey,
		&file.Filename,
		&file.ContentType,
		&file.SizeBytes,
		&file.Checksum,
		&file.Status,
		&file.CreatedBy,
		&file.CreatedAt,
		&file.UpdatedAt,
		&file.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.FileObject{}, domain.ErrNotFound
	}
	return file, err
}

func (s *Store) SoftDelete(ctx context.Context, fileID string, deletedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE file_objects
SET status = $2, deleted_at = $3, updated_at = $3
WHERE id = $1 AND deleted_at IS NULL
`, fileID, domain.FileStatusDeleted, deletedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ ports.Store = (*Store)(nil)
