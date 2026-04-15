package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

const replayWindowTTL = 10 * time.Minute

func recordReplayWindow(ctx context.Context, store ports.ReplayWindowStore, entry domain.ReplayWindowEntry) error {
	if store == nil {
		return nil
	}
	return store.Append(ctx, entry, replayWindowTTL)
}

func buildResumeToken(pageID string, revision domain.PageRevision, entries []domain.ReplayWindowEntry) string {
	token := domain.ResumeToken{
		PageID:       pageID,
		RevisionID:   revision.ID,
		RevisionNo:   revision.RevisionNo,
	}
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		token.ReplayHeadRevisionNo = last.RevisionNo
		token.ReplayHeadPatchID = last.PatchID
	}
	payload, err := json.Marshal(token)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func parseResumeToken(raw string) (domain.ResumeToken, bool) {
	if raw == "" {
		return domain.ResumeToken{}, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return domain.ResumeToken{}, false
	}
	var token domain.ResumeToken
	if err := json.Unmarshal(decoded, &token); err != nil {
		return domain.ResumeToken{}, false
	}
	return token, true
}
