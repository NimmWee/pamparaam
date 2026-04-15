package usecase

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
)

type DeleteFile struct {
	store      ports.Store
	authorizer *FileActionAuthorizer
	now        func() time.Time
}

func NewDeleteFile(store ports.Store, authorizer *FileActionAuthorizer, now func() time.Time) *DeleteFile {
	return &DeleteFile{store: store, authorizer: authorizer, now: now}
}

type DeleteFileInput struct {
	FileID        string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

func (u *DeleteFile) Execute(ctx context.Context, input DeleteFileInput) (domain.FileObjectPayload, error) {
	file, err := u.store.GetFile(ctx, input.FileID)
	if err != nil {
		return domain.FileObjectPayload{}, err
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionFileUpload, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   file.WorkspaceID,
		PageID:        file.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.FileObjectPayload{}, err
	}

	deletedAt := u.now().UTC()
	if err := u.store.SoftDelete(ctx, file.ID, deletedAt); err != nil {
		return domain.FileObjectPayload{}, err
	}
	return domain.FileObjectPayload{
		FileID:      file.ID,
		ObjectKey:   file.ObjectKey,
		Filename:    file.Filename,
		ContentType: file.ContentType,
		SizeBytes:   file.SizeBytes,
		Status:      domain.FileStatusDeleted,
	}, nil
}
