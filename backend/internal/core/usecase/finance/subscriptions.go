package finance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/pkg/apperror"
	"github.com/raulaguila/fin/pkg/validate"
)

const (
	NotifySubscription = "finance_subscription"

	subscriptionEarly   = 10 // a charge up to 10 days before the due day pays that period
	subscriptionLate    = 3  // days past the due day before a missing charge is late
	subscriptionYearEnd = 30 // a yearly one still unpaid this long after its day waits for next year
	subscriptionMonths  = 13 // history read for the last charge of a yearly one
)

// matches reports whether a description holds the keyword as whole words.
func matches(s domain.Subscription, desc string) bool {
	k := normText(s.Keyword)
	return k != "" && strings.Contains(" "+normText(desc)+" ", " "+k+" ")
}

// dueOn is the due day of s in a month, the last day when the month is shorter.
func dueOn(s domain.Subscription, year int, month time.Month) time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(year, month, min(s.Day, last), 0, 0, 0, 0, time.UTC)
}

// nextDue is the due date of the period after the one due on due.
func nextDue(s domain.Subscription, due time.Time) time.Time {
	if s.Month == 0 {
		return dueOn(s, due.Year(), due.Month()+1)
	}
	return dueOn(s, due.Year()+1, due.Month())
}

// subscriptionStatus fills the last charge, whether the current period is paid,
// the next due date and lateness, from spending (newest first).
func subscriptionStatus(subs []domain.Subscription, spending []domain.Transaction, today time.Time) {
	for i := range subs {
		s := &subs[i]
		for _, t := range spending {
			if matches(*s, t.Description) {
				d := t.Date
				s.LastPaid, s.LastAmount, s.LastAccount = &d, -t.Amount, t.AccountID
				break
			}
		}
		due := dueOn(*s, today.Year(), today.Month())
		if s.Month != 0 {
			if due = dueOn(*s, today.Year(), time.Month(s.Month)); today.After(due.AddDate(0, 0, subscriptionYearEnd)) {
				due = nextDue(*s, due)
			}
		}
		s.Paid = s.LastPaid != nil && !s.LastPaid.Before(due.AddDate(0, 0, -subscriptionEarly))
		s.NextDue = due
		if s.Paid {
			s.NextDue = nextDue(*s, due)
		}
		s.Late = !s.Paid && today.After(s.NextDue.AddDate(0, 0, subscriptionLate))
	}
}

// detected drops the recurring charges the user dismissed or registered.
func detected(found []domain.Recurring, subs []domain.Subscription, ignored []string) []domain.Recurring {
	out := found[:0:0]
next:
	for _, r := range found {
		for _, k := range ignored {
			if r.Key == k {
				continue next
			}
		}
		for _, s := range subs {
			if matches(s, r.Name) {
				continue next
			}
		}
		out = append(out, r)
	}
	return out
}

// subscriptions loads the registered ones with their status and the dismissed keys.
func (s *Service) subscriptions(ctx context.Context, userID string, spending []domain.Transaction, today time.Time) ([]domain.Subscription, []string, error) {
	subs, err := s.repo().Subscriptions(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	ignored, err := s.repo().IgnoredRecurring(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	subscriptionStatus(subs, spending, today)
	return subs, ignored, nil
}

func (s *Service) Subscriptions(ctx context.Context, userID string) (*domain.Subscriptions, error) {
	today := monthDay(s.Clock.Now())
	spending, err := s.repo().Spending(ctx, userID, monthStart(today).AddDate(0, 1-subscriptionMonths, 0), today.AddDate(0, 0, 1), spendingCap)
	if err != nil {
		return nil, err
	}
	subs, ignored, err := s.subscriptions(ctx, userID, spending, today)
	if err != nil {
		return nil, err
	}
	// Detection looks at the last months only: a charge that stopped is not one any more.
	found := recurring(recentWindow(spending, monthStart(today).AddDate(0, 1-recurringMonths, 0)))
	out := &domain.Subscriptions{Items: subs, Detected: detected(found, subs, ignored)}
	for _, sub := range subs {
		if sub.Month == 0 {
			out.Monthly += sub.Amount
		} else {
			out.Monthly += sub.Amount / 12
		}
	}
	return out, nil
}

func (s *Service) checkSubscription(ctx context.Context, userID string, sub *domain.Subscription) error {
	var err error
	if sub.Name, err = shortText("name", sub.Name, 60); err != nil {
		return err
	}
	if sub.Keyword, err = shortText("keyword", sub.Keyword, 60); err != nil {
		return err
	}
	if len([]rune(normText(sub.Keyword))) < 2 {
		return apperror.New(apperror.CodeInvalidInput, "the keyword needs letters").WithField("keyword")
	}
	if sub.Amount <= 0 || sub.Amount > maxMoneyCents {
		return apperror.New(apperror.CodeInvalidInput, "invalid amount").WithField("amount")
	}
	if sub.Day < 1 || sub.Day > 31 {
		return apperror.New(apperror.CodeInvalidInput, "invalid day").WithField("day")
	}
	if sub.Month < 0 || sub.Month > 12 {
		return apperror.New(apperror.CodeInvalidInput, "invalid month").WithField("month")
	}
	if sub.CategoryID != "" {
		if _, err := s.ownCategory(ctx, userID, sub.CategoryID); err != nil {
			return err
		}
	}
	if sub.AccountID != "" {
		if _, err := s.ownAccount(ctx, userID, sub.AccountID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CreateSubscription(ctx context.Context, userID string, sub domain.Subscription) (*domain.Subscription, error) {
	if err := s.checkSubscription(ctx, userID, &sub); err != nil {
		return nil, err
	}
	if err := s.repo().CreateSubscription(ctx, userID, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Service) UpdateSubscription(ctx context.Context, userID, id string, sub domain.Subscription) (*domain.Subscription, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	sub.ID = id
	if err := s.checkSubscription(ctx, userID, &sub); err != nil {
		return nil, err
	}
	if err := s.repo().UpdateSubscription(ctx, userID, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Service) DeleteSubscription(ctx context.Context, userID, id string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	return s.repo().DeleteSubscription(ctx, userID, id)
}

func (s *Service) IgnoreRecurring(ctx context.Context, userID, key string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 120 {
		return apperror.New(apperror.CodeInvalidInput, "invalid key").WithField("key")
	}
	return s.repo().IgnoreRecurring(ctx, userID, key)
}

// subscriptionAlerts warns of a charge that did not come and of a registered one
// charged above its amount. The price key matches spendingAlerts', so a charge
// both see is told once.
func subscriptionAlerts(subs []domain.Subscription, spending []domain.Transaction, day time.Time) []alert {
	var out []alert
	for _, s := range subs {
		if s.Late {
			out = append(out, alert{key: "sub-late:" + s.ID + ":" + s.NextDue.Format(time.DateOnly), level: domain.NotificationWarning, kind: NotifySubscription,
				title: "Cobrança não apareceu: " + s.Name,
				text: fmt.Sprintf("%s vencia em %s (%s) e não apareceu no extrato. Se já pagou, confira a palavra-chave \"%s\" em Assinaturas.",
					s.Name, s.NextDue.Format("02/01"), brl(s.Amount), s.Keyword)})
		}
		if s.LastPaid == nil || s.LastPaid.Before(day.AddDate(0, 0, -priceDays)) || s.LastAmount*100 <= s.Amount*105 {
			continue // only a charge this week at least 5% above the registered amount
		}
		for _, t := range spending {
			if matches(s, t.Description) {
				out = append(out, alert{key: "price:" + merchantKey(t.Description) + ":" + t.Date.Format(time.DateOnly), level: domain.NotificationInfo, kind: NotifyPrice,
					title: "Conta mais cara: " + s.Name,
					text:  fmt.Sprintf("%s cobrou %s em %s; o cadastrado é %s.", s.Name, brl(s.LastAmount), t.Date.Format("02/01"), brl(s.Amount))})
				break
			}
		}
	}
	return out
}
