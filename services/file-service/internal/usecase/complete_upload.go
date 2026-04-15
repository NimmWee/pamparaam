package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
)

type CompleteUploadInput struct {
	UploadID      string
	PageID        string
	Checksum      string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

type CompleteUpload struct {
	store       ports.Store
	objectStore ports.ObjectStore
	authorizer  *FileActionAuthorizer
	now         func() time.Time
}

func NewCompleteUpload(store ports.Store, objectStore ports.ObjectStore, authorizer *FileActionAuthorizer, now func() time.Time) *CompleteUpload {
	return &CompleteUpload{store: store, objectStore: objectStore, authorizer: authorizer, now: now}
}

func (u *CompleteUpload) Execute(ctx context.Context, input CompleteUploadInput) (domain.FileObjectPayload, error) {
	session, err := u.store.GetUploadSession(ctx, input.UploadID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.FileObjectPayload{}, err
	}
	if err != nil {
		return domain.FileObjectPayload{}, err
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionFileUpload, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   session.WorkspaceID,
		PageID:        session.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.FileObjectPayload{}, err
	}
	if session.PageID != "" && input.PageID != "" && session.PageID != input.PageID {
		return domain.FileObjectPayload{}, fmt.Errorf("validation error")
	}
	if session.Checksum != "" && input.Checksum != "" && session.Checksum != input.Checksum {
		return domain.FileObjectPayload{}, fmt.Errorf("validation error")
	}

	file, err := u.store.GetFile(ctx, session.FileID)
	if err != nil {
		return domain.FileObjectPayload{}, err
	}
	now := u.now().UTC()
	file.Status = domain.FileStatusReady
	file.UpdatedAt = now
	if err := u.store.CompleteUpload(ctx, input.UploadID, file); err != nil {
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
