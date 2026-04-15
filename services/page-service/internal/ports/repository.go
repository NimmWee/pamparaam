package ports

import (
	"context"

	"github.com/mtc/wiki-editor-backend/pkg/messaging"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

type PageWriter interface {
	CreatePage(ctx context.Context, page domain.Page, revision domain.PageRevision) error
	SaveDraftRevision(ctx context.Context, expectedBaseRevisionNo int64, page domain.Page, revision domain.PageRevision, idempotency *domain.DraftIdempotencyRecord) error
	SavePublishedRevision(ctx context.Context, expectedBaseRevisionNo int64, page domain.Page, revision domain.PageRevision) error
	ArchivePage(ctx context.Context, expectedBaseRevisionNo int64, page domain.Page) error
}

type ProjectionWriter interface {
	ReplaceEmbeddedTableRefs(ctx context.Context, revisionID string, refs []domain.EmbeddedTableReference) error
	ReplaceAttachmentRefs(ctx context.Context, revisionID string, refs []domain.AttachmentReferenceRecord) error
	ReplacePageLinks(ctx context.Context, revisionID string, refs []domain.PageLinkRecord) error
}

type OutboxWriter interface {
	Add(ctx context.Context, records []domain.OutboxRecord) error
}

type Store interface {
	Execute(ctx context.Context, fn func(PageWriter, ProjectionWriter, OutboxWriter) error) error
	GetPage(ctx context.Context, pageID string) (domain.Page, error)
	GetRevision(ctx context.Context, pageID string, view domain.RevisionView) (domain.PageRevision, error)
	GetRevisionByID(ctx context.Context, pageID string, revisionID string) (domain.PageRevision, error)
	ListRevisions(ctx context.Context, pageID string) ([]domain.PageRevision, error)
	ListEmbeddedTableRefs(ctx context.Context, revisionID string) ([]domain.EmbeddedTableReference, error)
	ListAttachmentRefs(ctx context.Context, revisionID string) ([]domain.AttachmentReferenceRecord, error)
	ListPageLinks(ctx context.Context, revisionID string) ([]domain.PageLinkRecord, error)
	GetDraftIdempotency(ctx context.Context, pageID, idempotencyKey string) (domain.DraftIdempotencyRecord, bool, error)
	Ping(ctx context.Context) error
	messaging.OutboxStore
}
