package domain

import "time"

type PreviewState string

const (
	PreviewStateReady    PreviewState = "ready"
	PreviewStateCached   PreviewState = "cached"
	PreviewStateDegraded PreviewState = "degraded"
)

type CachedTablePreview struct {
	MWSTableID string
	Title      string
	Schema     map[string]any
	Preview    []map[string]any
	FetchedAt  time.Time
	ExpiresAt  time.Time
}

type ResolvedEmbed struct {
	Allowed         bool
	MWSTableID      string
	Title           string
	DisplayConfig   map[string]any
	Schema          map[string]any
	PreviewRows     []map[string]any
	PreviewState    PreviewState
	CacheTTLSeconds int32
}
