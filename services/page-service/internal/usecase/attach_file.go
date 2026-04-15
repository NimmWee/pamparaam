package usecase

import (
	"context"
	"fmt"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type attachmentHydrator struct {
	files ports.FileMetadataResolver
}

func newAttachmentHydrator(files ports.FileMetadataResolver) *attachmentHydrator {
	return &attachmentHydrator{files: files}
}

func (h *attachmentHydrator) ValidateAndHydrate(ctx context.Context, document domain.Document, pageID, workspaceID, actorUserID string) (domain.Document, error) {
	if h.files == nil {
		return document, nil
	}
	hydrated := document
	for index, block := range hydrated.Blocks {
		if block.Attachment == nil || block.Attachment.FileID == "" {
			continue
		}
		metadata, err := h.files.GetFileMetadata(ctx, ports.FileMetadataInput{
			FileID:      block.Attachment.FileID,
			PageID:      pageID,
			WorkspaceID: workspaceID,
			ActorUserID: actorUserID,
		})
		if err != nil {
			return domain.Document{}, err
		}
		if !metadata.Exists || metadata.Status != "ready" {
			return domain.Document{}, fmt.Errorf("%w: attachment %s unavailable", ErrValidation, block.Attachment.FileID)
		}
		copy := *block.Attachment
		copy.Filename = metadata.Filename
		hydrated.Blocks[index].Attachment = &copy
	}
	return hydrated, nil
}
