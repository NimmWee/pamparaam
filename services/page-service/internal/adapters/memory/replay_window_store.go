package memory

import (
	"context"
	"sync"
	"time"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type ReplayWindowStore struct {
	mu      sync.RWMutex
	values  map[string][]domain.ReplayWindowEntry
	limit   int
	clock   func() time.Time
}

func NewReplayWindowStore(limit int, clock func() time.Time) *ReplayWindowStore {
	if limit <= 0 {
		limit = 32
	}
	if clock == nil {
		clock = time.Now
	}
	return &ReplayWindowStore{
		values: make(map[string][]domain.ReplayWindowEntry),
		limit:  limit,
		clock:  clock,
	}
}

func (s *ReplayWindowStore) Append(_ context.Context, entry domain.ReplayWindowEntry, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = s.clock().UTC()
	}
	entries := append(s.values[entry.PageID], entry)
	if ttl > 0 {
		cutoff := s.clock().UTC().Add(-ttl)
		filtered := entries[:0]
		for _, existing := range entries {
			if existing.CreatedAt.After(cutoff) || existing.CreatedAt.Equal(cutoff) {
				filtered = append(filtered, existing)
			}
		}
		entries = append([]domain.ReplayWindowEntry(nil), filtered...)
	}
	if len(entries) > s.limit {
		entries = append([]domain.ReplayWindowEntry(nil), entries[len(entries)-s.limit:]...)
	}
	s.values[entry.PageID] = entries
	return nil
}

func (s *ReplayWindowStore) ListSinceRevision(_ context.Context, pageID string, fromRevisionNo int64) ([]domain.ReplayWindowEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	source := s.values[pageID]
	result := make([]domain.ReplayWindowEntry, 0, len(source))
	for _, entry := range source {
		if entry.RevisionNo > fromRevisionNo {
			result = append(result, entry)
		}
	}
	return result, nil
}

var _ ports.ReplayWindowStore = (*ReplayWindowStore)(nil)
