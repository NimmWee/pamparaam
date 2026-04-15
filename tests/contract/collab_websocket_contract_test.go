package contract_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	gatewayapp "github.com/mtc/wiki-editor-backend/services/api-gateway/app"
	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func TestCollabWebsocketBootstrapAndPatchContract(t *testing.T) {
	stack := testsupport.NewRealtimeStack(t, gatewayapp.Config{}, 100*time.Millisecond, time.Second)

	createResponse := performJSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/pages", map[string]any{
		"workspace_id": workspaceID,
		"title":        "Realtime Page",
		"slug":         "realtime-page",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "base"},
			},
		},
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create page status = %d", createResponse.StatusCode)
	}

	var created map[string]any
	decodeBody(t, createResponse, &created)
	pageID := created["page_id"].(string)

	conn := testsupport.DialCollabSocket(t, stack.Gateway.URL, pageID, workspaceID, "test-token", nil)
	if err := conn.WriteJSON(map[string]any{
		"type":       "join_session",
		"request_id": "req-join",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"page_id":      pageID,
			"workspace_id": workspaceID,
		},
	}); err != nil {
		t.Fatalf("write join_session: %v", err)
	}

	joined := testsupport.ReadEnvelope(t, conn)
	if joined.Type != "session_joined" {
		t.Fatalf("first event type = %q, want session_joined", joined.Type)
	}
	var joinedPayload map[string]any
	if err := json.Unmarshal(joined.Payload, &joinedPayload); err != nil {
		t.Fatalf("unmarshal session_joined: %v", err)
	}
	sessionID := joinedPayload["session_id"].(string)
	if got := int(joinedPayload["current_revision_no"].(float64)); got != 1 {
		t.Fatalf("current_revision_no = %d, want 1", got)
	}

	presence := testsupport.ReadEnvelope(t, conn)
	if presence.Type != "presence_state" {
		t.Fatalf("second event type = %q, want presence_state", presence.Type)
	}

	if err := conn.WriteJSON(map[string]any{
		"type":       "submit_patch",
		"request_id": "req-patch",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"session_id":       sessionID,
			"page_id":          pageID,
			"base_revision_no": 1,
			"patch_id":         "patch-001",
			"ops": []map[string]any{
				{"op": "replace_block_text", "block_id": "blk-1", "value": "updated by contract"},
			},
		},
	}); err != nil {
		t.Fatalf("write submit_patch: %v", err)
	}

	accepted := testsupport.ReadEnvelope(t, conn)
	if accepted.Type != "patch_accepted" {
		t.Fatalf("patch event type = %q, want patch_accepted", accepted.Type)
	}
	var acceptedPayload map[string]any
	if err := json.Unmarshal(accepted.Payload, &acceptedPayload); err != nil {
		t.Fatalf("unmarshal patch_accepted: %v", err)
	}
	if got := int(acceptedPayload["accepted_revision_no"].(float64)); got != 2 {
		t.Fatalf("accepted_revision_no = %d, want 2", got)
	}
}
