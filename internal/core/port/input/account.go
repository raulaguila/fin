package input

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// AccountService is what a logged in user does with their own account. Every
// method acts on userID only, which callers take from the verified token.
type AccountService interface {
	Profile(ctx context.Context, userID string) (*domain.Profile, error)

	Sessions(ctx context.Context, userID string) ([]domain.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error
	RevokeOtherSessions(ctx context.Context, userID, currentSessionID string) error

	Preferences(ctx context.Context, userID string) (*domain.Preferences, error)
	UpdatePreferences(ctx context.Context, p domain.Preferences) (*domain.Preferences, error)

	Notifications(ctx context.Context, userID string, page domain.Page) ([]domain.Notification, int64, error)
	CountUnread(ctx context.Context, userID string) (int64, error)
	MarkRead(ctx context.Context, userID, notificationID string) error
}
