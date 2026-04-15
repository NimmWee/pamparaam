package usecase

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
)

type RefreshResult struct {
	Tokens      domain.TokenPair
	User        domain.User
	Memberships []domain.Membership
}

func (a *Authenticator) Refresh(ctx context.Context, rawToken string) (RefreshResult, error) {
	refreshClaims, err := a.tokenService.ParseRefreshToken(rawToken)
	if err != nil {
		return RefreshResult{}, ErrInvalidRefreshToken
	}

	session, err := a.repository.GetRefreshSession(ctx, refreshClaims.SessionID)
	if err != nil {
		return RefreshResult{}, ErrInvalidRefreshToken
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		return RefreshResult{}, ErrInvalidRefreshToken
	}

	user, err := a.repository.GetUserByID(ctx, session.UserID)
	if err != nil {
		return RefreshResult{}, err
	}

	tokens, newSessionID, refreshExpiresAt, err := a.tokenService.IssueTokens(ctx, user)
	if err != nil {
		return RefreshResult{}, err
	}

	if err := a.repository.RevokeRefreshSession(ctx, session.ID, newSessionID, time.Now().UTC()); err != nil {
		return RefreshResult{}, err
	}
	if err := a.repository.CreateRefreshSession(ctx, domain.RefreshSession{
		ID:        newSessionID,
		UserID:    user.ID,
		ExpiresAt: refreshExpiresAt,
	}); err != nil {
		return RefreshResult{}, err
	}

	memberships, err := a.repository.ListMemberships(ctx, user.ID)
	if err != nil {
		return RefreshResult{}, err
	}

	return RefreshResult{
		Tokens:      tokens,
		User:        user,
		Memberships: memberships,
	}, nil
}
