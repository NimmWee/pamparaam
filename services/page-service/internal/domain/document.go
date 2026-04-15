package domain

import "time"

type BlockType string

const (
	BlockTypeParagraph  BlockType = "paragraph"
	BlockTypeHeading    BlockType = "heading"
	BlockTypeChecklist  BlockType = "checklist"
	BlockTypeQuote      BlockType = "quote"
	BlockTypeCode       BlockType = "code"
	BlockTypeTableEmbed BlockType = "table_embed"
	BlockTypePageLink   BlockType = "page_link"
	BlockTypeImage      BlockType = "image"
	BlockTypeFile       BlockType = "file"
)

type Document struct {
	Blocks   []DocumentBlock `json:"blocks"`
	Metadata map[string]any  `json:"metadata,omitempty"`
}

type DocumentBlock struct {
	ID         string                `json:"id"`
	Type       BlockType             `json:"type"`
	Text       string                `json:"text,omitempty"`
	Attrs      map[string]any        `json:"attrs,omitempty"`
	Embed      *TableEmbedDescriptor `json:"embed,omitempty"`
	Link       *PageLinkReference    `json:"link,omitempty"`
	Attachment *AttachmentReference  `json:"attachment,omitempty"`
}

type TableEmbedDescriptor struct {
	MWSTableID    string           `json:"mws_table_id"`
	Title         string           `json:"title,omitempty"`
	DisplayConfig map[string]any   `json:"display_config,omitempty"`
	PreviewState  PreviewState     `json:"preview_state,omitempty"`
	Schema        map[string]any   `json:"schema,omitempty"`
	PreviewRows   []map[string]any `json:"preview_rows,omitempty"`
}

type PageLinkReference struct {
	PageID   string `json:"page_id"`
	Title    string `json:"title,omitempty"`
	LinkKind string `json:"link_kind,omitempty"`
}

type AttachmentReference struct {
	FileID   string `json:"file_id"`
	BlockID  string `json:"block_id,omitempty"`
	Filename string `json:"filename,omitempty"`
}

type DocumentReferences struct {
	EmbeddedTables []EmbeddedTableReference
	Attachments    []AttachmentReferenceRecord
	PageLinks      []PageLinkRecord
}

func (d Document) CanonicalSnapshot() Document {
	blocks := make([]DocumentBlock, 0, len(d.Blocks))
	for _, block := range d.Blocks {
		canonical := block
		if block.Embed != nil {
			canonical.Embed = &TableEmbedDescriptor{
				MWSTableID:    block.Embed.MWSTableID,
				Title:         block.Embed.Title,
				DisplayConfig: cloneMap(block.Embed.DisplayConfig),
			}
		}
		blocks = append(blocks, canonical)
	}

	return Document{
		Blocks:   blocks,
		Metadata: cloneMap(d.Metadata),
	}
}

func (d Document) ExtractReferences(pageID, revisionID string, now time.Time, nextID func() string) DocumentReferences {
	references := DocumentReferences{}
	for _, block := range d.Blocks {
		if block.Embed != nil && block.Embed.MWSTableID != "" {
			references.EmbeddedTables = append(references.EmbeddedTables, EmbeddedTableReference{
				ID:             nextID(),
				PageRevisionID: revisionID,
				PageID:         pageID,
				BlockID:        block.ID,
				MWSTableID:     block.Embed.MWSTableID,
				DisplayConfig:  cloneMap(block.Embed.DisplayConfig),
				CreatedAt:      now,
			})
		}
		if block.Attachment != nil && block.Attachment.FileID != "" {
			references.Attachments = append(references.Attachments, AttachmentReferenceRecord{
				ID:             nextID(),
				PageRevisionID: revisionID,
				PageID:         pageID,
				BlockID:        block.ID,
				FileID:         block.Attachment.FileID,
				CreatedAt:      now,
			})
		}
		if block.Link != nil && block.Link.PageID != "" {
			references.PageLinks = append(references.PageLinks, PageLinkRecord{
				ID:             nextID(),
				PageRevisionID: revisionID,
				SourcePageID:   pageID,
				TargetPageID:   block.Link.PageID,
				BlockID:        block.ID,
				LinkKind:       firstNonEmpty(block.Link.LinkKind, "page_ref"),
				CreatedAt:      now,
			})
		}
	}
	return references
}

func (d Document) HydrateEmbeds(descriptors map[string]TableEmbedDescriptor) Document {
	blocks := make([]DocumentBlock, 0, len(d.Blocks))
	for _, block := range d.Blocks {
		hydrated := block
		if descriptor, ok := descriptors[block.ID]; ok {
			copy := descriptor
			hydrated.Embed = &copy
		}
		blocks = append(blocks, hydrated)
	}
	return Document{
		Blocks:   blocks,
		Metadata: cloneMap(d.Metadata),
	}
}

func (d Document) EmbedTitleByBlockID(blockID string) string {
	for _, block := range d.Blocks {
		if block.ID == blockID && block.Embed != nil {
			return block.Embed.Title
		}
	}
	return ""
}

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
