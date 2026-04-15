package contract_test

import (
	"net/http"
	"testing"
)

func TestPublishRestoreContract(t *testing.T) {
	_, gatewayServer := newGatewayBackedPageStack(t)

	createResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/v1/pages", map[string]any{
		"workspace_id": workspaceID,
		"title":        "Release Notes",
		"slug":         "release-notes",
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
	decodeBody(t, createResponse, &created)
	pageID := created["page_id"].(string)

	saveResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodPatch, gatewayServer.URL+"/api/v1/pages/"+pageID+"/draft", map[string]any{
		"base_revision_no": 1,
		"document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "v2 draft"},
			},
		},
	})
	if saveResponse.StatusCode != http.StatusOK {
		t.Fatalf("autosave status = %d", saveResponse.StatusCode)
	}

	publishResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/v1/pages/"+pageID+"/publish", map[string]any{
		"base_revision_no": 2,
	})
	if publishResponse.StatusCode != http.StatusOK {
		t.Fatalf("publish status = %d", publishResponse.StatusCode)
	}

	var published map[string]any
	decodeBody(t, publishResponse, &published)
	if got := int(published["published_revision_no"].(float64)); got != 3 {
		t.Fatalf("published_revision_no = %d, want 3", got)
	}
	publishedRevisionID := published["published_revision_id"].(string)

	versionsResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/v1/pages/"+pageID+"/versions", nil)
	if versionsResponse.StatusCode != http.StatusOK {
		t.Fatalf("versions status = %d", versionsResponse.StatusCode)
	}

	var versions map[string]any
	decodeBody(t, versionsResponse, &versions)
	revisions := versions["revisions"].([]any)
	if len(revisions) != 3 {
		t.Fatalf("version count = %d, want 3", len(revisions))
	}

	restoreResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/v1/pages/"+pageID+"/versions/"+publishedRevisionID+"/restore", nil)
	if restoreResponse.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d", restoreResponse.StatusCode)
	}

	var restored map[string]any
	decodeBody(t, restoreResponse, &restored)
	if got := int(restored["accepted_revision_no"].(float64)); got != 4 {
		t.Fatalf("restored revision no = %d, want 4", got)
	}
	document := restored["document"].(map[string]any)
	blocks := document["blocks"].([]any)
	if got := blocks[0].(map[string]any)["text"].(string); got != "v2 draft" {
		t.Fatalf("restored document text = %q, want v2 draft", got)
	}

	pageResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/v1/pages/"+pageID+"?view=draft", nil)
	if pageResponse.StatusCode != http.StatusOK {
		t.Fatalf("get page status = %d", pageResponse.StatusCode)
	}
	var page map[string]any
	decodeBody(t, pageResponse, &page)
	if got := int(page["current_draft_revision_no"].(float64)); got != 4 {
		t.Fatalf("current_draft_revision_no = %d, want 4", got)
	}
	if got := int(page["current_published_revision_no"].(float64)); got != 3 {
		t.Fatalf("current_published_revision_no = %d, want 3", got)
	}
}
