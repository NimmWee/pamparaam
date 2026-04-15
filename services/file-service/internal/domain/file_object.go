package domain

import "time"

type FileStatus string

const (
	FileStatusUploading FileStatus = "uploading"
	FileStatusReady     FileStatus = "ready"
	FileStatusDeleted   FileStatus = "deleted"
)

type FileObject struct {
	ID          string
	WorkspaceID string
	PageID      string
	ObjectKey   string
	Filename    string
	ContentType string
	SizeBytes   int64
	Checksum    string
	Status      FileStatus
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type UploadSession struct {
	ID          string
	FileID      string
	WorkspaceID string
	PageID      string
	ObjectKey   string
	Filename    string
	ContentType string
	SizeBytes   int64
	Checksum    string
	ExpiresAt   time.Time
	CompletedAt *time.Time
}

type UploadSessionPayload struct {
	UploadID  string            `json:"upload_id"`
	FileID    string            `json:"file_id"`
	ObjectKey string            `json:"object_key"`
	UploadURL string            `json:"upload_url"`
	ExpiresAt time.Time         `json:"expires_at"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type FileObjectPayload struct {
	FileID      string     `json:"file_id"`
	ObjectKey   string     `json:"object_key"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
	Status      FileStatus `json:"status"`
	DownloadURL string     `json:"download_url,omitempty"`
}
