package usecase

import (
	"context"

	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/ports"
)

type CurrentUserReader struct {
	repository ports.Repository
}

func NewCurrentUserReader(repository ports.Repository) *CurrentUserReader {
	return &CurrentUserReader{repository: repository}
}

func (r *CurrentUserReader) Execute(ctx context.Context, userID string) (domain.User, []domain.Membership, error) {
	user, err := r.repository.GetUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, nil, err
	}

	memberships, err := r.repository.ListMemberships(ctx, userID)
	if err != nil {
		return domain.User{}, nil, err
	}

	return user, memberships, nil
}
