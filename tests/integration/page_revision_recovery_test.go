package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	pageapp "github.com/mtc/wiki-editor-backend/services/page-service/app"
)

const recoveryWorkspaceID = "55555555-5555-5555-5555-555555555555"

func TestDraftRecoveryIntegration(t *testing.T) {
	pageService, err := pageapp.NewApplication(pageapp.Config{})
	if err != nil {
		t.Fatalf("new page application: %v", err)
	}
	t.Cleanup(func() { _ = pageService.Close() })

	server := httptest.NewServer(pageService.Handler)
	t.Cleanup(server.Close)

	createResponse := doJSON(t, server.Client(), http.MethodPost, server.URL+"/pages", map[string]any{
		"workspace_id": recoveryWorkspaceID,
		"title":        "Recovery Page",
		"slug":         "recovery-page",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "v1"},
			},
		},
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create page status = %d", createResponse.StatusCode)
	}

	var created map[string]any
	mustDecode(t, createResponse, &created)
	pageID := created["page_id"].(string)

	saveResponse := doJSON(t, server.Client(), http.MethodPatch, server.URL+"/pages/"+pageID+"/draft", map[string]any{
		"base_revision_no": 1,
		"document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "server accepted"},
			},
		},
	})
	if saveResponse.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d", saveResponse.StatusCode)
	}

	conflictResponse := doJSON(t, server.Client(), http.MethodPatch, server.URL+"/pages/"+pageID+"/draft", map[string]any{
		"base_revision_no": 1,
		"document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "stale local state"},
			},
		},
	})
	if conflictResponse.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d", conflictResponse.StatusCode)
	}

	recoveryResponse := doJSON(t, server.Client(), http.MethodGet, server.URL+"/pages/"+pageID+"/draft/recover", nil)
	if recoveryResponse.StatusCode != http.StatusOK {
		t.Fatalf("recovery status = %d", recoveryResponse.StatusCode)
	}

	var recovered map[string]any
	mustDecode(t, recoveryResponse, &recovered)
	if got := int(recovered["accepted_revision_no"].(float64)); got != 2 {
		t.Fatalf("accepted_revision_no = %d, want 2", got)
	}
	document := recovered["document"].(map[string]any)
	blocks := document["blocks"].([]any)
	if got := blocks[0].(map[string]any)["text"].(string); got != "server accepted" {
		t.Fatalf("recovered document text = %q, want server accepted", got)
	}
}
