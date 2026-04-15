package memory

import (
	"context"
	"sync"
	"time"

	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/ports"
)

type RoomStateStore struct {
	mu     sync.RWMutex
	values map[string]domain.RoomState
}

func NewRoomStateStore() *RoomStateStore {
	return &RoomStateStore{values: map[string]domain.RoomState{}}
}

func (s *RoomStateStore) Get(_ context.Context, roomID string) (domain.RoomState, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.values[roomID]
	if !ok || (!state.ExpiresAt.IsZero() && time.Now().UTC().After(state.ExpiresAt)) {
		return domain.RoomState{}, false, nil
	}
	return state, true, nil
}

func (s *RoomStateStore) Save(_ context.Context, state domain.RoomState, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state.ExpiresAt = time.Now().UTC().Add(ttl)
	s.values[state.RoomID] = state
	return nil
}

func (s *RoomStateStore) Delete(_ context.Context, roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.values, roomID)
	return nil
}

type PresenceStore struct {
	mu      sync.RWMutex
	members map[string]map[string]domain.PresenceMember
}

func NewPresenceStore() *PresenceStore {
	return &PresenceStore{members: map[string]map[string]domain.PresenceMember{}}
}

func (s *PresenceStore) Upsert(_ context.Context, member domain.PresenceMember, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.members[member.RoomID] == nil {
		s.members[member.RoomID] = map[string]domain.PresenceMember{}
	}
	member.LastSeenAt = time.Now().UTC().Add(ttl)
	s.members[member.RoomID][member.SessionID] = member
	return nil
}

func (s *PresenceStore) Touch(_ context.Context, roomID, sessionID string, cursor *domain.Cursor, selection *domain.Selection, seenAt time.Time, ttl time.Duration) (domain.PresenceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	member, ok := s.members[roomID][sessionID]
	if !ok {
		return domain.PresenceMember{}, domain.ErrSessionNotFound
	}
	member.Cursor = cursor
	if selection != nil {
		member.Selection = selection
	}
	member.LastSeenAt = seenAt.Add(ttl)
	s.members[roomID][sessionID] = member
	return member, nil
}

func (s *PresenceStore) Remove(_ context.Context, roomID, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if roomMembers := s.members[roomID]; roomMembers != nil {
		delete(roomMembers, sessionID)
		if len(roomMembers) == 0 {
			delete(s.members, roomID)
		}
	}
	return nil
}

func (s *PresenceStore) List(_ context.Context, roomID string) ([]domain.PresenceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	roomMembers := s.members[roomID]
	now := time.Now().UTC()
	result := make([]domain.PresenceMember, 0, len(roomMembers))
	for sessionID, member := range roomMembers {
		if !member.LastSeenAt.IsZero() && now.After(member.LastSeenAt) {
			delete(roomMembers, sessionID)
			continue
		}
		result = append(result, member)
	}
	if len(roomMembers) == 0 {
		delete(s.members, roomID)
	}
	return result, nil
}

var _ ports.RoomStateStore = (*RoomStateStore)(nil)
var _ ports.PresenceStore = (*PresenceStore)(nil)
