package ports

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
)

type Store interface {
	CreateUploadSession(ctx context.Context, session domain.UploadSession, file domain.FileObject) error
	GetUploadSession(ctx context.Context, uploadID string) (domain.UploadSession, error)
	CompleteUpload(ctx context.Context, uploadID string, file domain.FileObject) error
	GetFile(ctx context.Context, fileID string) (domain.FileObject, error)
	SoftDelete(ctx context.Context, fileID string, deletedAt time.Time) error
	Ping(ctx context.Context) error
}

type ObjectStore interface {
	PresignUpload(ctx context.Context, objectKey string, expiresIn time.Duration) (string, map[string]string, error)
	PresignDownload(ctx context.Context, objectKey string, expiresIn time.Duration) (string, error)
}
