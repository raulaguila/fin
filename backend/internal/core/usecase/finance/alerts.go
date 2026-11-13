package finance

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/internal/core/port/output"
	"github.com/raulaguila/fin/pkg/validate"
)

// Notification kinds of this module. Title and message go in Portuguese: the app is pt-BR.
const (
	NotifyBudget     = "finance_budget"
	NotifyConsent    = "finance_consent"
	NotifyConnection = "finance_connection"
	NotifyBill       = "finance_bill"
	NotifyLargeSpend = "finance_large_spend"
	NotifySummary    = "finance_summary"
	NotifyOverdraft  = "finance_overdraft"
	NotifyDuplicate  = "finance_duplicate"
	NotifyPrice      = "finance_price"
	NotifyWeekly     = "finance_weekly"
	NotifyGoal       = "finance_goal"
)

const (
	consentWarnDays = 15
	billWarnDays    = 3
	largeMinimum    = 500_00 // a large spend is at least R$ 500
	largeFactor     = 3      // and 3 times the typical outflow
	summaryDays     = 3      // the monthly summary goes out in the first days of the month
	recentDays      = 3      // charges this recent are checked for duplicates and interest
	priceDays       = 7      // a price rise is told within a week of the charge
	alertMonths     = 4      // spending window of the alerts, as long as the recurring detection
	paceMonths      = 6      // full months the saving pace averages
)

type alert struct {
	key   string
	level string
	kind  string
	title string
	text  string
}

// notify creates the notification once per key; a repeated run changes nothing.
func (s *Service) notify(ctx context.Context, userID string, a alert) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		isNew, err := uow.Finance().MarkAlert(ctx, userID, a.key)
		if err != nil || !isNew {
			return err
		}
		err = uow.Notifications().Create(ctx, &domain.Notification{
			UserID: userID, Level: a.level, Kind: a.kind,
			Title: validate.Text(a.title, 120), Message: validate.Text(a.text, 4000),
		})
		if err != nil || s.Mail == nil || a.level == domain.NotificationInfo {
			return err
		}
		return s.email(ctx, uow, userID, a)
	})
}

// email queues the alert for the mailer, in the same transaction as the notification.
// Only warnings and critical alerts go out: info stays in the app.
func (s *Service) email(ctx context.Context, uow output.UnitOfWork, userID string, a alert) error {
	u, err := uow.Users().FindByID(ctx, userID)
	if err != nil {
		return err
	}
	// The codec seals Token at rest: the outbox never holds the alert in clear text.
	payload, err := s.Mail.Encode(output.EmailToken{Email: u.Email, Token: validate.Text(a.title, 120) + "\n" + validate.Text(a.text, 4000)})
	if err != nil {
		return err
	}
	return uow.Outbox().Enqueue(ctx, domain.OutboxPurposeAlert, payload)
}

// Alerts checks the user's data and notifies what needs attention.
func (s *Service) Alerts(ctx context.Context, userID string) error {
	now := s.Clock.Now()
	var alerts []alert
	add := func(list []alert, err error) error {
		alerts = append(alerts, list...)
		return err
	}
	if err := add(s.budgetAlerts(ctx, userID, now)); err != nil {
		return err
	}
	if err := add(s.connectionAlerts(ctx, userID, now)); err != nil {
		return err
	}
	if err := add(s.goalAlerts(ctx, userID, now)); err != nil {
		return err
	}
	day := monthDay(now)
	spending, err := s.repo().Spending(ctx, userID, monthStart(now).AddDate(0, 1-alertMonths, 0), day.AddDate(0, 0, 1), spendingCap)
	if err != nil {
		return err
	}
	subs, _, err := s.subscriptions(ctx, userID, spending, day)
	if err != nil {
		return err
	}
	alerts = append(alerts, largeSpendAlerts(spending, day)...)
	alerts = append(alerts, spendingAlerts(spending, day)...)
	alerts = append(alerts, subscriptionAlerts(subs, spending, day)...)
	for _, a := range alerts {
		if err := s.notify(ctx, userID, a); err != nil {
			return err
		}
	}
	if err := s.snapshotNetWorth(ctx, userID, now); err != nil {
		return err
	}
	if err := s.monthlySummary(ctx, userID, now); err != nil {
		return err
	}
	return s.weeklySummary(ctx, userID, now, spending)
}

func (s *Service) budgetAlerts(ctx context.Context, userID string, now time.Time) ([]alert, error) {
	from := monthStart(now)
	cats, err := s.repo().Categories(ctx, userID)
	if err != nil {
		return nil, err
	}
	totals, err := s.repo().Totals(ctx, userID, from, from.AddDate(0, 1, 0))
	if err != nil {
		return nil, err
	}
	_, _, list := combine(cats, totals)
	if anyRollover(cats) {
		prev, err := s.repo().Totals(ctx, userID, from.AddDate(0, -1, 0), from)
		if err != nil {
			return nil, err
		}
		carryOver(list, prev)
	}
	var out []alert
	month := from.Format("2006-01")
	for _, c := range list {
		if c.Kind != domain.KindExpense || c.Limit <= 0 || c.CategoryID == "" {
			continue
		}
		used := percent(-c.Total, c.Limit)
		switch {
		case used >= 100:
			out = append(out, alert{key: "budget:" + c.CategoryID + ":" + month + ":100", level: domain.NotificationCritical, kind: NotifyBudget,
				title: "Orçamento de " + c.Name + " estourado",
				text:  fmt.Sprintf("Você gastou %s em %s este mês, %d%% do limite de %s.", brl(-c.Total), c.Name, used, brl(c.Limit))})
		case used >= 80:
			out = append(out, alert{key: "budget:" + c.CategoryID + ":" + month + ":80", level: domain.NotificationWarning, kind: NotifyBudget,
				title: "Orçamento de " + c.Name + " em " + fmt.Sprint(used) + "%",
				text:  fmt.Sprintf("Restam %s do limite de %s em %s até o fim do mês.", brl(c.Limit+c.Total), brl(c.Limit), c.Name)})
		}
	}
	return out, nil
}

// savingPace is what was left on average in the last full months: income plus
// (negative) spending, never below zero.
func (s *Service) savingPace(ctx context.Context, userID string, now time.Time) (domain.Money, error) {
	to := monthStart(now)
	months, err := s.repo().MonthlyTotals(ctx, userID, to.AddDate(0, -paceMonths, 0), to)
	if err != nil || len(months) == 0 {
		return 0, err
	}
	var left domain.Money
	for _, m := range months {
		left += m.Income + m.Expense
	}
	return max(0, left/domain.Money(len(months))), nil
}

// monthsLeft counts the months from now to due, the current one included.
func monthsLeft(now, due time.Time) int {
	return max(1, (due.Year()-now.Year())*12+int(due.Month()-now.Month())+1)
}

// goalAlerts tells, once a month, about a goal with a date that the saving pace does not reach.
func (s *Service) goalAlerts(ctx context.Context, userID string, now time.Time) ([]alert, error) {
	goals, err := s.repo().Goals(ctx, userID)
	if err != nil || len(goals) == 0 {
		return nil, err
	}
	pace, err := s.savingPace(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	var out []alert
	month := now.Format("2006-01")
	for _, g := range goals {
		missing := g.Target - g.Current
		if g.DueDate == nil || missing <= 0 || g.DueDate.Before(now) {
			continue
		}
		need := missing / domain.Money(monthsLeft(now, *g.DueDate))
		if need <= pace {
			continue
		}
		out = append(out, alert{key: "goal:" + g.ID + ":" + month, level: domain.NotificationInfo, kind: NotifyGoal,
			title: "Meta " + g.Name + " fora do ritmo",
			text: fmt.Sprintf("Faltam %s até %s: %s por mês. Nos últimos meses sobraram em média %s por mês.",
				brl(missing), g.DueDate.Format("02/01/2006"), brl(need), brl(pace))})
	}
	return out, nil
}

func (s *Service) connectionAlerts(ctx context.Context, userID string, now time.Time) ([]alert, error) {
	conns, err := s.repo().Connections(ctx, userID)
	if err != nil {
		return nil, err
	}
	year, week := now.ISOWeek()
	var out []alert
	for _, c := range conns {
		if c.ConsentExpiresAt != nil {
			if days := int(c.ConsentExpiresAt.Sub(now).Hours() / 24); days <= consentWarnDays {
				out = append(out, alert{key: "consent:" + c.ID + ":" + c.ConsentExpiresAt.Format(time.DateOnly), level: domain.NotificationWarning, kind: NotifyConsent,
					title: "Renove a conexão com " + c.Institution,
					text:  fmt.Sprintf("O consentimento do Open Finance com %s vence em %s. Renove em Conexões para não perder a sincronização.", c.Institution, c.ConsentExpiresAt.Format("02/01/2006"))})
			}
		}
		if c.Status == domain.ConnError || c.Status == domain.ConnLoginRequired {
			text := "A sincronização com " + c.Institution + " falhou."
			if c.Status == domain.ConnLoginRequired {
				text = "O banco " + c.Institution + " pediu uma nova autorização. Abra Conexões e reconecte."
			}
			out = append(out, alert{key: fmt.Sprintf("conn:%s:%s:%d-%d", c.ID, c.Status, year, week), level: domain.NotificationWarning, kind: NotifyConnection,
				title: "Problema na conexão com " + c.Institution, text: text})
		}
		for _, a := range c.Accounts {
			if a.Type == domain.AccountBank && a.Balance < 0 {
				out = append(out, alert{key: fmt.Sprintf("overdraft:%s:%d-%d", a.ID, year, week), level: domain.NotificationCritical, kind: NotifyOverdraft,
					title: "Conta " + a.Name + " no negativo",
					text:  fmt.Sprintf("O saldo da conta %s em %s está em %s: o banco cobra juros do cheque especial a cada dia.", a.Name, c.Institution, brl(a.Balance))})
			}
			due := a.NextBillDue(monthDay(now))
			if a.Type != domain.AccountCredit || due == nil || a.Balance <= 0 {
				continue
			}
			if days := int(due.Sub(monthDay(now)).Hours() / 24); days <= billWarnDays {
				out = append(out, alert{key: "bill:" + a.ID + ":" + due.Format(time.DateOnly), level: domain.NotificationInfo, kind: NotifyBill,
					title: "Fatura do " + a.Name + " vence em " + due.Format("02/01"),
					text:  fmt.Sprintf("A fatura do cartão %s, de %s, vence em %s.", a.Name, brl(a.Balance), due.Format("02/01/2006"))})
			}
		}
	}
	return out, nil
}

func monthDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// largeSpendAlerts flags a recent outflow far above the user's typical one
// over the last 90 days. spending is newest first.
func largeSpendAlerts(all []domain.Transaction, day time.Time) []alert {
	spending := recentWindow(all, day.AddDate(0, 0, -90))
	if len(spending) < 10 {
		return nil
	}
	amounts := make([]domain.Money, 0, len(spending))
	for _, t := range spending {
		amounts = append(amounts, -t.Amount)
	}
	slices.Sort(amounts)
	threshold := max(largeMinimum, amounts[len(amounts)/2]*largeFactor)
	var out []alert
	for _, t := range spending {
		if t.Date.Before(day.AddDate(0, 0, -2)) {
			break // newest first
		}
		if -t.Amount >= threshold {
			out = append(out, alert{key: "large:" + t.ID, level: domain.NotificationInfo, kind: NotifyLargeSpend,
				title: "Gasto alto: " + brl(-t.Amount),
				text:  fmt.Sprintf("%s em %s (%s). É bem acima do seu gasto típico.", brl(-t.Amount), t.Description, t.Date.Format("02/01"))})
		}
	}
	return out
}

// spendingAlerts flags interest charged, a charge repeated the same day and a
// recurring charge that got more expensive.
func spendingAlerts(spending []domain.Transaction, day time.Time) []alert {
	var out []alert
	recent := recentWindow(spending, day.AddDate(0, 0, -recentDays))
	for _, t := range recent {
		if t.CategoryName == catInterest {
			out = append(out, alert{key: "interest:" + t.ID, level: domain.NotificationWarning, kind: NotifyOverdraft,
				title: "Cobrança de juros: " + brl(-t.Amount),
				text:  fmt.Sprintf("%s em %s (%s). Juros de cheque especial ou atraso costumam ser os mais caros: vale cobrir o saldo.", brl(-t.Amount), t.Description, t.Date.Format("02/01"))})
		}
	}
	for _, group := range duplicates(recent) {
		t := group[0]
		out = append(out, alert{key: fmt.Sprintf("dup:%s:%s:%d:%s", t.AccountID, t.Date.Format(time.DateOnly), t.Amount, merchantKey(t.Description)),
			level: domain.NotificationWarning, kind: NotifyDuplicate,
			title: fmt.Sprintf("Cobrança repetida: %d× %s", len(group), brl(-t.Amount)),
			text:  fmt.Sprintf("%s aparece %d vezes em %s com o mesmo valor (%s). Se não foram compras separadas, conteste com o banco.", t.Description, len(group), t.Date.Format("02/01"), brl(-t.Amount))})
	}
	for _, r := range recurring(spending) {
		if !r.Changed || r.Last >= r.Previous || r.LastDate.Before(day.AddDate(0, 0, -priceDays)) {
			continue // only a rise (more negative) charged this week
		}
		out = append(out, alert{key: "price:" + merchantKey(r.Name) + ":" + r.LastDate.Format(time.DateOnly), level: domain.NotificationInfo, kind: NotifyPrice,
			title: "Conta mais cara: " + r.Name,
			text:  fmt.Sprintf("%s passou de %s para %s (%s).", r.Name, brl(-r.Previous), brl(-r.Last), r.LastDate.Format("02/01"))})
	}
	return out
}

// monthlySummary sends last month's recap once, in the first days of a month.
// With the assistant on, the model writes it from the computed facts.
func (s *Service) monthlySummary(ctx context.Context, userID string, now time.Time) error {
	if now.Day() > summaryDays {
		return nil
	}
	prev := monthStart(now).AddDate(0, -1, 0)
	key := "summary:" + prev.Format("2006-01")
	sum, err := s.Summary(ctx, userID, prev)
	if err != nil {
		return err
	}
	if sum.Income == 0 && sum.Expense == 0 {
		return nil
	}
	title := "Resumo de " + monthName(prev)
	text := fmt.Sprintf("Receitas %s, despesas %s, sobra %s.", brl(sum.Income), brl(-sum.Expense), brl(sum.Income+sum.Expense))
	if s.AI != nil {
		// Check first: a repeated run must not pay for a second answer.
		isNew, err := s.repo().MarkAlert(ctx, userID, key+":ai")
		if err != nil {
			return err
		}
		if isNew {
			facts, err := s.financialContext(ctx, userID, prev.AddDate(0, 1, -1))
			if err != nil {
				return err
			}
			answer, err := s.AI.Ask(ctx, systemPrompt, "DADOS (calculados pelo app):\n"+facts+
				"\nEscreva o resumo do mês "+monthName(prev)+" em até 6 frases: como foi, onde mais gastou, o que mudou e uma dica concreta de economia.", nil)
			if err != nil {
				s.Log.Warn("Monthly summary by the assistant failed", slog.String("error", err.Error()))
			} else if answer = strings.TrimSpace(answer); answer != "" {
				text = answer
			}
		}
	}
	return s.notify(ctx, userID, alert{key: key, level: domain.NotificationInfo, kind: NotifySummary, title: title, text: text})
}

// weeklySummary sends, on Mondays, what the last 7 days cost against the 7
// before and what falls due this week. With the assistant on, the model writes it.
func (s *Service) weeklySummary(ctx context.Context, userID string, now time.Time, spending []domain.Transaction) error {
	if now.Weekday() != time.Monday {
		return nil
	}
	day := monthDay(now)
	year, week := day.ISOWeek()
	key := fmt.Sprintf("week:%d-%02d", year, week)
	text, ok := weekFacts(spending, day)
	if !ok {
		return nil
	}
	cal, err := s.Calendar(ctx, userID)
	if err != nil {
		return err
	}
	var due []string
	for _, e := range cal.Events {
		if e.Amount < 0 && e.Date.Before(day.AddDate(0, 0, 7)) {
			due = append(due, fmt.Sprintf("%s %s %s", e.Date.Format("02/01"), e.Name, brl(-e.Amount)))
		}
	}
	if len(due) > 0 {
		text += " Vence esta semana: " + strings.Join(due, "; ") + "."
	}
	if s.AI != nil {
		// Check first: a repeated run must not pay for a second answer.
		isNew, err := s.repo().MarkAlert(ctx, userID, key+":ai")
		if err != nil {
			return err
		}
		if isNew {
			answer, err := s.AI.Ask(ctx, systemPrompt, "DADOS (calculados pelo app):\n"+text+
				"\nEscreva o resumo da semana em até 4 frases: quanto gastou, onde, comparação com a semana anterior, o que vence e uma dica concreta.", nil)
			if err != nil {
				s.Log.Warn("Weekly summary by the assistant failed", slog.String("error", err.Error()))
			} else if answer = strings.TrimSpace(answer); answer != "" {
				text = answer
			}
		}
	}
	return s.notify(ctx, userID, alert{key: key, level: domain.NotificationInfo, kind: NotifyWeekly, title: "Sua semana", text: text})
}

// weekFacts tells what the 7 days before day cost, against the 7 before them,
// and the top 3 categories. ok is false for a week without spending.
func weekFacts(spending []domain.Transaction, day time.Time) (string, bool) {
	weekAgo, twoAgo := day.AddDate(0, 0, -7), day.AddDate(0, 0, -14)
	var last, before domain.Money
	byCat := map[string]domain.Money{}
	for _, t := range spending {
		switch {
		case !t.Date.Before(weekAgo) && t.Date.Before(day):
			last += t.Amount
			byCat[cmp.Or(t.CategoryName, "Sem categoria")] += t.Amount
		case !t.Date.Before(twoAgo) && t.Date.Before(weekAgo):
			before += t.Amount
		}
	}
	if last == 0 {
		return "", false
	}
	names := slices.SortedFunc(maps.Keys(byCat), func(a, b string) int { return cmp.Compare(byCat[a], byCat[b]) })
	top := make([]string, 0, 3)
	for _, n := range names[:min(len(names), 3)] {
		top = append(top, fmt.Sprintf("%s %s", n, brl(-byCat[n])))
	}
	return fmt.Sprintf("Gastou %s nos últimos 7 dias (%s na semana anterior). Mais em: %s.", brl(-last), brl(-before), strings.Join(top, ", ")), true
}

var months = []string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

func monthName(t time.Time) string { return months[t.Month()-1] + " de " + fmt.Sprint(t.Year()) }
