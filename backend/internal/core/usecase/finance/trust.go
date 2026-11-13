package finance

import (
	"context"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/pkg/validate"
)

const (
	pairDays    = 3     // a transfer between own accounts lands within this many days
	pairMinimum = 10_00 // smaller amounts are too often coincidences
	pairMonths  = 4     // window the matching looks at
	healthDays  = 90    // window of the counts on the data health screen
	undoKeep    = 7 * 24 * time.Hour
)

// PairTransfers files money that left one of the user's accounts and arrived in
// another as "Entre minhas contas", so it counts as neither spending nor income.
func (s *Service) PairTransfers(ctx context.Context, userID string) (int64, error) {
	cat, err := s.categorizer(ctx, userID)
	if err != nil {
		return 0, err
	}
	own, ok := cat.byName[strings.ToLower(catOwnAccounts)]
	if !ok {
		return 0, nil // the user renamed or removed it: nothing to file into
	}
	from := monthStart(s.Clock.Now()).AddDate(0, 1-pairMonths, 0)
	return s.repo().PairTransfers(ctx, userID, own.ID, from, pairDays, pairMinimum)
}

// Undo puts back what a bulk change or a new rule overwrote, and drops the rule.
func (s *Service) Undo(ctx context.Context, userID, undoID string) (int64, error) {
	if err := validate.ID(undoID); err != nil {
		return 0, err
	}
	return s.repo().Undo(ctx, userID, undoID)
}

// ResetBalanceCheck accepts the current gap of a bank account and checks from now on.
func (s *Service) ResetBalanceCheck(ctx context.Context, userID, accountID string) error {
	if err := validate.ID(accountID); err != nil {
		return err
	}
	return s.repo().StartBalanceCheck(ctx, userID, accountID, true)
}

// DataHealth tells whether the numbers can be trusted: sync state comes from the connections.
func (s *Service) DataHealth(ctx context.Context, userID string) (*domain.DataHealth, error) {
	return s.repo().DataHealth(ctx, userID, s.Clock.Now().AddDate(0, 0, -healthDays))
}
