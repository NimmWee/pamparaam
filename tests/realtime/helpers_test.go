package realtime_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func createRealtimePage(t *testing.T, client *http.Client, gatewayURL string) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"workspace_id": testsupport.WorkspaceID,
		"title":        "Realtime Sync",
		"slug":         "realtime-sync",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "base"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal create page: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, gatewayURL+"/api/v1/pages", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-Id", testsupport.WorkspaceID)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create page request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create page status = %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode create page: %v", err)
	}
	return payload["page_id"].(string)
}

func joinSession(t *testing.T, conn *websocket.Conn, pageID string, lastKnownRevisionNo int64, lastKnownPatchID string) (string, testsupport.Envelope, testsupport.Envelope) {
	t.Helper()

	if err := conn.WriteJSON(map[string]any{
		"type":       "join_session",
		"request_id": "req-join",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"page_id":                pageID,
			"workspace_id":           testsupport.WorkspaceID,
			"last_known_revision_no": lastKnownRevisionNo,
			"last_known_patch_id":    lastKnownPatchID,
		},
	}); err != nil {
		t.Fatalf("write join_session: %v", err)
	}

	first := readEnvelopeWithDeadline(t, conn)
	second := readEnvelopeWithDeadline(t, conn)
	if first.Type != "session_joined" {
		t.Fatalf("first join event = %q, want session_joined", first.Type)
	}
	if second.Type != "presence_state" {
		t.Fatalf("second join event = %q, want presence_state", second.Type)
	}

	var joined map[string]any
	if err := json.Unmarshal(first.Payload, &joined); err != nil {
		t.Fatalf("unmarshal session_joined: %v", err)
	}
	return joined["session_id"].(string), first, second
}

func readEnvelopeWithDeadline(t *testing.T, conn *websocket.Conn) testsupport.Envelope {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	return testsupport.ReadEnvelope(t, conn)
}
