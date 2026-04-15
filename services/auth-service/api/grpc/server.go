package grpcapi

import (
	"context"

	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/usecase"
)

type Server struct {
	authorizer *usecase.Authorizer
}

func NewServer(authorizer *usecase.Authorizer) *Server {
	return &Server{authorizer: authorizer}
}

func (s *Server) Authorize(ctx context.Context, request *authv1.AuthorizeRequest) (*authv1.AuthorizeResponse, error) {
	decision, err := s.authorizer.Execute(ctx, usecase.AuthorizeInput{
		SubjectUserID: request.Identity.ActorUserID,
		WorkspaceID:   request.Identity.WorkspaceID,
		PageID:        request.PageID,
		Action:        domain.Action(request.Action),
	})
	if err != nil {
		return nil, err
	}

	return &authv1.AuthorizeResponse{
		Allowed:                  decision.Allowed,
		EffectiveWorkspaceRole:   string(decision.EffectiveWorkspaceRole),
		EffectivePagePermissions: decision.EffectivePagePermissions,
		DenialReason:             decision.DenialReason,
	}, nil
}

func (s *Server) BatchAuthorize(ctx context.Context, request *authv1.BatchAuthorizeRequest) (*authv1.BatchAuthorizeResponse, error) {
	results := make([]authv1.AuthorizeResponse, 0, len(request.Checks))
	for _, check := range request.Checks {
		decision, err := s.authorizer.Execute(ctx, usecase.AuthorizeInput{
			SubjectUserID: check.Identity.ActorUserID,
			WorkspaceID:   check.Identity.WorkspaceID,
			PageID:        check.PageID,
			Action:        domain.Action(check.Action),
		})
		if err != nil {
			return nil, err
		}

		results = append(results, authv1.AuthorizeResponse{
			Allowed:                  decision.Allowed,
			EffectiveWorkspaceRole:   string(decision.EffectiveWorkspaceRole),
			EffectivePagePermissions: decision.EffectivePagePermissions,
			DenialReason:             decision.DenialReason,
		})
	}

	return &authv1.BatchAuthorizeResponse{Results: results}, nil
}
