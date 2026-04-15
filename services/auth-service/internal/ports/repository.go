package ports

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
)

type Repository interface {
	Ping(ctx context.Context) error
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUserByID(ctx context.Context, userID string) (domain.User, error)
	ListMemberships(ctx context.Context, userID string) ([]domain.Membership, error)
	ListPageGrants(ctx context.Context, userID, workspaceID, pageID string) ([]domain.PageGrant, error)
	CreateRefreshSession(ctx context.Context, session domain.RefreshSession) error
	GetRefreshSession(ctx context.Context, sessionID string) (domain.RefreshSession, error)
	RevokeRefreshSession(ctx context.Context, sessionID, replacedBy string, revokedAt time.Time) error
}
