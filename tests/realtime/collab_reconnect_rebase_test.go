package realtime_test

import (
	"encoding/json"
	"testing"
	"time"

	gatewayapp "github.com/mtc/wiki-editor-backend/services/api-gateway/app"
	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func TestRealtimeReconnectAndRebaseRecovery(t *testing.T) {
	stack := testsupport.NewRealtimeStack(t, gatewayapp.Config{}, 100*time.Millisecond, time.Second)
	pageID := createRealtimePage(t, stack.Gateway.Client(), stack.Gateway.URL)

	conn := testsupport.DialCollabSocket(t, stack.Gateway.URL, pageID, testsupport.WorkspaceID, "test-token", nil)
	sessionID, _, _ := joinSession(t, conn, pageID, 0, "")

	if err := conn.WriteJSON(map[string]any{
		"type":       "submit_patch",
		"request_id": "req-patch",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"session_id":       sessionID,
			"page_id":          pageID,
			"base_revision_no": 1,
			"patch_id":         "patch-accepted",
			"ops": []map[string]any{
				{"op": "replace_block_text", "block_id": "blk-1", "value": "server head"},
			},
		},
	}); err != nil {
		t.Fatalf("write accepted patch: %v", err)
	}

	accepted := readEnvelopeWithDeadline(t, conn)
	if accepted.Type != "patch_accepted" {
		t.Fatalf("accepted event = %q, want patch_accepted", accepted.Type)
	}

	if err := conn.WriteJSON(map[string]any{
		"type":       "submit_patch",
		"request_id": "req-stale",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"session_id":       sessionID,
			"page_id":          pageID,
			"base_revision_no": 1,
			"patch_id":         "patch-stale",
			"ops": []map[string]any{
				{"op": "replace_block_text", "block_id": "blk-1", "value": "stale"},
			},
		},
	}); err != nil {
		t.Fatalf("write stale patch: %v", err)
	}

	rebase := readEnvelopeWithDeadline(t, conn)
	if rebase.Type != "rebase_required" {
		t.Fatalf("stale patch event = %q, want rebase_required", rebase.Type)
	}
	var rebasePayload map[string]any
	if err := json.Unmarshal(rebase.Payload, &rebasePayload); err != nil {
		t.Fatalf("unmarshal rebase_required: %v", err)
	}
	if got := int(rebasePayload["latest_revision_no"].(float64)); got != 2 {
		t.Fatalf("latest_revision_no = %d, want 2", got)
	}

	reconnect := testsupport.DialCollabSocket(t, stack.Gateway.URL, pageID, testsupport.WorkspaceID, "test-token", nil)
	if err := reconnect.WriteJSON(map[string]any{
		"type":       "join_session",
		"request_id": "req-rejoin",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"page_id":                pageID,
			"workspace_id":           testsupport.WorkspaceID,
			"last_known_revision_no": 1,
			"last_known_patch_id":    "patch-stale",
		},
	}); err != nil {
		t.Fatalf("write reconnect join_session: %v", err)
	}

	rejoinEnvelope := readEnvelopeWithDeadline(t, reconnect)
	if rejoinEnvelope.Type != "rebase_required" {
		t.Fatalf("reconnect event = %q, want rebase_required", rejoinEnvelope.Type)
	}
	var rejoinPayload map[string]any
	if err := json.Unmarshal(rejoinEnvelope.Payload, &rejoinPayload); err != nil {
		t.Fatalf("unmarshal reconnect rebase_required: %v", err)
	}
	document := rejoinPayload["server_document"].(map[string]any)
	blocks := document["blocks"].([]any)
	if got := blocks[0].(map[string]any)["text"].(string); got != "server head" {
		t.Fatalf("server_document text = %q, want server head", got)
	}
}
