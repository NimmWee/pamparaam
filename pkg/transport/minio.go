package transport

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOConfig carries the minimum configuration required for object storage access.
type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	DefaultBucket   string
	EnsureBucket    bool
}

// OpenMinIO creates a MinIO client and optionally ensures the default bucket exists.
func OpenMinIO(ctx context.Context, cfg MinIOConfig) (*minio.Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint is required")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	if cfg.EnsureBucket && cfg.DefaultBucket != "" {
		exists, err := client.BucketExists(ctx, cfg.DefaultBucket)
		if err != nil {
			return nil, err
		}

		if !exists {
			if err := client.MakeBucket(ctx, cfg.DefaultBucket, minio.MakeBucketOptions{}); err != nil {
				return nil, err
			}
		}
	}

	return client, nil
}
