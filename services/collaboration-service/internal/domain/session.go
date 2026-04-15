package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

const (
	EventJoinSession    = "join_session"
	EventHeartbeat      = "heartbeat"
	EventUpdatePresence = "update_presence"
	EventSubmitPatch    = "submit_patch"
	EventLeaveSession   = "leave_session"

	EventSessionJoined  = "session_joined"
	EventPresenceState  = "presence_state"
	EventPresenceChange = "presence_changed"
	EventPatchAccepted  = "patch_accepted"
	EventPatchRejected  = "patch_rejected"
	EventRebaseRequired = "rebase_required"
	EventPong           = "pong"
	EventError          = "error"
)

type Envelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	SentAt    time.Time       `json:"sent_at,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type Document struct {
	Blocks   []Block        `json:"blocks"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Block struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Text       string         `json:"text,omitempty"`
	Attrs      map[string]any `json:"attrs,omitempty"`
	Embed      map[string]any `json:"embed,omitempty"`
	Link       map[string]any `json:"link,omitempty"`
	Attachment map[string]any `json:"attachment,omitempty"`
}

type PatchOperation struct {
	Op      string          `json:"op"`
	BlockID string          `json:"block_id,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
}

type RoomState struct {
	RoomID            string    `json:"room_id"`
	PageID            string    `json:"page_id"`
	WorkspaceID       string    `json:"workspace_id"`
	CurrentRevisionID string    `json:"current_revision_id"`
	CurrentRevisionNo int64     `json:"current_revision_no"`
	LastPatchID       string    `json:"last_patch_id,omitempty"`
	Document          Document  `json:"document"`
	ExpiresAt         time.Time `json:"expires_at"`
}

type PresenceMember struct {
	RoomID      string     `json:"room_id"`
	SessionID   string     `json:"session_id"`
	UserID      string     `json:"user_id"`
	DisplayName string     `json:"display_name,omitempty"`
	Cursor      *Cursor    `json:"cursor,omitempty"`
	Selection   *Selection `json:"selection,omitempty"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	LastPatchID string     `json:"last_patch_id,omitempty"`
	WorkspaceID string     `json:"workspace_id"`
	PageID      string     `json:"page_id"`
}

type Cursor struct {
	BlockID string `json:"block_id"`
	Offset  int    `json:"offset"`
}

type Selection struct {
	FromBlockID string `json:"from_block_id"`
	ToBlockID   string `json:"to_block_id"`
}

type JoinSessionPayload struct {
	PageID              string `json:"page_id"`
	WorkspaceID         string `json:"workspace_id"`
	LastKnownRevisionNo int64  `json:"last_known_revision_no,omitempty"`
	LastKnownPatchID    string `json:"last_known_patch_id,omitempty"`
}

type SessionJoinedPayload struct {
	SessionID                string   `json:"session_id"`
	PageID                   string   `json:"page_id"`
	WorkspaceID              string   `json:"workspace_id"`
	CurrentRevisionNo        int64    `json:"current_revision_no"`
	CurrentRevisionID        string   `json:"current_revision_id"`
	Document                 Document `json:"document"`
	HeartbeatIntervalSeconds int      `json:"heartbeat_interval_seconds"`
	PresenceTTLSeconds       int      `json:"presence_ttl_seconds"`
}

type PresenceStatePayload struct {
	SessionID string           `json:"session_id"`
	Members   []PresenceMember `json:"members"`
}

type PresenceChangedPayload struct {
	SessionID string         `json:"session_id"`
	Event     string         `json:"event"`
	Member    PresenceMember `json:"member"`
}

type HeartbeatPayload struct {
	SessionID string  `json:"session_id"`
	Cursor    *Cursor `json:"cursor,omitempty"`
}

type UpdatePresencePayload struct {
	SessionID string     `json:"session_id"`
	Cursor    *Cursor    `json:"cursor,omitempty"`
	Selection *Selection `json:"selection,omitempty"`
}

type LeaveSessionPayload struct {
	SessionID string `json:"session_id"`
}

type SubmitPatchPayload struct {
	SessionID      string           `json:"session_id"`
	PageID         string           `json:"page_id"`
	BaseRevisionNo int64            `json:"base_revision_no"`
	PatchID        string           `json:"patch_id"`
	Ops            []PatchOperation `json:"ops"`
}

type PatchAcceptedPayload struct {
	SessionID          string           `json:"session_id"`
	PageID             string           `json:"page_id"`
	AcceptedRevisionNo int64            `json:"accepted_revision_no"`
	AcceptedRevisionID string           `json:"accepted_revision_id"`
	PatchID            string           `json:"patch_id"`
	Ops                []PatchOperation `json:"ops"`
}

type PatchRejectedPayload struct {
	SessionID string         `json:"session_id"`
	PatchID   string         `json:"patch_id"`
	Reason    string         `json:"reason"`
	Details   map[string]any `json:"details,omitempty"`
}

type RebaseRequiredPayload struct {
	SessionID          string   `json:"session_id,omitempty"`
	Reason             string   `json:"reason"`
	LatestRevisionNo   int64    `json:"latest_revision_no"`
	LatestRevisionID   string   `json:"latest_revision_id"`
	ServerDocument     Document `json:"server_document"`
	ConflictingPatchID string   `json:"conflicting_patch_id,omitempty"`
}

type PongPayload struct {
	SessionID  string    `json:"session_id"`
	ReceivedAt time.Time `json:"received_at"`
}

type ErrorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type ValidationError struct {
	BlockID string
	Message string
}

func (e *ValidationError) Error() string {
	if e.BlockID == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.BlockID, e.Message)
}

var ErrSessionNotFound = errors.New("session not found")

func RoomKey(workspaceID, pageID string) string {
	return workspaceID + "::" + pageID
}

func (d Document) Clone() Document {
	blocks := make([]Block, 0, len(d.Blocks))
	for _, block := range d.Blocks {
		blocks = append(blocks, Block{
			ID:         block.ID,
			Type:       block.Type,
			Text:       block.Text,
			Attrs:      cloneMap(block.Attrs),
			Embed:      cloneMap(block.Embed),
			Link:       cloneMap(block.Link),
			Attachment: cloneMap(block.Attachment),
		})
	}
	return Document{
		Blocks:   blocks,
		Metadata: cloneMap(d.Metadata),
	}
}

func ApplyPatch(document Document, ops []PatchOperation) (Document, error) {
	next := document.Clone()
	for _, op := range ops {
		switch op.Op {
		case "replace_block_text":
			index := blockIndex(next.Blocks, op.BlockID)
			if index < 0 {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "target block does not exist"}
			}
			var text string
			if err := json.Unmarshal(op.Value, &text); err != nil {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "value must be a string"}
			}
			next.Blocks[index].Text = text
		case "replace_block_attrs":
			index := blockIndex(next.Blocks, op.BlockID)
			if index < 0 {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "target block does not exist"}
			}
			var attrs map[string]any
			if err := json.Unmarshal(op.Value, &attrs); err != nil {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "value must be an object"}
			}
			next.Blocks[index].Attrs = attrs
		case "replace_embed_config":
			index := blockIndex(next.Blocks, op.BlockID)
			if index < 0 {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "target block does not exist"}
			}
			var value struct {
				DisplayConfig map[string]any `json:"display_config"`
			}
			if err := json.Unmarshal(op.Value, &value); err != nil {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "value must contain display_config"}
			}
			if next.Blocks[index].Embed == nil {
				next.Blocks[index].Embed = map[string]any{}
			}
			next.Blocks[index].Embed["display_config"] = value.DisplayConfig
		case "insert_block":
			var value struct {
				Block        Block  `json:"block"`
				Index        *int   `json:"index,omitempty"`
				AfterBlockID string `json:"after_block_id,omitempty"`
			}
			if err := json.Unmarshal(op.Value, &value); err != nil {
				return Document{}, &ValidationError{Message: "value must contain block"}
			}
			insertAt := len(next.Blocks)
			if value.Index != nil {
				insertAt = *value.Index
			} else if value.AfterBlockID != "" {
				position := blockIndex(next.Blocks, value.AfterBlockID)
				if position < 0 {
					return Document{}, &ValidationError{BlockID: value.AfterBlockID, Message: "after_block_id does not exist"}
				}
				insertAt = position + 1
			}
			if insertAt < 0 || insertAt > len(next.Blocks) {
				return Document{}, &ValidationError{Message: "insert index out of range"}
			}
			next.Blocks = slices.Insert(next.Blocks, insertAt, value.Block)
		case "move_block":
			index := blockIndex(next.Blocks, op.BlockID)
			if index < 0 {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "target block does not exist"}
			}
			var value struct {
				ToIndex int `json:"to_index"`
			}
			if err := json.Unmarshal(op.Value, &value); err != nil {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "value must contain to_index"}
			}
			if value.ToIndex < 0 || value.ToIndex >= len(next.Blocks) {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "to_index out of range"}
			}
			block := next.Blocks[index]
			next.Blocks = append(next.Blocks[:index], next.Blocks[index+1:]...)
			next.Blocks = slices.Insert(next.Blocks, value.ToIndex, block)
		case "delete_block":
			index := blockIndex(next.Blocks, op.BlockID)
			if index < 0 {
				return Document{}, &ValidationError{BlockID: op.BlockID, Message: "target block does not exist"}
			}
			next.Blocks = append(next.Blocks[:index], next.Blocks[index+1:]...)
		default:
			return Document{}, &ValidationError{BlockID: op.BlockID, Message: "unsupported operation"}
		}
	}
	return next, nil
}

func blockIndex(blocks []Block, blockID string) int {
	for index, block := range blocks {
		if block.ID == blockID {
			return index
		}
	}
	return -1
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
