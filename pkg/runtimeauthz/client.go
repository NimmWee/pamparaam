package runtimeauthz

import (
	"context"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
)

type CheckInput struct {
	ActorUserID string
	WorkspaceID string
	PageID      string
	Action      authz.Action
}

type Decision struct {
	Allowed                  bool
	EffectiveWorkspaceRole   string
	EffectivePagePermissions []string
	DenialReason             string
}

type Client struct {
	remote authv1.AuthorizationServiceClient
}

func NewClient(remote authv1.AuthorizationServiceClient) *Client {
	if remote == nil {
		return nil
	}
	return &Client{remote: remote}
}

func (c *Client) Enabled() bool {
	return c != nil && c.remote != nil
}

func (c *Client) Authorize(ctx context.Context, input CheckInput) (Decision, error) {
	if !c.Enabled() {
		return Decision{}, nil
	}

	response, err := c.remote.Authorize(ctx, &authv1.AuthorizeRequest{
		Identity: authv1.IdentityContext{
			RequestID:     transport.RequestIDFromContext(ctx),
			CorrelationID: transport.CorrelationIDFromContext(ctx),
			ActorUserID:   input.ActorUserID,
			WorkspaceID:   input.WorkspaceID,
		},
		PageID: input.PageID,
		Action: string(input.Action),
	})
	if err != nil {
		return Decision{}, err
	}

	return Decision{
		Allowed:                  response.Allowed,
		EffectiveWorkspaceRole:   response.EffectiveWorkspaceRole,
		EffectivePagePermissions: append([]string(nil), response.EffectivePagePermissions...),
		DenialReason:             response.DenialReason,
	}, nil
}

func (c *Client) BatchAuthorize(ctx context.Context, inputs []CheckInput) ([]Decision, error) {
	if !c.Enabled() || len(inputs) == 0 {
		return nil, nil
	}

	checks := make([]authv1.AuthorizeRequest, 0, len(inputs))
	for _, input := range inputs {
		checks = append(checks, authv1.AuthorizeRequest{
			Identity: authv1.IdentityContext{
				RequestID:     transport.RequestIDFromContext(ctx),
				CorrelationID: transport.CorrelationIDFromContext(ctx),
				ActorUserID:   input.ActorUserID,
				WorkspaceID:   input.WorkspaceID,
			},
			PageID: input.PageID,
			Action: string(input.Action),
		})
	}

	response, err := c.remote.BatchAuthorize(ctx, &authv1.BatchAuthorizeRequest{
		Identity: authv1.IdentityContext{
			RequestID:     transport.RequestIDFromContext(ctx),
			CorrelationID: transport.CorrelationIDFromContext(ctx),
		},
		Checks: checks,
	})
	if err != nil {
		return nil, err
	}

	results := make([]Decision, 0, len(response.Results))
	for _, result := range response.Results {
		results = append(results, Decision{
			Allowed:                  result.Allowed,
			EffectiveWorkspaceRole:   result.EffectiveWorkspaceRole,
			EffectivePagePermissions: append([]string(nil), result.EffectivePagePermissions...),
			DenialReason:             result.DenialReason,
		})
	}
	return results, nil
}
