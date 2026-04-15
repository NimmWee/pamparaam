package domain

import "time"

type ReplayEventKind string

const (
	ReplayEventCollabPatch ReplayEventKind = "collab_patch"
	ReplayEventAutosave    ReplayEventKind = "rest_autosave"
	ReplayEventPublish     ReplayEventKind = "publish"
	ReplayEventRestore     ReplayEventKind = "restore"
)

type ReplayWindowEntry struct {
	PageID       string          `json:"page_id"`
	RevisionID   string          `json:"revision_id"`
	RevisionNo   int64           `json:"revision_no"`
	Kind         ReplayEventKind `json:"kind"`
	PatchID      string          `json:"patch_id,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	WorkspaceID  string          `json:"workspace_id,omitempty"`
	ActorUserID  string          `json:"actor_user_id,omitempty"`
}

type ResumeToken struct {
	PageID                string `json:"page_id"`
	RevisionID            string `json:"revision_id"`
	RevisionNo            int64  `json:"revision_no"`
	ReplayHeadRevisionNo  int64  `json:"replay_head_revision_no"`
	ReplayHeadPatchID     string `json:"replay_head_patch_id,omitempty"`
}

