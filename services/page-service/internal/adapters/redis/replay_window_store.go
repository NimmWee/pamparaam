package redis

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type ReplayWindowStore struct {
	client *goredis.Client
	prefix string
	limit  int64
}

func NewReplayWindowStore(client *goredis.Client, limit int64) *ReplayWindowStore {
	if limit <= 0 {
		limit = 32
	}
	return &ReplayWindowStore{
		client: client,
		prefix: "page:replay-window:",
		limit:  limit,
	}
}

func (s *ReplayWindowStore) Append(ctx context.Context, entry domain.ReplayWindowEntry, ttl time.Duration) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	key := s.prefix + entry.PageID
	pipe := s.client.TxPipeline()
	pipe.RPush(ctx, key, payload)
	pipe.LTrim(ctx, key, -s.limit, -1)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *ReplayWindowStore) ListSinceRevision(ctx context.Context, pageID string, fromRevisionNo int64) ([]domain.ReplayWindowEntry, error) {
	values, err := s.client.LRange(ctx, s.prefix+pageID, 0, -1).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := make([]domain.ReplayWindowEntry, 0, len(values))
	for _, value := range values {
		var entry domain.ReplayWindowEntry
		if err := json.Unmarshal([]byte(value), &entry); err != nil {
			return nil, err
		}
		if entry.RevisionNo > fromRevisionNo {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

var _ ports.ReplayWindowStore = (*ReplayWindowStore)(nil)
