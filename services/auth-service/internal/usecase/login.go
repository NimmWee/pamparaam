package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/ports"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	Tokens      domain.TokenPair
	User        domain.User
	Memberships []domain.Membership
}

type Authenticator struct {
	repository     ports.Repository
	passwordHasher ports.PasswordHasher
	tokenService   ports.TokenService
}

func NewAuthenticator(repository ports.Repository, passwordHasher ports.PasswordHasher, tokenService ports.TokenService) *Authenticator {
	return &Authenticator{
		repository:     repository,
		passwordHasher: passwordHasher,
		tokenService:   tokenService,
	}
}

func (a *Authenticator) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	user, err := a.repository.GetUserByEmail(ctx, strings.TrimSpace(strings.ToLower(input.Email)))
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	if err := a.passwordHasher.Compare(user.PasswordHash, input.Password); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	tokens, sessionID, refreshExpiresAt, err := a.tokenService.IssueTokens(ctx, user)
	if err != nil {
		return LoginResult{}, err
	}

	if err := a.repository.CreateRefreshSession(ctx, domain.RefreshSession{
		ID:        sessionID,
		UserID:    user.ID,
		ExpiresAt: refreshExpiresAt,
	}); err != nil {
		return LoginResult{}, err
	}

	memberships, err := a.repository.ListMemberships(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Tokens:      tokens,
		User:        user,
		Memberships: memberships,
	}, nil
}
