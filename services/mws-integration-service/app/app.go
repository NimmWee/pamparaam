package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	mwsv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/mwsv1"
	grpcapi "github.com/mtc/wiki-editor-backend/services/mws-integration-service/api/grpc"
	"github.com/mtc/wiki-editor-backend/services/mws-integration-service/internal/adapters"
	"github.com/mtc/wiki-editor-backend/services/mws-integration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/mws-integration-service/internal/usecase"
)

type Config struct {
	MWSBaseURL string
	HTTPClient *http.Client
	Now        func() time.Time
	CacheTTL   time.Duration
}

func NewResolverServer(cfg Config) (mwsv1.MWSIntegrationServiceServer, error) {
	cache := &memoryCache{}
	client := adapters.NewMWSClient(cfg.MWSBaseURL, cfg.HTTPClient)
	resolver := usecase.NewEmbedResolver(client, cache, cfg.Now, cfg.CacheTTL)
	return grpcapi.NewServer(resolver), nil
}

type memoryCache struct {
	mu     sync.RWMutex
	values map[string]domain.CachedTablePreview
}

func (c *memoryCache) Get(_ context.Context, tableID string) (domain.CachedTablePreview, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.values == nil {
		return domain.CachedTablePreview{}, false, nil
	}
	value, ok := c.values[tableID]
	return value, ok, nil
}

func (c *memoryCache) Put(_ context.Context, value domain.CachedTablePreview) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.values == nil {
		c.values = make(map[string]domain.CachedTablePreview)
	}
	c.values[value.MWSTableID] = value
	return nil
}
