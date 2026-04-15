package contract_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	gatewayapp "github.com/mtc/wiki-editor-backend/services/api-gateway/app"
	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func TestSyncResumeContract(t *testing.T) {
	stack := testsupport.NewRealtimeStack(t, gatewayapp.Config{}, 100*time.Millisecond, time.Second)
	pageID := createContractRealtimePage(t, stack.Gateway.Client(), stack.Gateway.URL)

	initialResume := performSyncResumeRequest(t, stack.Gateway.Client(), stack.Gateway.URL, map[string]any{
		"page_id":                pageID,
		"last_known_revision_no": 1,
	})
	if initialResume.StatusCode != http.StatusOK {
		t.Fatalf("initial sync resume status = %d, want %d", initialResume.StatusCode, http.StatusOK)
	}

	var initialPayload map[string]any
	decodeBody(t, initialResume, &initialPayload)
	if got := initialPayload["mode"].(string); got != "resume" {
		t.Fatalf("initial mode = %q, want resume", got)
	}
	initialToken, _ := initialPayload["resume_token"].(string)
	if initialToken == "" {
		t.Fatalf("initial resume_token missing: %#v", initialPayload)
	}

	conn := testsupport.DialCollabSocket(t, stack.Gateway.URL, pageID, testsupport.WorkspaceID, "test-token", nil)
	sessionID := contractJoinSession(t, conn, pageID)
	if err := conn.WriteJSON(map[string]any{
		"type":       "submit_patch",
		"request_id": "req-sync-patch",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"session_id":       sessionID,
			"page_id":          pageID,
			"base_revision_no": 1,
			"patch_id":         "patch-accepted",
			"ops": []map[string]any{
				{"op": "replace_block_text", "block_id": "blk-1", "value": "accepted over websocket"},
			},
		},
	}); err != nil {
		t.Fatalf("write submit_patch: %v", err)
	}

	accepted := testsupport.ReadEnvelope(t, conn)
	if accepted.Type != "patch_accepted" {
		t.Fatalf("patch event = %q, want patch_accepted", accepted.Type)
	}

	replaceResponse := performSyncResumeRequest(t, stack.Gateway.Client(), stack.Gateway.URL, map[string]any{
		"page_id":                pageID,
		"last_known_revision_no": 1,
		"pending_patch_ids":      []string{"patch-accepted", "patch-local-only"},
	})
	if replaceResponse.StatusCode != http.StatusOK {
		t.Fatalf("replace sync resume status = %d, want %d", replaceResponse.StatusCode, http.StatusOK)
	}

	var replacePayload map[string]any
	decodeBody(t, replaceResponse, &replacePayload)
	if got := replacePayload["mode"].(string); got != "replace" {
		t.Fatalf("replace mode = %q, want replace", got)
	}
	if got := int(replacePayload["current_revision_no"].(float64)); got != 2 {
		t.Fatalf("current_revision_no = %d, want 2", got)
	}
	if got := replacePayload["replay_window_patch_ids"].([]any)[0].(string); got != "patch-accepted" {
		t.Fatalf("replay_window_patch_ids[0] = %q, want patch-accepted", got)
	}
	if got := replacePayload["missing_patch_ids"].([]any)[0].(string); got != "patch-local-only" {
		t.Fatalf("missing_patch_ids[0] = %q, want patch-local-only", got)
	}
	document := replacePayload["document"].(map[string]any)
	blocks := document["blocks"].([]any)
	if got := blocks[0].(map[string]any)["text"].(string); got != "accepted over websocket" {
		t.Fatalf("replace document text = %q, want accepted over websocket", got)
	}
	nextToken := replacePayload["resume_token"].(string)
	if nextToken == "" || nextToken == initialToken {
		t.Fatalf("resume_token = %q, want changed non-empty token", nextToken)
	}

	resumeResponse := performSyncResumeRequest(t, stack.Gateway.Client(), stack.Gateway.URL, map[string]any{
		"page_id":                pageID,
		"last_known_revision_no": 2,
		"resume_token":           nextToken,
		"pending_patch_ids":      []string{"patch-local-only"},
	})
	if resumeResponse.StatusCode != http.StatusOK {
		t.Fatalf("resume sync status = %d, want %d", resumeResponse.StatusCode, http.StatusOK)
	}

	var resumed map[string]any
	decodeBody(t, resumeResponse, &resumed)
	if got := resumed["mode"].(string); got != "resume" {
		t.Fatalf("final mode = %q, want resume", got)
	}
	if _, ok := resumed["document"]; ok {
		t.Fatalf("resume payload should omit document: %#v", resumed)
	}
	if len(resumed["missing_patch_ids"].([]any)) != 1 {
		t.Fatalf("missing_patch_ids = %#v, want one entry", resumed["missing_patch_ids"])
	}
}

func createContractRealtimePage(t *testing.T, client *http.Client, gatewayURL string) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"workspace_id": testsupport.WorkspaceID,
		"title":        "Sync Resume",
		"slug":         "sync-resume",
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

func contractJoinSession(t *testing.T, conn *websocket.Conn, pageID string) string {
	t.Helper()

	if err := conn.WriteJSON(map[string]any{
		"type":       "join_session",
		"request_id": "req-join",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"page_id":      pageID,
			"workspace_id": testsupport.WorkspaceID,
		},
	}); err != nil {
		t.Fatalf("write join_session: %v", err)
	}

	joined := testsupport.ReadEnvelope(t, conn)
	if joined.Type != "session_joined" {
		t.Fatalf("join event = %q, want session_joined", joined.Type)
	}
	presence := testsupport.ReadEnvelope(t, conn)
	if presence.Type != "presence_state" {
		t.Fatalf("presence event = %q, want presence_state", presence.Type)
	}

	var payload map[string]any
	if err := json.Unmarshal(joined.Payload, &payload); err != nil {
		t.Fatalf("unmarshal join payload: %v", err)
	}
	return payload["session_id"].(string)
}

func performSyncResumeRequest(t *testing.T, client *http.Client, gatewayURL string, body any) *http.Response {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal sync request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, gatewayURL+"/api/v1/editor/sync", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new sync request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-Id", testsupport.WorkspaceID)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do sync request: %v", err)
	}
	return resp
}
