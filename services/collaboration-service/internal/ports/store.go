package ports

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/domain"
)

type RoomStateStore interface {
	Get(ctx context.Context, roomID string) (domain.RoomState, bool, error)
	Save(ctx context.Context, state domain.RoomState, ttl time.Duration) error
	Delete(ctx context.Context, roomID string) error
}

type PresenceStore interface {
	Upsert(ctx context.Context, member domain.PresenceMember, ttl time.Duration) error
	Touch(ctx context.Context, roomID, sessionID string, cursor *domain.Cursor, selection *domain.Selection, seenAt time.Time, ttl time.Duration) (domain.PresenceMember, error)
	Remove(ctx context.Context, roomID, sessionID string) error
	List(ctx context.Context, roomID string) ([]domain.PresenceMember, error)
}

type PageRevisionClient interface {
	GetRevisionHead(ctx context.Context, input PageRevisionHeadInput) (PageRevisionHead, error)
	CommitRevision(ctx context.Context, input CommitRevisionInput) (CommitRevisionResult, error)
}

type PageRevisionHeadInput struct {
	PageID        string
	RequestID     string
	CorrelationID string
	ActorUserID   string
	WorkspaceID   string
}

type PageRevisionHead struct {
	PageID      string
	WorkspaceID string
	RevisionID  string
	RevisionNo  int64
	Document    domain.Document
}

type CommitRevisionInput struct {
	PageID         string
	BaseRevisionNo int64
	PatchID        string
	Ops            []domain.PatchOperation
	Document       domain.Document
	RequestID      string
	CorrelationID  string
	ActorUserID    string
	WorkspaceID    string
}

type CommitRevisionResult struct {
	AcceptedRevisionID string
	AcceptedRevisionNo int64
	DocumentHash       string
}
