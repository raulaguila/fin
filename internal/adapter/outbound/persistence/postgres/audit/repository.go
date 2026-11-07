package audit

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

type Repository struct {
	db *gorm.DB
}

var _ output.AuditRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Record(ctx context.Context, e domain.AuditEntry) error {
	details := []byte("{}")
	if len(e.Details) > 0 {
		var err error
		if details, err = json.Marshal(e.Details); err != nil {
			return err
		}
	}
	return shared.WrapError(r.db.WithContext(ctx).Exec(`
		INSERT INTO usr.usr_audit_log (actor_id, request_id, action, target_id, details)
		VALUES (NULLIF(?, '')::uuid, NULLIF(?, ''), ?, NULLIF(?, '')::uuid, ?::jsonb)
	`, e.ActorID, e.RequestID, e.Action, e.TargetID, string(details)).Error, "audit log")
}
