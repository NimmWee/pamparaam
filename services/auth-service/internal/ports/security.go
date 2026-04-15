package ports

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenService interface {
	IssueTokens(ctx context.Context, user domain.User) (domain.TokenPair, string, time.Time, error)
	ParseAccessToken(token string) (domain.AccessClaims, error)
	ParseRefreshToken(token string) (domain.RefreshClaims, error)
	JWKS() map[string]any
}
