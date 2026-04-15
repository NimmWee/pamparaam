package minioadapter

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
)

type ObjectStore struct {
	client        *minio.Client
	bucket        string
	publicBaseURL *url.URL
}

func NewObjectStore(client *minio.Client, bucket, publicBaseURL string) *ObjectStore {
	store := &ObjectStore{client: client, bucket: bucket}
	if publicBaseURL != "" {
		if parsed, err := url.Parse(publicBaseURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			store.publicBaseURL = parsed
		}
	}
	return store
}

func (s *ObjectStore) PresignUpload(ctx context.Context, objectKey string, expiresIn time.Duration) (string, map[string]string, error) {
	urlValue, err := s.client.PresignedPutObject(ctx, s.bucket, objectKey, expiresIn)
	if err != nil {
		return "", nil, err
	}
	return s.publicURL(urlValue), map[string]string{}, nil
}

func (s *ObjectStore) PresignDownload(ctx context.Context, objectKey string, expiresIn time.Duration) (string, error) {
	urlValue, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, expiresIn, url.Values{})
	if err != nil {
		return "", err
	}
	return s.publicURL(urlValue), nil
}

func (s *ObjectStore) publicURL(source *url.URL) string {
	if s.publicBaseURL == nil {
		return source.String()
	}

	value := *source
	value.Scheme = s.publicBaseURL.Scheme
	value.Host = s.publicBaseURL.Host
	if s.publicBaseURL.Path != "" && s.publicBaseURL.Path != "/" {
		value.Path = fmt.Sprintf("%s%s", strings.TrimRight(s.publicBaseURL.Path, "/"), value.Path)
	}
	return value.String()
}

var _ ports.ObjectStore = (*ObjectStore)(nil)
