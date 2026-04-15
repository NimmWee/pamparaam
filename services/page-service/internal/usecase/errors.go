package usecase

import (
	"fmt"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

type RebaseRequiredError struct {
	Payload domain.ConflictPayload
}

func (e *RebaseRequiredError) Error() string {
	return fmt.Sprintf("rebase required: latest revision %d", e.Payload.LatestRevisionNo)
}
