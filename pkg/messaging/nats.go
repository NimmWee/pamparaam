package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSConfig holds the minimum connection settings for service startup.
type NATSConfig struct {
	URL           string
	Name          string
	ReconnectWait time.Duration
	MaxReconnects int
}

// NATSPublisher adapts a NATS connection to the generic outbox Publisher interface.
type NATSPublisher struct {
	Conn *nats.Conn
}

// Connect creates a scoped NATS client that can be reused by services later.
func Connect(ctx context.Context, cfg NATSConfig) (*nats.Conn, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("nats url is required")
	}

	options := []nats.Option{
		nats.Name(cfg.Name),
		nats.Timeout(5 * time.Second),
	}
	if cfg.ReconnectWait > 0 {
		options = append(options, nats.ReconnectWait(cfg.ReconnectWait))
	}
	if cfg.MaxReconnects > 0 {
		options = append(options, nats.MaxReconnects(cfg.MaxReconnects))
	}

	conn, err := nats.Connect(cfg.URL, options...)
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	return conn, nil
}

// PublishJSON marshals and publishes an event payload on the requested subject.
func PublishJSON(conn *nats.Conn, subject string, payload any, headers map[string]string) error {
	if conn == nil {
		return fmt.Errorf("nats connection is nil")
	}
	if subject == "" {
		return fmt.Errorf("subject is required")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := nats.NewMsg(subject)
	msg.Data = body
	for key, value := range headers {
		msg.Header.Set(key, value)
	}

	return conn.PublishMsg(msg)
}

// Publish sends a pre-serialized payload to the configured subject.
func (p NATSPublisher) Publish(_ context.Context, subject string, payload []byte, headers map[string]string) error {
	if p.Conn == nil {
		return fmt.Errorf("nats connection is nil")
	}

	msg := nats.NewMsg(subject)
	msg.Data = payload
	for key, value := range headers {
		msg.Header.Set(key, value)
	}

	if err := p.Conn.PublishMsg(msg); err != nil {
		return err
	}
	return p.Conn.FlushTimeout(2 * time.Second)
}
