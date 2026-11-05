package health

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

// CheckUseCase aggregates dependency pings.
type CheckUseCase struct {
	Postgres output.Pinger
	Cache    output.Pinger // optional; nil → skipped
}

var _ input.HealthChecker = (*CheckUseCase)(nil)

func New(postgres output.Pinger, cache output.Pinger) *CheckUseCase {
	return &CheckUseCase{Postgres: postgres, Cache: cache}
}

func (u *CheckUseCase) Check(ctx context.Context) (*domain.HealthReport, error) {
	report := &domain.HealthReport{
		Checks: make(map[string]domain.CheckStatus, 2),
	}

	if u.Postgres == nil {
		report.Checks["postgres"] = domain.CheckFail
	} else if err := u.Postgres.Ping(ctx); err != nil {
		report.Checks["postgres"] = domain.CheckFail
	} else {
		report.Checks["postgres"] = domain.CheckOK
	}

	if u.Cache == nil {
		report.Checks["cache"] = domain.CheckSkipped
	} else if err := u.Cache.Ping(ctx); err != nil {
		report.Checks["cache"] = domain.CheckFail
	} else {
		report.Checks["cache"] = domain.CheckOK
	}

	report.Aggregate()
	return report, nil
}
