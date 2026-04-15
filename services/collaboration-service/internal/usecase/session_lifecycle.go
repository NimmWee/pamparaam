package usecase

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/ports"
)

type SessionLifecycle struct {
	rooms      ports.RoomStateStore
	presence   ports.PresenceStore
	pageClient ports.PageRevisionClient
	now        func() time.Time
	nextID     func() string
}

func NewSessionLifecycle(rooms ports.RoomStateStore, presence ports.PresenceStore, pageClient ports.PageRevisionClient, now func() time.Time, nextID func() string) *SessionLifecycle {
	return &SessionLifecycle{rooms: rooms, presence: presence, pageClient: pageClient, now: now, nextID: nextID}
}

type JoinInput struct {
	PageID              string
	WorkspaceID         string
	ActorUserID         string
	DisplayName         string
	LastKnownRevisionNo int64
	LastKnownPatchID    string
	TTL                 time.Duration
	HeartbeatInterval   time.Duration
}

type JoinResult struct {
	Joined   *domain.SessionJoinedPayload
	Presence domain.PresenceStatePayload
	Rebase   *domain.RebaseRequiredPayload
}

func (u *SessionLifecycle) Join(ctx context.Context, input JoinInput) (JoinResult, error) {
	roomID := domain.RoomKey(input.WorkspaceID, input.PageID)
	room, found, err := u.rooms.Get(ctx, roomID)
	if err != nil {
		return JoinResult{}, err
	}
	if !found {
		refreshed, err := u.RefreshSnapshot(ctx, input.PageID, input.WorkspaceID)
		if err != nil {
			return JoinResult{}, err
		}
		room = refreshed
		if err := u.rooms.Save(ctx, room, input.TTL); err != nil {
			return JoinResult{}, err
		}
	}

	if input.LastKnownRevisionNo > 0 && input.LastKnownRevisionNo < room.CurrentRevisionNo {
		return JoinResult{
			Rebase: &domain.RebaseRequiredPayload{
				Reason:             "stale_patch",
				LatestRevisionNo:   room.CurrentRevisionNo,
				LatestRevisionID:   room.CurrentRevisionID,
				ServerDocument:     room.Document,
				ConflictingPatchID: input.LastKnownPatchID,
			},
		}, nil
	}

	sessionID := u.nextID()
	member := domain.PresenceMember{
		RoomID:      roomID,
		SessionID:   sessionID,
		UserID:      input.ActorUserID,
		DisplayName: input.DisplayName,
		WorkspaceID: input.WorkspaceID,
		PageID:      input.PageID,
		LastSeenAt:  u.now().UTC(),
	}
	if err := u.presence.Upsert(ctx, member, input.TTL); err != nil {
		return JoinResult{}, err
	}

	members, err := u.presence.List(ctx, roomID)
	if err != nil {
		return JoinResult{}, err
	}

	return JoinResult{
		Joined: &domain.SessionJoinedPayload{
			SessionID:                sessionID,
			PageID:                   input.PageID,
			WorkspaceID:              input.WorkspaceID,
			CurrentRevisionNo:        room.CurrentRevisionNo,
			CurrentRevisionID:        room.CurrentRevisionID,
			Document:                 room.Document,
			HeartbeatIntervalSeconds: int(input.HeartbeatInterval.Seconds()),
			PresenceTTLSeconds:       int(input.TTL.Seconds()),
		},
		Presence: domain.PresenceStatePayload{
			SessionID: sessionID,
			Members:   members,
		},
	}, nil
}

func (u *SessionLifecycle) UpdatePresence(ctx context.Context, roomID, sessionID string, cursor *domain.Cursor, selection *domain.Selection, ttl time.Duration) (domain.PresenceMember, error) {
	return u.presence.Touch(ctx, roomID, sessionID, cursor, selection, u.now().UTC(), ttl)
}

func (u *SessionLifecycle) Heartbeat(ctx context.Context, roomID, sessionID string, cursor *domain.Cursor, ttl time.Duration) (domain.PongPayload, domain.PresenceMember, error) {
	member, err := u.presence.Touch(ctx, roomID, sessionID, cursor, nil, u.now().UTC(), ttl)
	if err != nil {
		return domain.PongPayload{}, domain.PresenceMember{}, err
	}
	return domain.PongPayload{
		SessionID:  sessionID,
		ReceivedAt: u.now().UTC(),
	}, member, nil
}

func (u *SessionLifecycle) Leave(ctx context.Context, roomID, sessionID string) error {
	return u.presence.Remove(ctx, roomID, sessionID)
}

func (u *SessionLifecycle) ListPresence(ctx context.Context, roomID string) ([]domain.PresenceMember, error) {
	return u.presence.List(ctx, roomID)
}

func (u *SessionLifecycle) RefreshSnapshot(ctx context.Context, pageID, workspaceID string) (domain.RoomState, error) {
	head, err := u.pageClient.GetRevisionHead(ctx, ports.PageRevisionHeadInput{
		PageID:        pageID,
		RequestID:     transport.RequestIDFromContext(ctx),
		CorrelationID: transport.CorrelationIDFromContext(ctx),
		WorkspaceID:   workspaceID,
	})
	if err != nil {
		return domain.RoomState{}, err
	}
	return domain.RoomState{
		RoomID:            domain.RoomKey(workspaceID, pageID),
		PageID:            pageID,
		WorkspaceID:       workspaceID,
		CurrentRevisionID: head.RevisionID,
		CurrentRevisionNo: head.RevisionNo,
		Document:          head.Document,
	}, nil
}

func (u *SessionLifecycle) RefreshExistingRoom(ctx context.Context, pageID, workspaceID string, ttl time.Duration) error {
	roomID := domain.RoomKey(workspaceID, pageID)
	room, found, err := u.rooms.Get(ctx, roomID)
	if err != nil || !found {
		return err
	}

	refreshed, err := u.RefreshSnapshot(ctx, pageID, workspaceID)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = time.Until(room.ExpiresAt)
	}
	if ttl <= 0 {
		ttl = 45 * time.Second
	}
	return u.rooms.Save(ctx, refreshed, ttl)
}
