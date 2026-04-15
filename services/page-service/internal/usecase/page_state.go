package usecase

import (
	"fmt"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

func ensurePageMutable(page domain.Page) error {
	if page.Status == domain.PageStatusArchived {
		return fmt.Errorf("%w: archived pages are read-only", ErrPageArchived)
	}
	return nil
}
