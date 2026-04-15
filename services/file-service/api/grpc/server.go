package grpcapi

import (
	"context"
	"errors"

	filev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/filev1"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	getFileMetadata *usecase.GetFileMetadata
}

func NewServer(getFileMetadata *usecase.GetFileMetadata) *Server {
	return &Server{getFileMetadata: getFileMetadata}
}

func (s *Server) GetFileMetadata(ctx context.Context, request *filev1.GetFileMetadataRequest) (*filev1.GetFileMetadataResponse, error) {
	file, found, err := s.getFileMetadata.Execute(ctx, usecase.GetFileMetadataInput{
		FileID:        request.FileID,
		Authenticated: false,
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if !found {
		return &filev1.GetFileMetadataResponse{Exists: false}, nil
	}
	return &filev1.GetFileMetadataResponse{
		Exists:      true,
		Status:      string(file.Status),
		Filename:    file.Filename,
		ContentType: file.ContentType,
		SizeBytes:   file.SizeBytes,
		ObjectKey:   file.ObjectKey,
	}, nil
}
