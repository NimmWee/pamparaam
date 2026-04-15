package contract_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func TestBacklinksAndSearchContract(t *testing.T) {
	stack := testsupport.NewUS4Stack(t, testsupport.TokenValidator{})

	targetResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/pages", "editor-token", map[string]any{
		"workspace_id": workspaceID,
		"title":        "Architecture Hub",
		"slug":         "architecture-hub",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "Core architecture notes"},
			},
		},
	})
	if targetResp.StatusCode != http.StatusCreated {
		t.Fatalf("target page status = %d", targetResp.StatusCode)
	}
	var target map[string]any
	decodeBody(t, targetResp, &target)
	targetPageID := target["page_id"].(string)

	relatedResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/pages", "editor-token", map[string]any{
		"workspace_id": workspaceID,
		"title":        "Architecture Playbook",
		"slug":         "architecture-playbook",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-rel", "type": "paragraph", "text": "Detailed playbook"},
			},
		},
	})
	if relatedResp.StatusCode != http.StatusCreated {
		t.Fatalf("related page status = %d", relatedResp.StatusCode)
	}
	var related map[string]any
	decodeBody(t, relatedResp, &related)
	relatedPageID := related["page_id"].(string)

	time.Sleep(10 * time.Millisecond)

	sourceResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/pages", "editor-token", map[string]any{
		"workspace_id": workspaceID,
		"title":        "Roadmap Index",
		"slug":         "roadmap-index",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-body", "type": "paragraph", "text": "This page references the architecture hub."},
				{"id": "blk-link", "type": "page_link", "link": map[string]any{"page_id": targetPageID, "title": "Architecture Hub", "link_kind": "page_ref"}},
				{"id": "blk-link-related", "type": "page_link", "link": map[string]any{"page_id": relatedPageID, "title": "Architecture Playbook", "link_kind": "page_ref"}},
				{"id": "blk-embed", "type": "table_embed", "embed": map[string]any{"mws_table_id": "tbl_123", "title": "Roadmap Metrics"}},
			},
		},
	})
	if sourceResp.StatusCode != http.StatusCreated {
		t.Fatalf("source page status = %d", sourceResp.StatusCode)
	}
	var source map[string]any
	decodeBody(t, sourceResp, &source)
	sourcePageID := source["page_id"].(string)

	testsupport.Eventually(t, 3*time.Second, func() bool {
		results, err := stack.SearchService.Store.Search(context.Background(), workspaceID, "Roadmap", "updated_at")
		if err != nil {
			return false
		}
		for _, result := range results {
			if result.PageID == sourcePageID {
				return true
			}
		}
		return false
	})

	testsupport.Eventually(t, 3*time.Second, func() bool {
		backlinks, err := stack.SearchService.Store.GetBacklinks(context.Background(), workspaceID, targetPageID)
		return err == nil && len(backlinks.Backlinks) == 1
	})

	backlinksResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/pages/"+targetPageID+"/backlinks", "editor-token", nil)
	if backlinksResp.StatusCode != http.StatusOK {
		t.Fatalf("backlinks status = %d", backlinksResp.StatusCode)
	}
	var backlinks map[string]any
	decodeBody(t, backlinksResp, &backlinks)
	backlinkItems := backlinks["backlinks"].([]any)
	if len(backlinkItems) != 1 {
		t.Fatalf("backlinks count = %d, want 1", len(backlinkItems))
	}
	if got := backlinkItems[0].(map[string]any)["page_id"].(string); got != sourcePageID {
		t.Fatalf("backlink page_id = %q, want %q", got, sourcePageID)
	}
	relatedItems := backlinks["related_pages"].([]any)
	if len(relatedItems) == 0 {
		t.Fatalf("related_pages empty")
	}
	foundRelated := false
	for _, item := range relatedItems {
		entry := item.(map[string]any)
		if entry["page_id"].(string) == sourcePageID || entry["page_id"].(string) == relatedPageID {
			foundRelated = true
		}
	}
	if !foundRelated {
		t.Fatalf("expected related page for %s or %s, got %#v", sourcePageID, relatedPageID, relatedItems)
	}

	searchResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/search?workspace_id="+workspaceID+"&q=Roadmap&sort=updated_at", "editor-token", nil)
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("search status = %d", searchResp.StatusCode)
	}
	var search map[string]any
	decodeBody(t, searchResp, &search)
	results := search["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("search results empty")
	}
	first := results[0].(map[string]any)
	if got := first["page_id"].(string); got != sourcePageID {
		t.Fatalf("first result page_id = %q, want %q", got, sourcePageID)
	}
	if got := first["match_type"].(string); got == "" {
		t.Fatalf("match_type missing: %#v", first)
	}

	embedResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/search?workspace_id="+workspaceID+"&q=Metrics", "editor-token", nil)
	if embedResp.StatusCode != http.StatusOK {
		t.Fatalf("embed search status = %d", embedResp.StatusCode)
	}
	var embedSearch map[string]any
	decodeBody(t, embedResp, &embedSearch)
	results = embedSearch["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("embed search results empty")
	}
	foundEmbed := false
	for _, item := range results {
		entry := item.(map[string]any)
		if entry["page_id"].(string) == sourcePageID && entry["match_type"].(string) == "embed_reference" {
			foundEmbed = true
		}
	}
	if !foundEmbed {
		raw, _ := json.Marshal(embedSearch)
		t.Fatalf("expected embed_reference result, got %s", raw)
	}
}
