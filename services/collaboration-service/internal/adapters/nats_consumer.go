package adapters

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/usecase"
	"github.com/nats-io/nats.go"
)

type RevisionRefreshConsumer struct {
	lifecycle *usecase.SessionLifecycle
}

func NewRevisionRefreshConsumer(lifecycle *usecase.SessionLifecycle) *RevisionRefreshConsumer {
	return &RevisionRefreshConsumer{lifecycle: lifecycle}
}

func (c *RevisionRefreshConsumer) Subscribe(ctx context.Context, conn *nats.Conn) error {
	if conn == nil {
		return nil
	}
	_, err := conn.Subscribe("page.draft.saved", func(message *nats.Msg) {
		c.refresh(ctx, message)
	})
	if err != nil {
		return err
	}
	_, err = conn.Subscribe("page.published", func(message *nats.Msg) {
		c.refresh(ctx, message)
	})
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (c *RevisionRefreshConsumer) refresh(ctx context.Context, message *nats.Msg) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	var payload struct {
		PageID      string `json:"page_id"`
		WorkspaceID string `json:"workspace_id"`
	}
	if err := json.Unmarshal(message.Data, &payload); err != nil {
		return
	}
	if payload.PageID == "" || payload.WorkspaceID == "" {
		return
	}
	_ = c.lifecycle.RefreshExistingRoom(ctx, payload.PageID, payload.WorkspaceID, 45*time.Second)
}
