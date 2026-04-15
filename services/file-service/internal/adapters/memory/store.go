package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
)

type Store struct {
	mu      sync.RWMutex
	files   map[string]domain.FileObject
	uploads map[string]domain.UploadSession
	baseURL string
}

func NewStore() *Store {
	return &Store{
		files:   map[string]domain.FileObject{},
		uploads: map[string]domain.UploadSession{},
		baseURL: "https://files.example.test",
	}
}

func (s *Store) CreateUploadSession(_ context.Context, session domain.UploadSession, file domain.FileObject) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploads[session.ID] = session
	s.files[file.ID] = file
	return nil
}

func (s *Store) GetUploadSession(_ context.Context, uploadID string) (domain.UploadSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.uploads[uploadID]
	if !ok {
		return domain.UploadSession{}, domain.ErrNotFound
	}
	return session, nil
}

func (s *Store) CompleteUpload(_ context.Context, uploadID string, file domain.FileObject) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.uploads[uploadID]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	session.CompletedAt = &now
	s.uploads[uploadID] = session
	s.files[file.ID] = file
	return nil
}

func (s *Store) GetFile(_ context.Context, fileID string) (domain.FileObject, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	file, ok := s.files[fileID]
	if !ok || file.Status == domain.FileStatusDeleted || file.DeletedAt != nil {
		return domain.FileObject{}, domain.ErrNotFound
	}
	return file, nil
}

func (s *Store) SoftDelete(_ context.Context, fileID string, deletedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.files[fileID]
	if !ok {
		return domain.ErrNotFound
	}
	file.Status = domain.FileStatusDeleted
	file.DeletedAt = &deletedAt
	file.UpdatedAt = deletedAt
	s.files[fileID] = file
	return nil
}

func (s *Store) Ping(context.Context) error {
	return nil
}

type ObjectStore struct{}

func NewObjectStore() *ObjectStore {
	return &ObjectStore{}
}

func (o *ObjectStore) PresignUpload(_ context.Context, objectKey string, expiresIn time.Duration) (string, map[string]string, error) {
	return fmt.Sprintf("https://files.example.test/upload/%s?expires_in=%d", objectKey, int(expiresIn.Seconds())), map[string]string{"x-upload-token": "memory"}, nil
}

func (o *ObjectStore) PresignDownload(_ context.Context, objectKey string, expiresIn time.Duration) (string, error) {
	return fmt.Sprintf("https://files.example.test/download/%s?expires_in=%d", objectKey, int(expiresIn.Seconds())), nil
}

var _ ports.Store = (*Store)(nil)
var _ ports.ObjectStore = (*ObjectStore)(nil)
