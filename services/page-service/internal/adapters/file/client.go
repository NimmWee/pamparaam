package file

import (
	"context"

	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	filev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/filev1"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type Client struct {
	client filev1.FileMetadataServiceClient
}

func NewClient(client filev1.FileMetadataServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) GetFileMetadata(ctx context.Context, input ports.FileMetadataInput) (ports.FileMetadata, error) {
	response, err := c.client.GetFileMetadata(ctx, &filev1.GetFileMetadataRequest{
		Identity: authv1.IdentityContext{
			ActorUserID: input.ActorUserID,
			WorkspaceID: input.WorkspaceID,
		},
		FileID: input.FileID,
		PageID: input.PageID,
	})
	if err != nil {
		return ports.FileMetadata{}, err
	}
	return ports.FileMetadata{
		Exists:      response.Exists,
		Status:      response.Status,
		Filename:    response.Filename,
		ContentType: response.ContentType,
		SizeBytes:   response.SizeBytes,
		ObjectKey:   response.ObjectKey,
	}, nil
}
