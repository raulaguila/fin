package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
)

const (
	maxToolRounds  = 4  // tool calls per question before the assistant gives up
	toolRows       = 60 // transactions a search shows the model
	maxActions     = 5
	calendarEvents = 12
)

// toolsPrompt teaches a plain-text protocol instead of native tool calling, so
// it works the same on Claude Code headless and on a small local model.
const toolsPrompt = `FERRAMENTAS (só leitura). Se os DADOS não bastarem (outro mês, uma loja, uma pessoa), responda APENAS com uma linha JSON, sem texto antes ou depois:
{"ferramenta":"buscar_transacoes","texto":"ifood","de":"2026-08-01","ate":"2026-08-31"}
  transações cuja descrição contém o texto (opcional) entre as datas (opcionais; padrão: últimos 12 meses), com o total.
{"ferramenta":"resumo_mes","mes":"2026-08"}
  receitas, despesas e gasto por categoria de um mês.
{"ferramenta":"pessoas"}
  Pix e TED enviados e recebidos por pessoa nos últimos 12 meses.
O app devolve o RESULTADO e você continua. Nunca invente um resultado.

AÇÕES: quando ajudar, proponha no fim da resposta, uma por linha; o usuário confirma num botão:
AÇÃO {"tipo":"regra","palavra":"ifood","categoria":"Delivery"}
  categoriza as transações com essa palavra, as de hoje e as futuras. Use só categorias da lista de categorias.
AÇÃO {"tipo":"assinatura","nome":"Netflix","valor":"55,90","dia":3,"palavra":"netflix"}
  cadastra uma assinatura ou conta fixa mensal.
Não escreva AÇÃO para outra coisa. Você nunca move dinheiro.`

// planningContext adds what the user planned: budgets are in the categories,
// this adds the year, subscriptions, goals and the next 30 days.
func (s *Service) planningContext(ctx context.Context, userID string, cats []domain.Category) (string, error) {
	var b strings.Builder
	b.WriteString("\nCategorias que existem:")
	for i, c := range cats {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, " %s [%s]", c.Name, c.Kind)
	}
	b.WriteString(".\n")

	trend, err := s.Trend(ctx, userID, 12)
	if err != nil {
		return "", err
	}
	b.WriteString("\nÚltimos 12 meses (receitas; despesas):\n")
	for _, m := range trend {
		if m.Income != 0 || m.Expense != 0 {
			fmt.Fprintf(&b, "- %s: %s; %s\n", m.Month.Format("01/2006"), brl(m.Income), brl(m.Expense))
		}
	}

	subs, err := s.Subscriptions(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(subs.Items) > 0 {
		fmt.Fprintf(&b, "\nAssinaturas e contas fixas cadastradas (custo por mês %s):\n", brl(subs.Monthly))
		for _, x := range subs.Items {
			when := fmt.Sprintf("todo dia %d", x.Day)
			if x.Month > 0 {
				when = fmt.Sprintf("todo ano em %02d/%02d", x.Day, x.Month)
			}
			state := "a pagar, vence " + x.NextDue.Format("02/01")
			switch {
			case x.Late:
				state = "ATRASADA"
			case x.Paid:
				state = "paga em " + x.LastPaid.Format("02/01")
			}
			fmt.Fprintf(&b, "- %s: %s, %s, %s", x.Name, brl(x.Amount), when, state)
			if x.LastAmount != 0 && x.LastAmount != x.Amount {
				fmt.Fprintf(&b, " (última cobrança %s)", brl(x.LastAmount))
			}
			b.WriteString("\n")
		}
	}
	if len(subs.Detected) > 0 {
		b.WriteString("\nCobranças que se repetem e ainda não foram cadastradas:\n")
		for _, r := range subs.Detected {
			fmt.Fprintf(&b, "- %s: %s, vista em %d meses, última em %s\n", r.Name, brl(r.Amount), r.Months, r.LastDate.Format("02/01"))
		}
	}

	goals, err := s.Goals(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(goals) > 0 {
		b.WriteString("\nMetas:\n")
		for _, g := range goals {
			fmt.Fprintf(&b, "- %s: %s de %s (%d%%)", g.Name, brl(g.Current), brl(g.Target), percent(g.Current, g.Target))
			if g.DueDate != nil {
				fmt.Fprintf(&b, ", até %s", g.DueDate.Format("02/01/2006"))
			}
			b.WriteString("\n")
		}
	}

	cal, err := s.Calendar(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(cal.Events) > 0 {
		fmt.Fprintf(&b, "\nPróximos 30 dias (saldo hoje %s, menor saldo previsto %s):\n", brl(cal.Start), brl(cal.Lowest))
		for _, e := range cal.Events[:min(len(cal.Events), calendarEvents)] {
			fmt.Fprintf(&b, "- %s %s: %s, saldo depois %s\n", e.Date.Format("02/01"), e.Name, brl(e.Amount), brl(e.Balance))
		}
	}
	return b.String(), nil
}

type toolCall struct {
	Name  string `json:"ferramenta"`
	Text  string `json:"texto"`
	From  string `json:"de"`
	To    string `json:"ate"`
	Month string `json:"mes"`
	raw   string
}

var toolJSON = regexp.MustCompile(`\{[^{}]*"ferramenta"[^{}]*\}`)

// parseToolCall finds a tool call anywhere in the answer: small models wrap it
// in a code fence or a sentence.
func parseToolCall(answer string) (toolCall, bool) {
	raw := toolJSON.FindString(answer)
	if raw == "" {
		return toolCall{}, false
	}
	var c toolCall
	if json.Unmarshal([]byte(raw), &c) != nil {
		return toolCall{}, false
	}
	switch c.Name {
	case "buscar_transacoes", "resumo_mes", "pessoas":
		c.raw = raw
		return c, true
	}
	return toolCall{}, false
}

func (c toolCall) status() string {
	switch c.Name {
	case "buscar_transacoes":
		if c.Text != "" {
			return "Buscando transações: " + c.Text
		}
		return "Buscando transações"
	case "resumo_mes":
		return "Lendo o mês " + c.Month
	default:
		return "Lendo as transferências por pessoa"
	}
}

// runTool answers a tool call. A bad argument goes back to the model as text,
// so it can fix the call.
func (s *Service) runTool(ctx context.Context, userID string, now time.Time, c toolCall, cats []domain.Category) (string, error) {
	switch c.Name {
	case "resumo_mes":
		month, err := time.ParseInLocation("2006-01", c.Month, now.Location())
		if err != nil {
			return "mês inválido: use AAAA-MM", nil
		}
		return s.monthTool(ctx, userID, month)
	case "pessoas":
		people, err := s.People(ctx, userID)
		if err != nil {
			return "", err
		}
		if len(people) == 0 {
			return "nenhum Pix ou TED com pessoas nos últimos 12 meses", nil
		}
		var b strings.Builder
		for _, p := range people {
			fmt.Fprintf(&b, "- %s: enviou %s, recebeu %s, %d transferências, última em %s\n",
				p.Name, brl(-p.Sent), brl(p.Received), p.Count, p.LastDate.Format("02/01/2006"))
		}
		return b.String(), nil
	default:
		return s.searchTool(ctx, userID, now, c)
	}
}

func (s *Service) monthTool(ctx context.Context, userID string, month time.Time) (string, error) {
	sum, err := s.Summary(ctx, userID, month)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: receitas %s, despesas %s, sobra %s.\n", month.Format("01/2006"), brl(sum.Income), brl(sum.Expense), brl(sum.Income+sum.Expense))
	for _, c := range sum.Categories {
		if c.Total == 0 {
			continue
		}
		name := c.Name
		if name == "" {
			name = "Sem categoria"
		}
		fmt.Fprintf(&b, "- %s [%s]: %s\n", name, c.Kind, brl(c.Total))
	}
	return b.String(), nil
}

func (s *Service) searchTool(ctx context.Context, userID string, now time.Time, c toolCall) (string, error) {
	day := monthDay(now)
	from, to := day.AddDate(-1, 0, 0), day.AddDate(0, 0, 1)
	if c.From != "" {
		t, err := time.ParseInLocation(time.DateOnly, c.From, now.Location())
		if err != nil {
			return "data 'de' inválida: use AAAA-MM-DD", nil
		}
		from = t
	}
	if c.To != "" {
		t, err := time.ParseInLocation(time.DateOnly, c.To, now.Location())
		if err != nil {
			return "data 'ate' inválida: use AAAA-MM-DD", nil
		}
		to = t.AddDate(0, 0, 1) // the model means the day inclusive
	}
	// shortcut: the total covers the first 100 matches; a paged sum when a search returns more.
	txs, count, err := s.Transactions(ctx, userID, domain.TransactionFilter{
		From: from, To: to, Search: strings.TrimSpace(c.Text), Page: domain.Page{Number: 1, Limit: 100},
	})
	if err != nil {
		return "busca inválida: " + err.Error(), nil
	}
	if count == 0 {
		return fmt.Sprintf("nenhuma transação entre %s e %s", from.Format("02/01/2006"), to.AddDate(0, 0, -1).Format("02/01/2006")), nil
	}
	var out, in domain.Money
	for _, t := range txs {
		if t.Amount < 0 {
			out += t.Amount
		} else {
			in += t.Amount
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d transações entre %s e %s. ", count, from.Format("02/01/2006"), to.AddDate(0, 0, -1).Format("02/01/2006"))
	if int64(len(txs)) < count {
		fmt.Fprintf(&b, "Somando só as %d mais recentes: ", len(txs))
	}
	fmt.Fprintf(&b, "saídas %s, entradas %s.\n", brl(out), brl(in))
	for _, t := range txs[:min(len(txs), toolRows)] {
		cat := t.CategoryName
		if cat == "" {
			cat = "Sem categoria"
		}
		fmt.Fprintf(&b, "- %s %s: %s (%s, %s)\n", t.Date.Format("02/01/2006"), t.Description, brl(t.Amount), cat, t.AccountName)
	}
	return b.String(), nil
}

var actionLine = regexp.MustCompile(`(?m)^[\s*>-]*A[ÇC][ÃA]O:?\s*(\{.*\})\s*$`)

// parseActions takes the proposed actions out of the answer and keeps those that
// are valid: an existing category, a sane amount and day.
func parseActions(answer string, cats []domain.Category) (string, []domain.ChatAction) {
	var actions []domain.ChatAction
	seen := map[string]bool{}
	for _, m := range actionLine.FindAllStringSubmatch(answer, -1) {
		var a struct {
			Kind     string `json:"tipo"`
			Keyword  string `json:"palavra"`
			Category string `json:"categoria"`
			Name     string `json:"nome"`
			Amount   any    `json:"valor"`
			Day      int    `json:"dia"`
		}
		if json.Unmarshal([]byte(m[1]), &a) != nil || len(actions) == maxActions {
			continue
		}
		keyword, err := ruleKeyword(a.Keyword)
		if err != nil || seen[a.Kind+keyword] {
			continue
		}
		switch a.Kind {
		case "regra":
			for _, c := range cats {
				if strings.EqualFold(c.Name, strings.TrimSpace(a.Category)) {
					actions = append(actions, domain.ChatAction{Kind: domain.ActionRule, Keyword: keyword, CategoryID: c.ID, CategoryName: c.Name})
					seen[a.Kind+keyword] = true
					break
				}
			}
		case "assinatura":
			amount, ok := moneyValue(a.Amount)
			name := strings.TrimSpace(a.Name)
			if ok && name != "" && len([]rune(name)) <= 60 && a.Day >= 1 && a.Day <= 31 {
				actions = append(actions, domain.ChatAction{Kind: domain.ActionSubscription, Keyword: keyword, Name: name, Amount: amount, Day: a.Day})
				seen[a.Kind+keyword] = true
			}
		}
	}
	text := strings.TrimSpace(actionLine.ReplaceAllString(answer, ""))
	return text, actions
}

// moneyValue reads 55.9, "55,90" or "R$ 1.234,56" as cents.
func moneyValue(v any) (domain.Money, bool) {
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case string:
		x = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(x), "R$"))
		if strings.Contains(x, ",") {
			x = strings.ReplaceAll(strings.ReplaceAll(x, ".", ""), ",", ".")
		}
		var err error
		if f, err = strconv.ParseFloat(x, 64); err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	cents := domain.Money(f*100 + 0.5)
	return cents, cents > 0 && cents <= maxMoneyCents
}
