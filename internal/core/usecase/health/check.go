package health

import (
	"context"
	"sync"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

const reportTTL = time.Second

// CheckUseCase aggregates dependency pings.
type CheckUseCase struct {
	Postgres output.Pinger

	mu   sync.Mutex
	last *domain.HealthReport
	at   time.Time
}

var _ input.HealthChecker = (*CheckUseCase)(nil)

func New(postgres output.Pinger) *CheckUseCase {
	return &CheckUseCase{Postgres: postgres}
}

// Check reuses a fresh report: /health has no rate limit, so without this every
// anonymous request would reach the database.
func (u *CheckUseCase) Check(ctx context.Context) (*domain.HealthReport, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.last != nil && time.Since(u.at) < reportTTL {
		return u.last, nil
	}
	u.last, u.at = u.probe(ctx), time.Now()
	return u.last, nil
}

func (u *CheckUseCase) probe(ctx context.Context) *domain.HealthReport {
	report := &domain.HealthReport{
		Checks: make(map[string]domain.CheckStatus, 1),
	}

	if u.Postgres == nil {
		report.Checks["postgres"] = domain.CheckFail
	} else if err := u.Postgres.Ping(ctx); err != nil {
		report.Checks["postgres"] = domain.CheckFail
	} else {
		report.Checks["postgres"] = domain.CheckOK
	}

	report.Aggregate()
	return report
}
