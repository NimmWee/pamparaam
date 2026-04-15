package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SubmitPatch struct {
	rooms      ports.RoomStateStore
	presence   ports.PresenceStore
	pageClient ports.PageRevisionClient
	roomLocks  sync.Map
}

func NewSubmitPatch(rooms ports.RoomStateStore, presence ports.PresenceStore, pageClient ports.PageRevisionClient) *SubmitPatch {
	return &SubmitPatch{rooms: rooms, presence: presence, pageClient: pageClient}
}

type SubmitPatchInput struct {
	SessionID      string
	PageID         string
	WorkspaceID    string
	ActorUserID    string
	BaseRevisionNo int64
	PatchID        string
	Ops            []domain.PatchOperation
	TTL            time.Duration
}

func (u *SubmitPatch) Execute(ctx context.Context, input SubmitPatchInput) (domain.PatchAcceptedPayload, *domain.RebaseRequiredPayload, *domain.PatchRejectedPayload, error) {
	roomID := domain.RoomKey(input.WorkspaceID, input.PageID)
	lock := u.roomLock(roomID)
	lock.Lock()
	defer lock.Unlock()

	room, found, err := u.rooms.Get(ctx, roomID)
	if err != nil {
		return domain.PatchAcceptedPayload{}, nil, nil, err
	}
	if !found {
		refreshed, refreshErr := u.reloadRoom(ctx, input.PageID, input.WorkspaceID, input.ActorUserID, input.TTL)
		if refreshErr != nil {
			return domain.PatchAcceptedPayload{}, nil, nil, refreshErr
		}
		room = refreshed
	}

	members, err := u.presence.List(ctx, roomID)
	if err != nil {
		return domain.PatchAcceptedPayload{}, nil, nil, err
	}
	sessionFound := false
	for _, member := range members {
		if member.SessionID == input.SessionID {
			sessionFound = true
			break
		}
	}
	if !sessionFound {
		return domain.PatchAcceptedPayload{}, nil, &domain.PatchRejectedPayload{
			SessionID: input.SessionID,
			PatchID:   input.PatchID,
			Reason:    "session_not_found",
		}, nil
	}

	if room.CurrentRevisionNo != input.BaseRevisionNo {
		return domain.PatchAcceptedPayload{}, &domain.RebaseRequiredPayload{
			SessionID:          input.SessionID,
			Reason:             "stale_patch",
			LatestRevisionNo:   room.CurrentRevisionNo,
			LatestRevisionID:   room.CurrentRevisionID,
			ServerDocument:     room.Document,
			ConflictingPatchID: input.PatchID,
		}, nil, nil
	}

	nextDocument, err := domain.ApplyPatch(room.Document, input.Ops)
	if err != nil {
		var validation *domain.ValidationError
		if errors.As(err, &validation) {
			return domain.PatchAcceptedPayload{}, nil, &domain.PatchRejectedPayload{
				SessionID: input.SessionID,
				PatchID:   input.PatchID,
				Reason:    "validation_failed",
				Details: map[string]any{
					"block_id": validation.BlockID,
					"message":  validation.Message,
				},
			}, nil
		}
		return domain.PatchAcceptedPayload{}, nil, nil, err
	}

	result, err := u.pageClient.CommitRevision(ctx, ports.CommitRevisionInput{
		PageID:         input.PageID,
		BaseRevisionNo: input.BaseRevisionNo,
		PatchID:        input.PatchID,
		Ops:            input.Ops,
		Document:       nextDocument,
		RequestID:      transport.RequestIDFromContext(ctx),
		CorrelationID:  transport.CorrelationIDFromContext(ctx),
		ActorUserID:    input.ActorUserID,
		WorkspaceID:    input.WorkspaceID,
	})
	if err != nil {
		if status.Code(err) == codes.Aborted {
			head, headErr := u.pageClient.GetRevisionHead(ctx, ports.PageRevisionHeadInput{
				PageID:        input.PageID,
				RequestID:     transport.RequestIDFromContext(ctx),
				CorrelationID: transport.CorrelationIDFromContext(ctx),
				ActorUserID:   input.ActorUserID,
				WorkspaceID:   input.WorkspaceID,
			})
			if headErr != nil {
				return domain.PatchAcceptedPayload{}, nil, nil, headErr
			}
			updated := domain.RoomState{
				RoomID:            roomID,
				PageID:            input.PageID,
				WorkspaceID:       input.WorkspaceID,
				CurrentRevisionID: head.RevisionID,
				CurrentRevisionNo: head.RevisionNo,
				Document:          head.Document,
			}
			if saveErr := u.rooms.Save(ctx, updated, input.TTL); saveErr != nil {
				return domain.PatchAcceptedPayload{}, nil, nil, saveErr
			}
			return domain.PatchAcceptedPayload{}, &domain.RebaseRequiredPayload{
				SessionID:          input.SessionID,
				Reason:             "stale_patch",
				LatestRevisionNo:   head.RevisionNo,
				LatestRevisionID:   head.RevisionID,
				ServerDocument:     head.Document,
				ConflictingPatchID: input.PatchID,
			}, nil, nil
		}
		return domain.PatchAcceptedPayload{}, nil, nil, err
	}

	room.Document = nextDocument
	room.CurrentRevisionID = result.AcceptedRevisionID
	room.CurrentRevisionNo = result.AcceptedRevisionNo
	room.LastPatchID = strings.TrimSpace(input.PatchID)
	if err := u.rooms.Save(ctx, room, input.TTL); err != nil {
		return domain.PatchAcceptedPayload{}, nil, nil, err
	}

	return domain.PatchAcceptedPayload{
		SessionID:          input.SessionID,
		PageID:             input.PageID,
		AcceptedRevisionNo: result.AcceptedRevisionNo,
		AcceptedRevisionID: result.AcceptedRevisionID,
		PatchID:            input.PatchID,
		Ops:                input.Ops,
	}, nil, nil, nil
}

func (u *SubmitPatch) reloadRoom(ctx context.Context, pageID, workspaceID, actorUserID string, ttl time.Duration) (domain.RoomState, error) {
	head, err := u.pageClient.GetRevisionHead(ctx, ports.PageRevisionHeadInput{
		PageID:        pageID,
		RequestID:     transport.RequestIDFromContext(ctx),
		CorrelationID: transport.CorrelationIDFromContext(ctx),
		ActorUserID:   actorUserID,
		WorkspaceID:   workspaceID,
	})
	if err != nil {
		return domain.RoomState{}, err
	}
	room := domain.RoomState{
		RoomID:            domain.RoomKey(workspaceID, pageID),
		PageID:            pageID,
		WorkspaceID:       workspaceID,
		CurrentRevisionID: head.RevisionID,
		CurrentRevisionNo: head.RevisionNo,
		Document:          head.Document,
	}
	return room, u.rooms.Save(ctx, room, ttl)
}

func (u *SubmitPatch) roomLock(roomID string) *sync.Mutex {
	lock, _ := u.roomLocks.LoadOrStore(roomID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}
