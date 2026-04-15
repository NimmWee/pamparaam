package usecase

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
)

type GetFile struct {
	store       ports.Store
	objectStore ports.ObjectStore
	authorizer  *FileActionAuthorizer
}

func NewGetFile(store ports.Store, objectStore ports.ObjectStore, authorizer *FileActionAuthorizer) *GetFile {
	return &GetFile{store: store, objectStore: objectStore, authorizer: authorizer}
}

type GetFileInput struct {
	FileID        string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

func (u *GetFile) Execute(ctx context.Context, input GetFileInput) (domain.FileObjectPayload, error) {
	file, err := u.store.GetFile(ctx, input.FileID)
	if err != nil {
		return domain.FileObjectPayload{}, err
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionFileRead, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   file.WorkspaceID,
		PageID:        file.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.FileObjectPayload{}, err
	}

	downloadURL, err := u.objectStore.PresignDownload(ctx, file.ObjectKey, 15*time.Minute)
	if err != nil {
		return domain.FileObjectPayload{}, err
	}
	return domain.FileObjectPayload{
		FileID:      file.ID,
		ObjectKey:   file.ObjectKey,
		Filename:    file.Filename,
		ContentType: file.ContentType,
		SizeBytes:   file.SizeBytes,
		Status:      file.Status,
		DownloadURL: downloadURL,
	}, nil
}
