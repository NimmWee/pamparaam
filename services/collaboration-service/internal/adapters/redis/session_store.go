package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/ports"
	goredis "github.com/redis/go-redis/v9"
)

type RoomStateStore struct {
	client *goredis.Client
	prefix string
}

func NewRoomStateStore(client *goredis.Client) *RoomStateStore {
	return &RoomStateStore{client: client, prefix: "collab:rooms:"}
}

func (s *RoomStateStore) Get(ctx context.Context, roomID string) (domain.RoomState, bool, error) {
	value, err := s.client.Get(ctx, s.prefix+roomID).Result()
	if err == goredis.Nil {
		return domain.RoomState{}, false, nil
	}
	if err != nil {
		return domain.RoomState{}, false, err
	}
	var state domain.RoomState
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		return domain.RoomState{}, false, err
	}
	return state, true, nil
}

func (s *RoomStateStore) Save(ctx context.Context, state domain.RoomState, ttl time.Duration) error {
	state.ExpiresAt = time.Now().UTC().Add(ttl)
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.prefix+state.RoomID, payload, ttl).Err()
}

func (s *RoomStateStore) Delete(ctx context.Context, roomID string) error {
	return s.client.Del(ctx, s.prefix+roomID).Err()
}

var _ ports.RoomStateStore = (*RoomStateStore)(nil)
