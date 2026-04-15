package usecase

import (
	"context"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

type GetEditorMetadataInput struct {
	WorkspaceID   string
	PageID        string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

type GetEditorMetadata struct {
	authorizer *PageActionAuthorizer
	runtime    EditorRuntimeCapabilities
}

type EditorRuntimeCapabilities struct {
	SupportsRealtimeCollaboration bool
	SupportsSyncResumeReplay      bool
	SupportsFilesIntegration      bool
	SupportsEmbedIntegration      bool
}

func NewGetEditorMetadata(authorizer *PageActionAuthorizer, runtime EditorRuntimeCapabilities) *GetEditorMetadata {
	return &GetEditorMetadata{authorizer: authorizer, runtime: runtime}
}

func (u *GetEditorMetadata) Execute(ctx context.Context, input GetEditorMetadataInput) (domain.EditorMetadataPayload, error) {
	subject := AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionPageView, subject); err != nil {
		return domain.EditorMetadataPayload{}, err
	}

	canEdit, err := u.authorizer.Allowed(ctx, authz.ActionPageEdit, subject)
	if err != nil {
		return domain.EditorMetadataPayload{}, err
	}
	canUploadFiles, err := u.authorizer.Allowed(ctx, authz.ActionFileUpload, subject)
	if err != nil {
		return domain.EditorMetadataPayload{}, err
	}
	canEmbedTables, err := u.authorizer.Allowed(ctx, authz.ActionPageEmbedTable, subject)
	if err != nil {
		return domain.EditorMetadataPayload{}, err
	}
	canPublish, err := u.authorizer.Allowed(ctx, authz.ActionPagePublish, subject)
	if err != nil {
		return domain.EditorMetadataPayload{}, err
	}
	canRestore, err := u.authorizer.Allowed(ctx, authz.ActionPageRestore, subject)
	if err != nil {
		return domain.EditorMetadataPayload{}, err
	}

	return domain.BuildEditorMetadata(domain.EditorCatalogOptions{
		CanEdit:                  canEdit,
		CanUploadFiles:           canUploadFiles,
		CanEmbedTables:           canEmbedTables,
		CanCollaborate:           canEdit,
		CanPublish:               canPublish,
		CanRestore:               canRestore,
		SupportsRealtimeCollab:   u.runtime.SupportsRealtimeCollaboration,
		SupportsSyncResumeReplay: u.runtime.SupportsSyncResumeReplay,
		SupportsFilesIntegration: u.runtime.SupportsFilesIntegration,
		SupportsEmbedIntegration: u.runtime.SupportsEmbedIntegration,
	}), nil
}
