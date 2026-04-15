package domain

type ConflictPayload struct {
	Reason             string   `json:"reason"`
	LatestRevisionNo   int64    `json:"latest_revision_no"`
	LatestRevisionID   string   `json:"latest_revision_id"`
	ServerDocument     Document `json:"server_document"`
	ConflictingPatchID string   `json:"conflicting_patch_id,omitempty"`
}
