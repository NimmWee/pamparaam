package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/ports"
	goredis "github.com/redis/go-redis/v9"
)

type PresenceStore struct {
	client    *goredis.Client
	memberKey string
	indexKey  string
}

func NewPresenceStore(client *goredis.Client) *PresenceStore {
	return &PresenceStore{
		client:    client,
		memberKey: "collab:presence:",
		indexKey:  "collab:presence:index:",
	}
}

func (s *PresenceStore) Upsert(ctx context.Context, member domain.PresenceMember, ttl time.Duration) error {
	member.LastSeenAt = time.Now().UTC().Add(ttl)
	payload, err := json.Marshal(member)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.memberRedisKey(member.RoomID, member.SessionID), payload, ttl)
	pipe.SAdd(ctx, s.roomIndexKey(member.RoomID), member.SessionID)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *PresenceStore) Touch(ctx context.Context, roomID, sessionID string, cursor *domain.Cursor, selection *domain.Selection, seenAt time.Time, ttl time.Duration) (domain.PresenceMember, error) {
	member, ok, err := s.getMember(ctx, roomID, sessionID)
	if err != nil {
		return domain.PresenceMember{}, err
	}
	if !ok {
		return domain.PresenceMember{}, domain.ErrSessionNotFound
	}
	member.Cursor = cursor
	if selection != nil {
		member.Selection = selection
	}
	member.LastSeenAt = seenAt.Add(ttl)
	if err := s.Upsert(ctx, member, ttl); err != nil {
		return domain.PresenceMember{}, err
	}
	return member, nil
}

func (s *PresenceStore) Remove(ctx context.Context, roomID, sessionID string) error {
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.memberRedisKey(roomID, sessionID))
	pipe.SRem(ctx, s.roomIndexKey(roomID), sessionID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *PresenceStore) List(ctx context.Context, roomID string) ([]domain.PresenceMember, error) {
	sessionIDs, err := s.client.SMembers(ctx, s.roomIndexKey(roomID)).Result()
	if err != nil {
		return nil, err
	}
	result := make([]domain.PresenceMember, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		member, ok, err := s.getMember(ctx, roomID, sessionID)
		if err != nil {
			return nil, err
		}
		if !ok {
			_ = s.client.SRem(ctx, s.roomIndexKey(roomID), sessionID).Err()
			continue
		}
		result = append(result, member)
	}
	return result, nil
}

func (s *PresenceStore) getMember(ctx context.Context, roomID, sessionID string) (domain.PresenceMember, bool, error) {
	value, err := s.client.Get(ctx, s.memberRedisKey(roomID, sessionID)).Result()
	if err == goredis.Nil {
		return domain.PresenceMember{}, false, nil
	}
	if err != nil {
		return domain.PresenceMember{}, false, err
	}
	var member domain.PresenceMember
	if err := json.Unmarshal([]byte(value), &member); err != nil {
		return domain.PresenceMember{}, false, err
	}
	return member, true, nil
}

func (s *PresenceStore) memberRedisKey(roomID, sessionID string) string {
	return s.memberKey + roomID + ":" + sessionID
}

func (s *PresenceStore) roomIndexKey(roomID string) string {
	return s.indexKey + roomID
}

var _ ports.PresenceStore = (*PresenceStore)(nil)
