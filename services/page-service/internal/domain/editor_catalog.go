package domain

type EditorBlockCatalogEntry struct {
	Type                  BlockType `json:"type"`
	Label                 string    `json:"label"`
	SupportsChildren      bool      `json:"supports_children"`
	SupportsCollaboration bool      `json:"supports_collaboration"`
}

type SlashMenuItem struct {
	Key            string    `json:"key"`
	Label          string    `json:"label"`
	Category       string    `json:"category"`
	BlockType      BlockType `json:"block_type"`
	SupportsHotkey bool      `json:"supports_hotkey"`
}

type HotkeyDefinition struct {
	Key         string `json:"key"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

type EmbedCatalogEntry struct {
	Source  string `json:"source"`
	Label   string `json:"label"`
	Allowed bool   `json:"allowed"`
}

type EditorMetadataPayload struct {
	BlockCatalog   []EditorBlockCatalogEntry `json:"block_catalog"`
	SlashMenuItems []SlashMenuItem           `json:"slash_menu_items"`
	Hotkeys        []HotkeyDefinition        `json:"hotkeys"`
	Capabilities   map[string]bool           `json:"capabilities"`
	EmbedCatalog   []EmbedCatalogEntry       `json:"embed_catalog"`
}

type EditorCatalogOptions struct {
	CanEdit                      bool
	CanUploadFiles               bool
	CanEmbedTables               bool
	CanCollaborate               bool
	CanPublish                   bool
	CanRestore                   bool
	SupportsRealtimeCollab       bool
	SupportsSyncResumeReplay     bool
	SupportsFilesIntegration     bool
	SupportsEmbedIntegration     bool
}

func BuildEditorMetadata(options EditorCatalogOptions) EditorMetadataPayload {
	blockCatalog := []EditorBlockCatalogEntry{
		{Type: BlockTypeParagraph, Label: "Paragraph", SupportsChildren: false, SupportsCollaboration: true},
		{Type: BlockTypeHeading, Label: "Heading", SupportsChildren: false, SupportsCollaboration: true},
		{Type: BlockTypeChecklist, Label: "Checklist", SupportsChildren: true, SupportsCollaboration: true},
		{Type: BlockTypeQuote, Label: "Quote", SupportsChildren: false, SupportsCollaboration: true},
		{Type: BlockTypeCode, Label: "Code block", SupportsChildren: false, SupportsCollaboration: false},
		{Type: BlockTypePageLink, Label: "Page link", SupportsChildren: false, SupportsCollaboration: true},
	}
	if options.CanEmbedTables && options.SupportsEmbedIntegration {
		blockCatalog = append(blockCatalog, EditorBlockCatalogEntry{
			Type:                  BlockTypeTableEmbed,
			Label:                 "MWS table",
			SupportsChildren:      false,
			SupportsCollaboration: false,
		})
	}
	if options.CanUploadFiles && options.SupportsFilesIntegration {
		blockCatalog = append(blockCatalog,
			EditorBlockCatalogEntry{Type: BlockTypeImage, Label: "Image", SupportsChildren: false, SupportsCollaboration: false},
			EditorBlockCatalogEntry{Type: BlockTypeFile, Label: "File", SupportsChildren: false, SupportsCollaboration: false},
		)
	}

	slashMenuItems := []SlashMenuItem{
		{Key: "insert_paragraph", Label: "Paragraph", Category: "basic", BlockType: BlockTypeParagraph, SupportsHotkey: false},
		{Key: "insert_heading", Label: "Heading", Category: "basic", BlockType: BlockTypeHeading, SupportsHotkey: true},
		{Key: "insert_checklist", Label: "Checklist", Category: "basic", BlockType: BlockTypeChecklist, SupportsHotkey: true},
		{Key: "insert_quote", Label: "Quote", Category: "basic", BlockType: BlockTypeQuote, SupportsHotkey: false},
		{Key: "insert_code", Label: "Code block", Category: "basic", BlockType: BlockTypeCode, SupportsHotkey: false},
		{Key: "insert_page_link", Label: "Page link", Category: "references", BlockType: BlockTypePageLink, SupportsHotkey: false},
	}
	if options.CanEmbedTables && options.SupportsEmbedIntegration {
		slashMenuItems = append(slashMenuItems, SlashMenuItem{
			Key:            "insert_mws_table",
			Label:          "MWS table",
			Category:       "embeds",
			BlockType:      BlockTypeTableEmbed,
			SupportsHotkey: false,
		})
	}
	if options.CanUploadFiles && options.SupportsFilesIntegration {
		slashMenuItems = append(slashMenuItems,
			SlashMenuItem{Key: "insert_image", Label: "Image", Category: "media", BlockType: BlockTypeImage, SupportsHotkey: false},
			SlashMenuItem{Key: "attach_file", Label: "File", Category: "media", BlockType: BlockTypeFile, SupportsHotkey: false},
		)
	}

	hotkeys := []HotkeyDefinition{
		{Key: "mod+/", Action: "open_slash_menu", Description: "Open slash menu"},
		{Key: "mod+alt+1", Action: "insert_heading", Description: "Insert heading"},
		{Key: "mod+shift+7", Action: "insert_checklist", Description: "Insert checklist"},
	}
	if options.CanPublish {
		hotkeys = append(hotkeys, HotkeyDefinition{Key: "mod+shift+p", Action: "publish_page", Description: "Publish current draft"})
	}

	return EditorMetadataPayload{
		BlockCatalog:   blockCatalog,
		SlashMenuItems: slashMenuItems,
		Hotkeys:        hotkeys,
		Capabilities: map[string]bool{
			"supports_slash_menu":             true,
			"supports_hotkeys":                true,
			"supports_sync_resume":            true,
			"supports_resume_replay_window":   options.SupportsSyncResumeReplay,
			"supports_realtime_collaboration": options.CanCollaborate && options.SupportsRealtimeCollab,
			"supports_files":                  options.CanUploadFiles && options.SupportsFilesIntegration,
			"supports_embed":                  options.CanEmbedTables && options.SupportsEmbedIntegration,
			"supports_mws_tables":             options.CanEmbedTables && options.SupportsEmbedIntegration,
			"supports_publish":                options.CanPublish,
			"supports_restore":                options.CanRestore,
		},
		EmbedCatalog: []EmbedCatalogEntry{
			{Source: "mws_table", Label: "MWS Table", Allowed: options.CanEmbedTables && options.SupportsEmbedIntegration},
		},
	}
}

type EditorSyncResumePayload struct {
	PageID               string    `json:"page_id"`
	Mode                 string    `json:"mode"`
	CurrentRevisionNo    int64     `json:"current_revision_no"`
	CurrentRevisionID    string    `json:"current_revision_id"`
	Document             *Document `json:"document,omitempty"`
	MissingPatchIDs      []string  `json:"missing_patch_ids,omitempty"`
	ReplayWindowPatchIDs []string  `json:"replay_window_patch_ids,omitempty"`
	ResumeToken          string    `json:"resume_token"`
}
