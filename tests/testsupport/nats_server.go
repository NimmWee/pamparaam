package testsupport

import (
	"fmt"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

func NewNATSServer(t *testing.T) string {
	t.Helper()

	options := &natsserver.Options{
		Host: "127.0.0.1",
		Port: -1,
	}
	server, err := natsserver.NewServer(options)
	if err != nil {
		t.Fatalf("new nats server: %v", err)
	}
	go server.Start()
	if !server.ReadyForConnections(10 * time.Second) {
		t.Fatalf("nats server not ready")
	}
	t.Cleanup(func() { server.Shutdown() })

	return fmt.Sprintf("nats://%s", server.Addr().String())
}
