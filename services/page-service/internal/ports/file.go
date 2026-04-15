package ports

import "context"

type FileMetadataInput struct {
	FileID      string
	PageID      string
	WorkspaceID string
	ActorUserID string
}

type FileMetadata struct {
	Exists      bool
	Status      string
	Filename    string
	ContentType string
	SizeBytes   int64
	ObjectKey   string
}

type FileMetadataResolver interface {
	GetFileMetadata(ctx context.Context, input FileMetadataInput) (FileMetadata, error)
}
