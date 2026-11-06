package output

import (
	"context"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// PreferenceRepository persists usr.usr_preference.
type PreferenceRepository interface {
	// Find answers not found for a user who never saved preferences.
	Find(ctx context.Context, userID string) (*domain.Preferences, error)
	Save(ctx context.Context, p *domain.Preferences) error
}

// NotificationRepository persists usr.usr_notification. Reads and writes always
// filter by owner, so a notification id alone reaches nothing.
type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	// List returns one page, newest first, and the count of all the user's notifications.
	List(ctx context.Context, userID string, page domain.Page) ([]domain.Notification, int64, error)
	CountUnread(ctx context.Context, userID string) (int64, error)
	// MarkRead is idempotent. False means this user has no such notification.
	MarkRead(ctx context.Context, id, userID string, now time.Time) (bool, error)
}
