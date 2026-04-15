package contract_test

import (
	"net/http"
	"testing"
)

func TestEditorMetadataContract(t *testing.T) {
	_, gatewayServer := newGatewayBackedPageStack(t)

	response := performJSONRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/v1/editor/metadata?workspace_id="+workspaceID, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("editor metadata status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var payload map[string]any
	decodeBody(t, response, &payload)

	blockCatalog, ok := payload["block_catalog"].([]any)
	if !ok || len(blockCatalog) == 0 {
		t.Fatalf("block_catalog = %#v, want non-empty array", payload["block_catalog"])
	}
	assertContainsValue(t, blockCatalog, "type", "paragraph")
	assertContainsValue(t, blockCatalog, "type", "table_embed")
	assertContainsValue(t, blockCatalog, "type", "file")

	slashMenuItems, ok := payload["slash_menu_items"].([]any)
	if !ok || len(slashMenuItems) == 0 {
		t.Fatalf("slash_menu_items = %#v, want non-empty array", payload["slash_menu_items"])
	}
	assertContainsValue(t, slashMenuItems, "key", "insert_heading")
	assertContainsValue(t, slashMenuItems, "key", "insert_mws_table")
	assertContainsValue(t, slashMenuItems, "key", "attach_file")

	hotkeys, ok := payload["hotkeys"].([]any)
	if !ok || len(hotkeys) == 0 {
		t.Fatalf("hotkeys = %#v, want non-empty array", payload["hotkeys"])
	}
	assertContainsValue(t, hotkeys, "key", "mod+/")
	assertContainsValue(t, hotkeys, "key", "mod+alt+1")

	capabilities, ok := payload["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("capabilities = %#v, want object", payload["capabilities"])
	}
	assertCapability(t, capabilities, "supports_hotkeys", true)
	assertCapability(t, capabilities, "supports_slash_menu", true)
	assertCapability(t, capabilities, "supports_sync_resume", true)
	assertCapability(t, capabilities, "supports_resume_replay_window", true)
	assertCapability(t, capabilities, "supports_files", true)
	assertCapability(t, capabilities, "supports_embed", true)
	assertCapability(t, capabilities, "supports_mws_tables", true)
	assertCapability(t, capabilities, "supports_publish", true)
	assertCapability(t, capabilities, "supports_restore", true)

	embedCatalog, ok := payload["embed_catalog"].([]any)
	if !ok || len(embedCatalog) == 0 {
		t.Fatalf("embed_catalog = %#v, want non-empty array", payload["embed_catalog"])
	}
	assertContainsValue(t, embedCatalog, "source", "mws_table")
}

func TestEditorMetadataContractFiltersRestrictedCapabilities(t *testing.T) {
	_, gatewayServer := newGatewayBackedPageStackWithRole(t, "viewer")

	response := performJSONRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/v1/editor/metadata?workspace_id="+workspaceID, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("editor metadata status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var payload map[string]any
	decodeBody(t, response, &payload)
	capabilities := payload["capabilities"].(map[string]any)
	assertCapability(t, capabilities, "supports_realtime_collaboration", false)
	assertCapability(t, capabilities, "supports_files", false)
	assertCapability(t, capabilities, "supports_embed", false)
	assertCapability(t, capabilities, "supports_publish", false)
	assertCapability(t, capabilities, "supports_restore", false)

	slashMenuItems := payload["slash_menu_items"].([]any)
	for _, forbidden := range []string{"insert_mws_table", "attach_file"} {
		for _, item := range slashMenuItems {
			entry := item.(map[string]any)
			if entry["key"].(string) == forbidden {
				t.Fatalf("unexpected slash menu item %q for viewer", forbidden)
			}
		}
	}
}

func assertContainsValue(t *testing.T, items []any, field, want string) {
	t.Helper()

	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := entry[field].(string); got == want {
			return
		}
	}

	t.Fatalf("missing %s=%q in %#v", field, want, items)
}

func assertCapability(t *testing.T, capabilities map[string]any, key string, want bool) {
	t.Helper()

	got, ok := capabilities[key].(bool)
	if !ok {
		t.Fatalf("capability %q missing in %#v", key, capabilities)
	}
	if got != want {
		t.Fatalf("capability %q = %t, want %t", key, got, want)
	}
}
