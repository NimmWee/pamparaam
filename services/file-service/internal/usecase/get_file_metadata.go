package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
)

type GetFileMetadata struct {
	store      ports.Store
	authorizer *FileActionAuthorizer
}

func NewGetFileMetadata(store ports.Store, authorizer *FileActionAuthorizer) *GetFileMetadata {
	return &GetFileMetadata{store: store, authorizer: authorizer}
}

type GetFileMetadataInput struct {
	FileID        string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

func (u *GetFileMetadata) Execute(ctx context.Context, input GetFileMetadataInput) (domain.FileObject, bool, error) {
	file, err := u.store.GetFile(ctx, input.FileID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.FileObject{}, false, nil
	}
	if err != nil {
		return domain.FileObject{}, false, err
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionFileRead, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   file.WorkspaceID,
		PageID:        file.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.FileObject{}, false, err
	}
	return file, true, nil
}

func DefaultNow() func() time.Time {
	return time.Now
}
