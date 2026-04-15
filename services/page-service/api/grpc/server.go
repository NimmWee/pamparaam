package grpcapi

import (
	"context"
	"encoding/json"
	"errors"

	pagev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/pagev1"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	headGetter   *usecase.GetRevisionHead
	commitWriter *usecase.CommitCollaborativeRevision
}

func NewServer(headGetter *usecase.GetRevisionHead, commitWriter *usecase.CommitCollaborativeRevision) *Server {
	return &Server{headGetter: headGetter, commitWriter: commitWriter}
}

func (s *Server) GetRevisionHead(ctx context.Context, request *pagev1.GetRevisionHeadRequest) (*pagev1.GetRevisionHeadResponse, error) {
	result, err := s.headGetter.Execute(ctx, request.PageID, request.Identity.WorkspaceID, request.Identity.ActorUserID, nil, false)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrPageNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	documentJSON, err := json.Marshal(result.Document)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pagev1.GetRevisionHeadResponse{
		PageID:               result.PageID,
		WorkspaceID:          result.WorkspaceID,
		CurrentRevisionID:    result.RevisionID,
		CurrentRevisionNo:    result.RevisionNo,
		DocumentSnapshotJSON: documentJSON,
	}, nil
}

func (s *Server) CommitCollaborativeRevision(ctx context.Context, request *pagev1.CommitCollaborativeRevisionRequest) (*pagev1.CommitCollaborativeRevisionResponse, error) {
	var snapshot domain.Document
	if err := json.Unmarshal(request.NewDocumentSnapshotJSON, &snapshot); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	result, err := s.commitWriter.Execute(ctx, usecase.CommitCollaborativeRevisionInput{
		PageID:         request.PageID,
		BaseRevisionNo: request.BaseRevisionNo,
		PatchID:        request.PatchID,
		Document:       snapshot,
		ActorUserID:    request.Identity.ActorUserID,
		WorkspaceID:    request.Identity.WorkspaceID,
		ActorRoles:     nil,
		Authenticated:  false,
	})
	if err != nil {
		var rebase *usecase.RebaseRequiredError
		switch {
		case errors.As(err, &rebase):
			return nil, status.Error(codes.Aborted, rebase.Error())
		case errors.Is(err, usecase.ErrPageArchived):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		case errors.Is(err, usecase.ErrValidation):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, usecase.ErrPageNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &pagev1.CommitCollaborativeRevisionResponse{
		AcceptedRevisionID: result.AcceptedRevisionID,
		AcceptedRevisionNo: result.AcceptedRevisionNo,
		DocumentHash:       result.DocumentHash,
	}, nil
}
