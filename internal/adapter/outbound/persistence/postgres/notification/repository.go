package notification

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

type Repository struct {
	db *gorm.DB
}

var _ output.NotificationRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type row struct {
	ID        uuid.UUID  `gorm:"column:id"`
	UserID    uuid.UUID  `gorm:"column:user_id"`
	Level     string     `gorm:"column:level"`
	Title     string     `gorm:"column:title"`
	Message   string     `gorm:"column:message"`
	ReadAt    *time.Time `gorm:"column:read_at"`
	CreatedAt time.Time  `gorm:"column:created_at"`
}

func (r *Repository) Create(ctx context.Context, n *domain.Notification) error {
	id, now := uuid.New(), time.Now()
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO usr.usr_notification (id, user_id, level, title, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, n.UserID, n.Level, n.Title, n.Message, now).Error
	if err != nil {
		return shared.WrapError(err, "notification")
	}
	n.ID = id.String()
	n.CreatedAt = now
	return nil
}

func (r *Repository) List(ctx context.Context, userID string, page domain.Page) ([]domain.Notification, int64, error) {
	owned := func() *gorm.DB {
		return r.db.WithContext(ctx).Table("usr.usr_notification").Where("user_id = ?", userID)
	}
	var total int64
	if err := owned().Count(&total).Error; err != nil {
		return nil, 0, shared.WrapError(err, "notification")
	}
	var rows []row
	err := owned().Order("created_at DESC, id").Limit(page.Limit).Offset(page.Offset()).Find(&rows).Error
	if err != nil {
		return nil, 0, shared.WrapError(err, "notification")
	}
	out := make([]domain.Notification, 0, len(rows))
	for _, m := range rows {
		out = append(out, domain.Notification{
			ID: m.ID.String(), UserID: m.UserID.String(), Level: m.Level, Title: m.Title,
			Message: m.Message, ReadAt: m.ReadAt, CreatedAt: m.CreatedAt,
		})
	}
	return out, total, nil
}

func (r *Repository) CountUnread(ctx context.Context, userID string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("usr.usr_notification").
		Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	if err != nil {
		return 0, shared.WrapError(err, "notification")
	}
	return n, nil
}

func (r *Repository) MarkRead(ctx context.Context, id, userID string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_notification SET read_at = COALESCE(read_at, ?) WHERE id = ? AND user_id = ?
	`, now, id, userID)
	if res.Error != nil {
		return false, shared.WrapError(res.Error, "notification")
	}
	return res.RowsAffected == 1, nil
}
