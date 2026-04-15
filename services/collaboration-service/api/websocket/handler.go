package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/usecase"
)

type Handler struct {
	lifecycle         *usecase.SessionLifecycle
	submitPatch       *usecase.SubmitPatch
	authorizer        *usecase.SessionAuthorizer
	heartbeatInterval time.Duration
	presenceTTL       time.Duration
	upgrader          websocket.Upgrader

	mu      sync.RWMutex
	clients map[string]map[*clientConn]struct{}
}

type clientConn struct {
	conn          *websocket.Conn
	writeMu       sync.Mutex
	roomID        string
	sessionID     string
	pageID        string
	workspaceID   string
	userID        string
	displayName   string
	roles         []string
	authenticated bool
}

func NewHandler(lifecycle *usecase.SessionLifecycle, submitPatch *usecase.SubmitPatch, authorizer *usecase.SessionAuthorizer, heartbeatInterval, presenceTTL time.Duration) *Handler {
	return &Handler{
		lifecycle:         lifecycle,
		submitPatch:       submitPatch,
		authorizer:        authorizer,
		heartbeatInterval: heartbeatInterval,
		presenceTTL:       presenceTTL,
		clients:           map[string]map[*clientConn]struct{}{},
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/ws/collab", h.handleCollabSocket)
}

func (h *Handler) handleCollabSocket(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(r.Header.Get("X-Auth-User-Id")) == "" {
		http.Error(w, "missing identity headers", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &clientConn{
		conn:          conn,
		userID:        strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		displayName:   strings.TrimSpace(r.Header.Get("X-Auth-Display-Name")),
		roles:         parseRoles(r.Header.Get("X-Auth-Roles")),
		authenticated: strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Auth-Authenticated")), "true"),
	}
	defer func() {
		if client.roomID != "" {
			_ = h.lifecycle.Leave(r.Context(), client.roomID, client.sessionID)
			h.removeClient(client)
		}
		_ = conn.Close()
	}()

	for {
		var envelope domain.Envelope
		if err := conn.ReadJSON(&envelope); err != nil {
			return
		}
		if envelope.SentAt.IsZero() {
			envelope.SentAt = time.Now().UTC()
		}
		if err := h.dispatch(r.Context(), client, r, envelope); err != nil {
			if errors.Is(err, domain.ErrSessionNotFound) {
				_ = client.writeEnvelope(domain.Envelope{
					Type:      domain.EventError,
					RequestID: envelope.RequestID,
					SentAt:    time.Now().UTC(),
					Payload:   mustPayload(domain.ErrorPayload{Code: "session_not_found", Message: "session expired or unknown", Retryable: true}),
				})
				continue
			}
			_ = client.writeEnvelope(domain.Envelope{
				Type:      domain.EventError,
				RequestID: envelope.RequestID,
				SentAt:    time.Now().UTC(),
				Payload:   mustPayload(domain.ErrorPayload{Code: "page_unavailable", Message: err.Error(), Retryable: true}),
			})
		}
	}
}

func (h *Handler) dispatch(ctx context.Context, client *clientConn, r *http.Request, envelope domain.Envelope) error {
	switch envelope.Type {
	case domain.EventJoinSession:
		return h.handleJoin(ctx, client, r, envelope)
	case domain.EventHeartbeat:
		return h.handleHeartbeat(ctx, client, envelope)
	case domain.EventUpdatePresence:
		return h.handlePresence(ctx, client, envelope)
	case domain.EventSubmitPatch:
		return h.handlePatch(ctx, client, envelope)
	case domain.EventLeaveSession:
		return h.handleLeave(ctx, client, envelope)
	default:
		return client.writeEnvelope(domain.Envelope{
			Type:      domain.EventError,
			RequestID: envelope.RequestID,
			SentAt:    time.Now().UTC(),
			Payload:   mustPayload(domain.ErrorPayload{Code: "invalid_payload", Message: "unsupported event type", Retryable: false}),
		})
	}
}

func (h *Handler) handleJoin(ctx context.Context, client *clientConn, r *http.Request, envelope domain.Envelope) error {
	var payload domain.JoinSessionPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return client.writeEnvelope(domain.Envelope{
			Type:      domain.EventError,
			RequestID: envelope.RequestID,
			SentAt:    time.Now().UTC(),
			Payload:   mustPayload(domain.ErrorPayload{Code: "invalid_payload", Message: err.Error(), Retryable: false}),
		})
	}
	if err := h.authorizer.Authorize(ctx, authz.ActionPageView, usecase.AuthorizationSubject{
		ActorUserID:   client.userID,
		WorkspaceID:   payload.WorkspaceID,
		PageID:        payload.PageID,
		Roles:         client.roles,
		Authenticated: client.authenticated,
	}); err != nil {
		return client.writeEnvelope(domain.Envelope{
			Type:      domain.EventError,
			RequestID: envelope.RequestID,
			SentAt:    time.Now().UTC(),
			Payload:   mustPayload(domain.ErrorPayload{Code: "forbidden", Message: "page view permission is required", Retryable: false}),
		})
	}

	result, err := h.lifecycle.Join(ctx, usecase.JoinInput{
		PageID:              payload.PageID,
		WorkspaceID:         payload.WorkspaceID,
		ActorUserID:         client.userID,
		DisplayName:         client.displayName,
		LastKnownRevisionNo: payload.LastKnownRevisionNo,
		LastKnownPatchID:    payload.LastKnownPatchID,
		TTL:                 h.presenceTTL,
		HeartbeatInterval:   h.heartbeatInterval,
	})
	if err != nil {
		return err
	}
	if result.Rebase != nil {
		return client.writeEnvelope(domain.Envelope{
			Type:      domain.EventRebaseRequired,
			RequestID: envelope.RequestID,
			SentAt:    time.Now().UTC(),
			Payload:   mustPayload(result.Rebase),
		})
	}

	client.roomID = domain.RoomKey(payload.WorkspaceID, payload.PageID)
	client.sessionID = result.Joined.SessionID
	client.workspaceID = payload.WorkspaceID
	client.pageID = payload.PageID
	h.addClient(client)

	if err := client.writeEnvelope(domain.Envelope{
		Type:      domain.EventSessionJoined,
		RequestID: envelope.RequestID,
		SentAt:    time.Now().UTC(),
		Payload:   mustPayload(result.Joined),
	}); err != nil {
		return err
	}
	if err := client.writeEnvelope(domain.Envelope{
		Type:      domain.EventPresenceState,
		RequestID: envelope.RequestID,
		SentAt:    time.Now().UTC(),
		Payload:   mustPayload(result.Presence),
	}); err != nil {
		return err
	}
	return h.broadcast(client.roomID, domain.Envelope{
		Type:      domain.EventPresenceChange,
		RequestID: envelope.RequestID,
		SentAt:    time.Now().UTC(),
		Payload: mustPayload(domain.PresenceChangedPayload{
			SessionID: client.sessionID,
			Event:     "joined",
			Member: domain.PresenceMember{
				SessionID:   client.sessionID,
				UserID:      client.userID,
				DisplayName: client.displayName,
			},
		}),
	}, client)
}

func (h *Handler) handleHeartbeat(ctx context.Context, client *clientConn, envelope domain.Envelope) error {
	var payload domain.HeartbeatPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return err
	}
	pong, member, err := h.lifecycle.Heartbeat(ctx, client.roomID, payload.SessionID, payload.Cursor, h.presenceTTL)
	if err != nil {
		return err
	}
	if err := client.writeEnvelope(domain.Envelope{
		Type:      domain.EventPong,
		RequestID: envelope.RequestID,
		SentAt:    time.Now().UTC(),
		Payload:   mustPayload(pong),
	}); err != nil {
		return err
	}
	return h.broadcast(client.roomID, domain.Envelope{
		Type:      domain.EventPresenceChange,
		RequestID: envelope.RequestID,
		SentAt:    time.Now().UTC(),
		Payload: mustPayload(domain.PresenceChangedPayload{
			SessionID: member.SessionID,
			Event:     "updated",
			Member:    member,
		}),
	}, client)
}

func (h *Handler) handlePresence(ctx context.Context, client *clientConn, envelope domain.Envelope) error {
	var payload domain.UpdatePresencePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return err
	}
	member, err := h.lifecycle.UpdatePresence(ctx, client.roomID, payload.SessionID, payload.Cursor, payload.Selection, h.presenceTTL)
	if err != nil {
		return err
	}
	return h.broadcast(client.roomID, domain.Envelope{
		Type:      domain.EventPresenceChange,
		RequestID: envelope.RequestID,
		SentAt:    time.Now().UTC(),
		Payload: mustPayload(domain.PresenceChangedPayload{
			SessionID: payload.SessionID,
			Event:     "updated",
			Member:    member,
		}),
	}, nil)
}

func (h *Handler) handlePatch(ctx context.Context, client *clientConn, envelope domain.Envelope) error {
	if err := h.authorizer.Authorize(ctx, authz.ActionPageEdit, usecase.AuthorizationSubject{
		ActorUserID:   client.userID,
		WorkspaceID:   client.workspaceID,
		PageID:        client.pageID,
		Roles:         client.roles,
		Authenticated: client.authenticated,
	}); err != nil {
		return client.writeEnvelope(domain.Envelope{
			Type:      domain.EventError,
			RequestID: envelope.RequestID,
			SentAt:    time.Now().UTC(),
			Payload:   mustPayload(domain.ErrorPayload{Code: "forbidden", Message: "page edit permission is required", Retryable: false}),
		})
	}
	var payload domain.SubmitPatchPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return err
	}
	accepted, rebase, rejected, err := h.submitPatch.Execute(ctx, usecase.SubmitPatchInput{
		SessionID:      payload.SessionID,
		PageID:         payload.PageID,
		WorkspaceID:    client.workspaceID,
		ActorUserID:    client.userID,
		BaseRevisionNo: payload.BaseRevisionNo,
		PatchID:        payload.PatchID,
		Ops:            payload.Ops,
		TTL:            h.presenceTTL,
	})
	if err != nil {
		return err
	}
	if rebase != nil {
		return client.writeEnvelope(domain.Envelope{
			Type:      domain.EventRebaseRequired,
			RequestID: envelope.RequestID,
			SentAt:    time.Now().UTC(),
			Payload:   mustPayload(rebase),
		})
	}
	if rejected != nil {
		return client.writeEnvelope(domain.Envelope{
			Type:      domain.EventPatchRejected,
			RequestID: envelope.RequestID,
			SentAt:    time.Now().UTC(),
			Payload:   mustPayload(rejected),
		})
	}
	return h.broadcast(client.roomID, domain.Envelope{
		Type:      domain.EventPatchAccepted,
		RequestID: envelope.RequestID,
		SentAt:    time.Now().UTC(),
		Payload:   mustPayload(accepted),
	}, nil)
}

func (h *Handler) handleLeave(ctx context.Context, client *clientConn, envelope domain.Envelope) error {
	var payload domain.LeaveSessionPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return err
	}
	if err := h.lifecycle.Leave(ctx, client.roomID, payload.SessionID); err != nil {
		return err
	}
	h.removeClient(client)
	return h.broadcast(client.roomID, domain.Envelope{
		Type:      domain.EventPresenceChange,
		RequestID: envelope.RequestID,
		SentAt:    time.Now().UTC(),
		Payload: mustPayload(domain.PresenceChangedPayload{
			SessionID: payload.SessionID,
			Event:     "left",
			Member: domain.PresenceMember{
				SessionID: payload.SessionID,
				UserID:    client.userID,
			},
		}),
	}, nil)
}

func (h *Handler) addClient(client *clientConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.roomID] == nil {
		h.clients[client.roomID] = map[*clientConn]struct{}{}
	}
	h.clients[client.roomID][client] = struct{}{}
}

func (h *Handler) removeClient(client *clientConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if roomClients := h.clients[client.roomID]; roomClients != nil {
		delete(roomClients, client)
		if len(roomClients) == 0 {
			delete(h.clients, client.roomID)
		}
	}
}

func (h *Handler) broadcast(roomID string, envelope domain.Envelope, skip *clientConn) error {
	h.mu.RLock()
	clients := make([]*clientConn, 0, len(h.clients[roomID]))
	for client := range h.clients[roomID] {
		if skip != nil && client == skip {
			continue
		}
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		if err := client.writeEnvelope(envelope); err != nil {
			return err
		}
	}
	return nil
}

func (c *clientConn) writeEnvelope(envelope domain.Envelope) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteJSON(envelope)
}

func mustPayload(value any) json.RawMessage {
	payload, _ := json.Marshal(value)
	return payload
}

func parseRoles(header string) []string {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	return strings.Split(header, ",")
}
