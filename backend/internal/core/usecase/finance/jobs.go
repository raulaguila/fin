package finance

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/raulaguila/fin/internal/core/policy"
	"github.com/raulaguila/fin/pkg/apperror"
)

// Job kinds. JobDaily is queued by pg_cron (000005_finance.up.sql).
const (
	JobDaily = "finance.daily"
	JobSync  = "finance.sync"
)

type syncJob struct {
	ConnectionID string `json:"connection_id"`
}

// Jobs returns this module's job handlers.
func (s *Service) Jobs() map[string]policy.JobHandler {
	return map[string]policy.JobHandler{
		JobDaily: s.runDaily,
		JobSync:  s.runSync,
	}
}

// runDaily queues one sync per connection; each sync runs the alerts of its user.
func (s *Service) runDaily(ctx context.Context, _ []byte) error {
	if err := s.repo().PruneUndo(ctx, s.Clock.Now().Add(-undoKeep)); err != nil {
		return err
	}
	if !s.OpenFinanceEnabled() {
		return nil
	}
	ids, err := s.repo().ConnectionIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.enqueueSync(ctx, s.Store, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runSync(ctx context.Context, payload []byte) error {
	var job syncJob
	if err := json.Unmarshal(payload, &job); err != nil || job.ConnectionID == "" {
		return policy.PermanentJobError(errors.New("finance.sync: bad payload"))
	}
	if !s.OpenFinanceEnabled() {
		return policy.PermanentJobError(errors.New("finance.sync: open finance disabled"))
	}
	c, err := s.sync(ctx, job.ConnectionID)
	if apperror.IsNotFound(err) {
		return nil // the connection was deleted after the job was queued
	}
	if err != nil {
		if c != nil && !errors.Is(err, ErrStillUpdating) {
			s.Log.Warn("Open finance sync failed", slog.String("connection_id", c.ID), slog.String("error", err.Error()))
		}
		return err
	}
	return s.Alerts(ctx, c.UserID)
}
