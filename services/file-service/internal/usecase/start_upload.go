package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
)

type StartUploadInput struct {
	WorkspaceID   string
	PageID        string
	Filename      string
	ContentType   string
	SizeBytes     int64
	Checksum      string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

type StartUpload struct {
	store       ports.Store
	objectStore ports.ObjectStore
	authorizer  *FileActionAuthorizer
	now         func() time.Time
	nextID      func() string
}

func NewStartUpload(store ports.Store, objectStore ports.ObjectStore, authorizer *FileActionAuthorizer, now func() time.Time, nextID func() string) *StartUpload {
	return &StartUpload{store: store, objectStore: objectStore, authorizer: authorizer, now: now, nextID: nextID}
}

func (u *StartUpload) Execute(ctx context.Context, input StartUploadInput) (domain.UploadSessionPayload, error) {
	if err := u.authorizer.Authorize(ctx, authz.ActionFileUpload, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.UploadSessionPayload{}, err
	}
	if strings.TrimSpace(input.WorkspaceID) == "" || strings.TrimSpace(input.Filename) == "" || input.SizeBytes <= 0 || input.SizeBytes > 25*1024*1024 {
		return domain.UploadSessionPayload{}, fmt.Errorf("validation error")
	}

	now := u.now().UTC()
	fileID := u.nextID()
	uploadID := u.nextID()
	objectKey := filepath.ToSlash(filepath.Join(input.WorkspaceID, fileID, filepath.Base(input.Filename)))
	uploadURL, headers, err := u.objectStore.PresignUpload(ctx, objectKey, 15*time.Minute)
	if err != nil {
		return domain.UploadSessionPayload{}, err
	}

	session := domain.UploadSession{
		ID:          uploadID,
		FileID:      fileID,
		WorkspaceID: input.WorkspaceID,
		PageID:      input.PageID,
		ObjectKey:   objectKey,
		Filename:    filepath.Base(input.Filename),
		ContentType: input.ContentType,
		SizeBytes:   input.SizeBytes,
		Checksum:    input.Checksum,
		ExpiresAt:   now.Add(15 * time.Minute),
	}
	file := domain.FileObject{
		ID:          fileID,
		WorkspaceID: input.WorkspaceID,
		PageID:      input.PageID,
		ObjectKey:   objectKey,
		Filename:    filepath.Base(input.Filename),
		ContentType: input.ContentType,
		SizeBytes:   input.SizeBytes,
		Checksum:    input.Checksum,
		Status:      domain.FileStatusUploading,
		CreatedBy:   input.ActorUserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := u.store.CreateUploadSession(ctx, session, file); err != nil {
		return domain.UploadSessionPayload{}, err
	}

	return domain.UploadSessionPayload{
		UploadID:  uploadID,
		FileID:    fileID,
		ObjectKey: objectKey,
		UploadURL: uploadURL,
		ExpiresAt: session.ExpiresAt,
		Headers:   headers,
	}, nil
}
