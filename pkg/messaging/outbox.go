package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// OutboxMessage is the storage-neutral representation of a pending domain event.
type OutboxMessage struct {
	ID          string
	Subject     string
	Payload     json.RawMessage
	Headers     map[string]string
	Attempts    int
	LastError   string
	OccurredAt  time.Time
	AvailableAt time.Time
}

// OutboxStore abstracts claiming and updating pending outbox messages.
type OutboxStore interface {
	ClaimPending(ctx context.Context, batchSize int) ([]OutboxMessage, error)
	MarkPublished(ctx context.Context, id string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id string, lastError string, nextAttemptAt time.Time) error
}

// Publisher is the minimal capability required by the relay.
type Publisher interface {
	Publish(ctx context.Context, subject string, payload []byte, headers map[string]string) error
}

// OutboxRelay pulls messages from a store and publishes them with bounded retries.
type OutboxRelay struct {
	Store       OutboxStore
	Publisher   Publisher
	Policy      RetryPolicy
	PollEvery   time.Duration
	BatchSize   int
	OnPublished func(message OutboxMessage)
	OnFailed    func(message OutboxMessage, err error)
}

// Run starts the polling loop until the context is cancelled.
func (r *OutboxRelay) Run(ctx context.Context) error {
	if r.Store == nil {
		return errors.New("outbox store is required")
	}
	if r.Publisher == nil {
		return errors.New("outbox publisher is required")
	}
	if r.PollEvery <= 0 {
		r.PollEvery = time.Second
	}
	if r.BatchSize <= 0 {
		r.BatchSize = 25
	}

	ticker := time.NewTicker(r.PollEvery)
	defer ticker.Stop()

	for {
		if err := r.flushOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *OutboxRelay) flushOnce(ctx context.Context) error {
	messages, err := r.Store.ClaimPending(ctx, r.BatchSize)
	if err != nil {
		return err
	}

	for _, message := range messages {
		publishErr := Retry(ctx, r.Policy, func(retryCtx context.Context) error {
			return r.Publisher.Publish(retryCtx, message.Subject, message.Payload, message.Headers)
		})
		if publishErr != nil {
			nextAttemptAt := time.Now().Add(r.Policy.Backoff)
			if nextAttemptAt.IsZero() {
				nextAttemptAt = time.Now().Add(time.Second)
			}
			if err := r.Store.MarkFailed(ctx, message.ID, publishErr.Error(), nextAttemptAt); err != nil {
				return err
			}
			if r.OnFailed != nil {
				r.OnFailed(message, publishErr)
			}
			continue
		}

		if err := r.Store.MarkPublished(ctx, message.ID, time.Now().UTC()); err != nil {
			return err
		}
		if r.OnPublished != nil {
			r.OnPublished(message)
		}
	}

	return nil
}
