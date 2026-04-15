package mws

import (
	"context"
	"encoding/json"

	mwsv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/mwsv1"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type Client struct {
	client mwsv1.MWSIntegrationServiceClient
}

func NewClient(client mwsv1.MWSIntegrationServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) Resolve(ctx context.Context, input ports.ResolveEmbedInput) (domain.TableEmbedDescriptor, error) {
	displayConfigJSON, err := json.Marshal(input.Reference.DisplayConfig)
	if err != nil {
		return domain.TableEmbedDescriptor{}, err
	}

	response, err := c.client.ResolveEmbed(ctx, &mwsv1.ResolveEmbedRequest{
		Identity: mwsv1.IdentityContext{
			RequestID:     transport.RequestIDFromContext(ctx),
			CorrelationID: transport.CorrelationIDFromContext(ctx),
			ActorUserID:   input.ActorUserID,
			WorkspaceID:   input.WorkspaceID,
			AccessToken:   input.AccessToken,
		},
		PageID:            input.PageID,
		MwsTableID:        input.Reference.MWSTableID,
		DisplayConfigJSON: displayConfigJSON,
		AllowDegraded:     input.AllowDegraded,
		StoredTitle:       input.StoredTitle,
	})
	if err != nil {
		return domain.TableEmbedDescriptor{}, err
	}

	var schema map[string]any
	if len(response.SchemaJSON) > 0 {
		if err := json.Unmarshal(response.SchemaJSON, &schema); err != nil {
			return domain.TableEmbedDescriptor{}, err
		}
	}
	var previewRows []map[string]any
	if len(response.PreviewRowsJSON) > 0 {
		if err := json.Unmarshal(response.PreviewRowsJSON, &previewRows); err != nil {
			return domain.TableEmbedDescriptor{}, err
		}
	}

	return domain.TableEmbedDescriptor{
		MWSTableID:    input.Reference.MWSTableID,
		Title:         response.Title,
		DisplayConfig: input.Reference.DisplayConfig,
		PreviewState:  domain.PreviewState(response.PreviewState),
		Schema:        schema,
		PreviewRows:   previewRows,
	}, nil
}
