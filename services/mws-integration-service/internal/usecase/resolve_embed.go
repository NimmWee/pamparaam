package usecase

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/services/mws-integration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/mws-integration-service/internal/ports"
)

type ResolveEmbedInput struct {
	AccessToken   string
	PageID        string
	MWSTableID    string
	DisplayConfig map[string]any
	AllowDegraded bool
	StoredTitle   string
	ForceRefresh  bool
}

type EmbedResolver struct {
	client   ports.MWSClient
	cache    ports.Cache
	now      func() time.Time
	cacheTTL time.Duration
}

func NewEmbedResolver(client ports.MWSClient, cache ports.Cache, now func() time.Time, cacheTTL time.Duration) *EmbedResolver {
	if now == nil {
		now = time.Now
	}
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &EmbedResolver{
		client:   client,
		cache:    cache,
		now:      now,
		cacheTTL: cacheTTL,
	}
}

func (r *EmbedResolver) Resolve(ctx context.Context, input ResolveEmbedInput) (domain.ResolvedEmbed, error) {
	if err := r.client.ValidateAccess(ctx, input.AccessToken, input.MWSTableID); err != nil {
		return r.degradedOrError(ctx, input, err)
	}

	schema, err := r.client.FetchSchema(ctx, input.AccessToken, input.MWSTableID)
	if err != nil {
		return r.degradedOrError(ctx, input, err)
	}

	previewRows, err := r.client.FetchPreview(ctx, input.AccessToken, input.MWSTableID)
	if err != nil {
		return r.degradedOrError(ctx, input, err)
	}

	resolved := domain.ResolvedEmbed{
		Allowed:         true,
		MWSTableID:      input.MWSTableID,
		Title:           firstNonEmpty(input.StoredTitle, input.MWSTableID),
		DisplayConfig:   input.DisplayConfig,
		Schema:          schema,
		PreviewRows:     previewRows,
		PreviewState:    domain.PreviewStateReady,
		CacheTTLSeconds: int32(r.cacheTTL / time.Second),
	}
	if resolved.Title == "" {
		resolved.Title = input.MWSTableID
	}

	if r.cache != nil {
		_ = r.cache.Put(ctx, domain.CachedTablePreview{
			MWSTableID: input.MWSTableID,
			Title:      resolved.Title,
			Schema:     schema,
			Preview:    previewRows,
			FetchedAt:  r.now().UTC(),
			ExpiresAt:  r.now().UTC().Add(r.cacheTTL),
		})
	}

	return resolved, nil
}

func (r *EmbedResolver) Refresh(ctx context.Context, input ResolveEmbedInput) (domain.ResolvedEmbed, error) {
	input.AllowDegraded = true
	input.ForceRefresh = true
	return r.Resolve(ctx, input)
}

func (r *EmbedResolver) degradedOrError(ctx context.Context, input ResolveEmbedInput, cause error) (domain.ResolvedEmbed, error) {
	if !input.AllowDegraded {
		return domain.ResolvedEmbed{}, cause
	}

	title := firstNonEmpty(input.StoredTitle, input.MWSTableID)
	resolved := domain.ResolvedEmbed{
		Allowed:         true,
		MWSTableID:      input.MWSTableID,
		Title:           title,
		DisplayConfig:   input.DisplayConfig,
		PreviewState:    domain.PreviewStateDegraded,
		CacheTTLSeconds: int32(r.cacheTTL / time.Second),
	}

	if r.cache == nil {
		return resolved, nil
	}

	cached, found, err := r.cache.Get(ctx, input.MWSTableID)
	if err != nil || !found {
		return resolved, nil
	}

	resolved.Title = firstNonEmpty(cached.Title, title)
	resolved.Schema = cached.Schema
	resolved.PreviewRows = cached.Preview
	return resolved, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
