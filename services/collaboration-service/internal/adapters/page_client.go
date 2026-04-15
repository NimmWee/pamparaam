package adapters

import (
	"context"
	"encoding/json"

	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	pagev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/pagev1"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/ports"
)

type PageClient struct {
	client pagev1.PageRevisionServiceClient
}

func NewPageClient(client pagev1.PageRevisionServiceClient) *PageClient {
	return &PageClient{client: client}
}

func (c *PageClient) GetRevisionHead(ctx context.Context, input ports.PageRevisionHeadInput) (ports.PageRevisionHead, error) {
	response, err := c.client.GetRevisionHead(ctx, &pagev1.GetRevisionHeadRequest{
		Identity: authv1.IdentityContext{
			RequestID:     input.RequestID,
			CorrelationID: input.CorrelationID,
			ActorUserID:   input.ActorUserID,
			WorkspaceID:   input.WorkspaceID,
		},
		PageID: input.PageID,
	})
	if err != nil {
		return ports.PageRevisionHead{}, err
	}

	var document domain.Document
	if err := json.Unmarshal(response.DocumentSnapshotJSON, &document); err != nil {
		return ports.PageRevisionHead{}, err
	}

	return ports.PageRevisionHead{
		PageID:      response.PageID,
		WorkspaceID: response.WorkspaceID,
		RevisionID:  response.CurrentRevisionID,
		RevisionNo:  response.CurrentRevisionNo,
		Document:    document,
	}, nil
}

func (c *PageClient) CommitRevision(ctx context.Context, input ports.CommitRevisionInput) (ports.CommitRevisionResult, error) {
	documentJSON, err := json.Marshal(input.Document)
	if err != nil {
		return ports.CommitRevisionResult{}, err
	}

	ops := make([]pagev1.PatchOperation, 0, len(input.Ops))
	for _, op := range input.Ops {
		ops = append(ops, pagev1.PatchOperation{
			Op:        op.Op,
			BlockID:   op.BlockID,
			ValueJSON: []byte(op.Value),
		})
	}

	response, err := c.client.CommitCollaborativeRevision(ctx, &pagev1.CommitCollaborativeRevisionRequest{
		Identity: authv1.IdentityContext{
			RequestID:     input.RequestID,
			CorrelationID: input.CorrelationID,
			ActorUserID:   input.ActorUserID,
			WorkspaceID:   input.WorkspaceID,
		},
		PageID:                  input.PageID,
		BaseRevisionNo:          input.BaseRevisionNo,
		PatchID:                 input.PatchID,
		AcceptedPatchOps:        ops,
		NewDocumentSnapshotJSON: documentJSON,
	})
	if err != nil {
		return ports.CommitRevisionResult{}, err
	}

	return ports.CommitRevisionResult{
		AcceptedRevisionID: response.AcceptedRevisionID,
		AcceptedRevisionNo: response.AcceptedRevisionNo,
		DocumentHash:       response.DocumentHash,
	}, nil
}

var _ ports.PageRevisionClient = (*PageClient)(nil)
