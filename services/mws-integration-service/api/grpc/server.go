package grpcapi

import (
	"context"
	"encoding/json"

	mwsv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/mwsv1"
	"github.com/mtc/wiki-editor-backend/services/mws-integration-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	resolver *usecase.EmbedResolver
}

func NewServer(resolver *usecase.EmbedResolver) *Server {
	return &Server{resolver: resolver}
}

func (s *Server) ResolveEmbed(ctx context.Context, request *mwsv1.ResolveEmbedRequest) (*mwsv1.ResolveEmbedResponse, error) {
	displayConfig := map[string]any{}
	if len(request.DisplayConfigJSON) > 0 {
		if err := json.Unmarshal(request.DisplayConfigJSON, &displayConfig); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	result, err := s.resolver.Resolve(ctx, usecase.ResolveEmbedInput{
		AccessToken:   request.Identity.AccessToken,
		PageID:        request.PageID,
		MWSTableID:    request.MwsTableID,
		DisplayConfig: displayConfig,
		AllowDegraded: request.AllowDegraded,
		StoredTitle:   request.StoredTitle,
		ForceRefresh:  request.ForceRefresh,
	})
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	schemaJSON, err := json.Marshal(result.Schema)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	previewJSON, err := json.Marshal(result.PreviewRows)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &mwsv1.ResolveEmbedResponse{
		Allowed:         result.Allowed,
		Title:           result.Title,
		SchemaJSON:      schemaJSON,
		PreviewRowsJSON: previewJSON,
		PreviewState:    string(result.PreviewState),
		CacheTTLSeconds: result.CacheTTLSeconds,
	}, nil
}

func (s *Server) RefreshEmbedPreview(ctx context.Context, request *mwsv1.RefreshEmbedPreviewRequest) (*mwsv1.RefreshEmbedPreviewResponse, error) {
	result, err := s.resolver.Refresh(ctx, usecase.ResolveEmbedInput{
		PageID:        request.PageID,
		MWSTableID:    request.MwsTableID,
		AllowDegraded: true,
	})
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	return &mwsv1.RefreshEmbedPreviewResponse{
		PreviewState: string(result.PreviewState),
	}, nil
}
