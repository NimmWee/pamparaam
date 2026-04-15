package contract_test

import (
	"net/http"
	"testing"
)

func TestAutosaveConflictContract(t *testing.T) {
	pageService, gatewayServer := newGatewayBackedPageStack(t)

	createResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/v1/pages", map[string]any{
		"workspace_id": workspaceID,
		"title":        "Conflict Page",
		"slug":         "conflict-page",
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

	firstSave := performJSONRequestWithHeaders(t, gatewayServer.Client(), http.MethodPatch, gatewayServer.URL+"/api/v1/pages/"+pageID+"/draft", map[string]any{
		"base_revision_no": 1,
		"document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "accepted draft"},
			},
		},
	}, map[string]string{"Idempotency-Key": "save-1"})
	if firstSave.StatusCode != http.StatusOK {
		t.Fatalf("first autosave status = %d", firstSave.StatusCode)
	}

	var saved map[string]any
	decodeBody(t, firstSave, &saved)
	if got := int(saved["accepted_revision_no"].(float64)); got != 2 {
		t.Fatalf("accepted_revision_no = %d, want 2", got)
	}

	conflictResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodPatch, gatewayServer.URL+"/api/v1/pages/"+pageID+"/draft", map[string]any{
		"base_revision_no": 1,
		"document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "stale draft"},
			},
		},
	})
	if conflictResponse.StatusCode != http.StatusConflict {
		t.Fatalf("conflict autosave status = %d, want %d", conflictResponse.StatusCode, http.StatusConflict)
	}

	var conflict map[string]any
	decodeBody(t, conflictResponse, &conflict)
	if got := conflict["reason"].(string); got != "stale_revision" {
		t.Fatalf("reason = %q, want stale_revision", got)
	}
	if got := int(conflict["latest_revision_no"].(float64)); got != 2 {
		t.Fatalf("latest_revision_no = %d, want 2", got)
	}
	serverDocument := conflict["server_document"].(map[string]any)
	blocks := serverDocument["blocks"].([]any)
	firstBlock := blocks[0].(map[string]any)
	if got := firstBlock["text"].(string); got != "accepted draft" {
		t.Fatalf("server_document text = %q, want accepted draft", got)
	}

	idempotentRetry := performJSONRequestWithHeaders(t, gatewayServer.Client(), http.MethodPatch, gatewayServer.URL+"/api/v1/pages/"+pageID+"/draft", map[string]any{
		"base_revision_no": 1,
		"document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "accepted draft"},
			},
		},
	}, map[string]string{"Idempotency-Key": "save-1"})
	if idempotentRetry.StatusCode != http.StatusOK {
		t.Fatalf("idempotent autosave retry status = %d", idempotentRetry.StatusCode)
	}

	var retried map[string]any
	decodeBody(t, idempotentRetry, &retried)
	if retried["accepted_revision_id"].(string) != saved["accepted_revision_id"].(string) {
		t.Fatalf("idempotent retry revision_id = %q, want %q", retried["accepted_revision_id"], saved["accepted_revision_id"])
	}

	_ = pageService
}
