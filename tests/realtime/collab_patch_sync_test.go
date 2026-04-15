package realtime_test

import (
	"encoding/json"
	"testing"
	"time"

	gatewayapp "github.com/mtc/wiki-editor-backend/services/api-gateway/app"
	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func TestRealtimeSessionPatchSync(t *testing.T) {
	stack := testsupport.NewRealtimeStack(t, gatewayapp.Config{}, 100*time.Millisecond, time.Second)
	pageID := createRealtimePage(t, stack.Gateway.Client(), stack.Gateway.URL)

	connOne := testsupport.DialCollabSocket(t, stack.Gateway.URL, pageID, testsupport.WorkspaceID, "test-token", nil)
	sessionOne, _, _ := joinSession(t, connOne, pageID, 0, "")

	connTwo := testsupport.DialCollabSocket(t, stack.Gateway.URL, pageID, testsupport.WorkspaceID, "test-token", nil)
	_, _, _ = joinSession(t, connTwo, pageID, 0, "")

	joinedNotice := readEnvelopeWithDeadline(t, connOne)
	if joinedNotice.Type != "presence_changed" {
		t.Fatalf("join broadcast event = %q, want presence_changed", joinedNotice.Type)
	}

	if err := connOne.WriteJSON(map[string]any{
		"type":       "submit_patch",
		"request_id": "req-patch",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"session_id":       sessionOne,
			"page_id":          pageID,
			"base_revision_no": 1,
			"patch_id":         "patch-001",
			"ops": []map[string]any{
				{"op": "replace_block_text", "block_id": "blk-1", "value": "updated realtime"},
			},
		},
	}); err != nil {
		t.Fatalf("write submit_patch: %v", err)
	}

	acceptedOne := readEnvelopeWithDeadline(t, connOne)
	acceptedTwo := readEnvelopeWithDeadline(t, connTwo)
	if acceptedOne.Type != "patch_accepted" {
		t.Fatalf("client one patch event = %q, want patch_accepted", acceptedOne.Type)
	}
	if acceptedTwo.Type != "patch_accepted" {
		t.Fatalf("client two patch event = %q, want patch_accepted", acceptedTwo.Type)
	}

	var payload map[string]any
	if err := json.Unmarshal(acceptedTwo.Payload, &payload); err != nil {
		t.Fatalf("unmarshal patch_accepted: %v", err)
	}
	if got := int(payload["accepted_revision_no"].(float64)); got != 2 {
		t.Fatalf("accepted_revision_no = %d, want 2", got)
	}
	if got := payload["patch_id"].(string); got != "patch-001" {
		t.Fatalf("patch_id = %q, want patch-001", got)
	}
}
