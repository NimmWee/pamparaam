package domain

import "time"

type PageProjection struct {
	PageID         string
	WorkspaceID    string
	Title          string
	SearchableText string
	LinkTitles     []string
	EmbedTitles    []string
	UpdatedAt      time.Time
}

type BacklinkReference struct {
	PageID         string `json:"page_id"`
	Title          string `json:"title"`
	Relation       string `json:"relation"`
	MatchedBlockID string `json:"matched_block_id,omitempty"`
}

type SearchResult struct {
	PageID    string    `json:"page_id"`
	Title     string    `json:"title"`
	MatchType string    `json:"match_type"`
	Snippet   string    `json:"snippet"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BacklinksPayload struct {
	PageID       string              `json:"page_id"`
	Backlinks    []BacklinkReference `json:"backlinks"`
	RelatedPages []BacklinkReference `json:"related_pages"`
}

type SearchPayload struct {
	Results []SearchResult `json:"results"`
}
