package finance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/pkg/apperror"
	"github.com/raulaguila/fin/pkg/validate"
)

const (
	maxChatTurns   = 12
	maxChatMessage = 4000
	topExpenses    = 25
)

// systemPrompt fixes what the assistant is: the numbers come computed from the
// user's data, the model only explains them. It cannot move money.
const systemPrompt = `Você é o Fin, assistente financeiro pessoal dentro de um app de controle de gastos.
Responda sempre em português do Brasil, de forma direta e curta (no máximo uns 8 parágrafos curtos ou uma lista).
Use apenas os dados do bloco DADOS; eles foram calculados pelo app a partir das contas do usuário e são a verdade.
Não invente números. Se faltar um dado para responder, diga o que falta.
Valores em reais no formato R$ 1.234,56. Gastos aparecem negativos nos dados; ao falar, diga "gastou R$ X".
"Entre minhas contas" e "Investimentos" são movimentações entre contas do próprio usuário: não são gastos.
Para perguntas do tipo "posso comprar X?", compare o valor com a sobra do mês (receitas + despesas), com o saldo
das contas, com as contas recorrentes e parcelas que ainda vão vencer, e responda sim, com cuidado ou não, explicando o porquê.
Você só lê dados: não faz pagamentos, transferências, nem pede senhas. Não é consultor de investimentos licenciado.`

func brl(v domain.Money) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	reais := fmt.Sprint(v / 100)
	var b strings.Builder
	for i, r := range reais {
		if i > 0 && (len(reais)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, b.String(), v%100)
}

// financialContext writes the facts the assistant answers from. It leaves out
// account numbers and anything else the answer does not need.
func (s *Service) financialContext(ctx context.Context, userID string, now time.Time) (string, error) {
	sum, err := s.Summary(ctx, userID, now)
	if err != nil {
		return "", err
	}
	from := monthStart(now)
	var b strings.Builder
	fmt.Fprintf(&b, "Hoje: %s. Mês corrente: %s (dia %d de %d).\n", now.Format(time.DateOnly), from.Format("01/2006"),
		now.Day(), from.AddDate(0, 1, -1).Day())
	fmt.Fprintf(&b, "Receitas no mês: %s. Despesas no mês: %s. Sobra: %s.\n", brl(sum.Income), brl(sum.Expense), brl(sum.Income+sum.Expense))

	b.WriteString("\nContas:\n")
	for _, a := range sum.Accounts {
		if a.Type == domain.AccountCredit {
			fmt.Fprintf(&b, "- Cartão %s: fatura atual %s", a.Name, brl(a.Balance))
			if a.Available != nil {
				fmt.Fprintf(&b, ", limite disponível %s", brl(*a.Available))
			}
			if a.BillDueDate != nil {
				fmt.Fprintf(&b, ", vence %s", a.BillDueDate.Format("02/01"))
			}
			b.WriteString("\n")
		} else {
			fmt.Fprintf(&b, "- Conta %s: saldo %s\n", a.Name, brl(a.Balance))
		}
	}
	if len(sum.Accounts) == 0 {
		b.WriteString("- nenhuma conta conectada ainda\n")
	}

	b.WriteString("\nCategorias no mês (total; limite; mês anterior):\n")
	for _, c := range sum.Categories {
		if c.Total == 0 && c.Limit == 0 {
			continue
		}
		name := c.Name
		if name == "" {
			name = "Sem categoria"
		}
		fmt.Fprintf(&b, "- %s [%s]: %s", name, c.Kind, brl(c.Total))
		if c.Limit > 0 {
			fmt.Fprintf(&b, "; limite %s (%d%% usado)", brl(c.Limit), percent(-c.Total, c.Limit))
		}
		if p, ok := sum.Previous[c.CategoryID]; ok && (c.CategoryID != "" || c.Total < 0) {
			fmt.Fprintf(&b, "; mês anterior %s", brl(p))
		}
		b.WriteString("\n")
	}

	if len(sum.Recurring) > 0 {
		b.WriteString("\nContas recorrentes (valor típico):\n")
		for _, r := range sum.Recurring {
			fmt.Fprintf(&b, "- %s: %s, visto em %d meses, último em %s", r.Name, brl(r.Amount), r.Months, r.LastDate.Format("02/01"))
			if r.Changed {
				fmt.Fprintf(&b, " (mudou: antes %s)", brl(r.Previous))
			}
			b.WriteString("\n")
		}
	}
	if len(sum.Installments) > 0 {
		b.WriteString("\nParcelas que ainda vão vencer:\n")
		for _, i := range sum.Installments {
			fmt.Fprintf(&b, "- %s: %s por mês, faltam %d\n", i.Description, brl(i.Amount), i.Remaining)
		}
	}

	spending, err := s.repo().Spending(ctx, userID, from, from.AddDate(0, 1, 0), spendingCap)
	if err != nil {
		return "", err
	}
	slices.SortFunc(spending, func(a, b domain.Transaction) int { return int(a.Amount - b.Amount) })
	if len(spending) > 0 {
		b.WriteString("\nMaiores gastos do mês:\n")
		for _, t := range spending[:min(len(spending), topExpenses)] {
			cat := t.CategoryName
			if cat == "" {
				cat = "Sem categoria"
			}
			fmt.Fprintf(&b, "- %s %s: %s (%s)\n", t.Date.Format("02/01"), t.Description, brl(t.Amount), cat)
		}
	}
	return b.String(), nil
}

func percent(part, whole domain.Money) int64 {
	if whole <= 0 {
		return 0
	}
	return part * 100 / whole
}

func (s *Service) Chat(ctx context.Context, userID string, history []domain.ChatMessage) (string, error) {
	if s.AI == nil {
		return "", disabled("assistant")
	}
	if len(history) == 0 || history[len(history)-1].Role != "user" {
		return "", apperror.New(apperror.CodeInvalidInput, "the last message must be the user's").WithField("messages")
	}
	history = history[max(0, len(history)-maxChatTurns):]
	facts, err := s.financialContext(ctx, userID, s.Clock.Now())
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("DADOS (calculados pelo app):\n")
	b.WriteString(facts)
	b.WriteString("\nCONVERSA:\n")
	for _, m := range history {
		text := strings.TrimSpace(validate.Text(m.Content, maxChatMessage))
		if text == "" {
			return "", apperror.New(apperror.CodeInvalidInput, "empty message").WithField("messages")
		}
		switch m.Role {
		case "user":
			b.WriteString("Usuário: ")
		case "assistant":
			b.WriteString("Fin: ")
		default:
			return "", apperror.New(apperror.CodeInvalidInput, "role must be user or assistant").WithField("messages")
		}
		b.WriteString(text)
		b.WriteString("\n")
	}
	b.WriteString("\nResponda à última mensagem do usuário.")
	return s.AI.Ask(ctx, systemPrompt, b.String())
}
