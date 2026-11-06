package policy

import (
	"context"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

// Disable blocks a user and revokes their sessions, so access tokens stop at the
// next request: Bearer checks the session every time.
func Disable(ctx context.Context, uow output.UnitOfWork, id string, now time.Time) error {
	if err := uow.Users().UpdateStatus(ctx, id, domain.UserStatusDisabled); err != nil {
		return err
	}
	return uow.Sessions().RevokeAllForUser(ctx, id, now, domain.RevokeAccountDisabled)
}
