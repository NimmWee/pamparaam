package adapters

import (
	"context"

	"github.com/mtc/wiki-editor-backend/pkg/messaging"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/usecase"
	"github.com/nats-io/nats.go"
)

type PageEventConsumer struct {
	projector *usecase.PageEventProjector
}

func NewPageEventConsumer(projector *usecase.PageEventProjector) *PageEventConsumer {
	return &PageEventConsumer{projector: projector}
}

func (c *PageEventConsumer) Subscribe(ctx context.Context, conn *nats.Conn) error {
	if conn == nil || c == nil || c.projector == nil {
		return nil
	}

	subscription, err := conn.SubscribeSync("page.>")
	if err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}

	go func() {
		for {
			message, err := subscription.NextMsgWithContext(ctx)
			if err != nil {
				return
			}

			headers := make(map[string]string, len(message.Header))
			for key := range message.Header {
				headers[key] = message.Header.Get(key)
			}

			_ = c.projector.Project(ctx, messaging.OutboxMessage{
				ID:      message.Header.Get("X-Event-Id"),
				Subject: message.Subject,
				Payload: message.Data,
				Headers: headers,
			})
		}
	}()

	return nil
}
