package finance

import (
	"testing"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
)

func TestAssistantToolCallsAndActions(t *testing.T) {
	// Small models wrap the call in a fence or a sentence.
	c, ok := parseToolCall("Vou buscar:\n```json\n{\"ferramenta\":\"buscar_transacoes\",\"texto\":\"ifood\",\"de\":\"2026-08-01\"}\n```")
	if !ok || c.Name != "buscar_transacoes" || c.Text != "ifood" || c.From != "2026-08-01" {
		t.Fatalf("tool call %+v %v", c, ok)
	}
	if _, ok := parseToolCall(`{"ferramenta":"apagar_tudo"}`); ok {
		t.Fatal("unknown tool accepted")
	}
	if _, ok := parseToolCall("Gastou R$ 50 {no mês}."); ok {
		t.Fatal("plain answer taken as a tool call")
	}

	cats := []domain.Category{{ID: "c1", Name: "Delivery", Kind: "expense"}}
	answer := "Resposta.\n" +
		`AÇÃO {"tipo":"regra","palavra":"iFood","categoria":"delivery"}` + "\n" +
		`- AÇÃO {"tipo":"regra","palavra":"uber","categoria":"Inexistente"}` + "\n" +
		`ACAO: {"tipo":"assinatura","nome":"Netflix","valor":"55,90","dia":3,"palavra":"netflix"}` + "\n" +
		`AÇÃO {"tipo":"assinatura","nome":"X","valor":0,"dia":3,"palavra":"xx"}` + "\n" +
		`AÇÃO {"tipo":"transferir","palavra":"pix"}`
	text, actions := parseActions(answer, cats)
	if text != "Resposta." {
		t.Fatalf("text %q", text)
	}
	want := []domain.ChatAction{
		{Kind: domain.ActionRule, Keyword: "iFood", CategoryID: "c1", CategoryName: "Delivery"},
		{Kind: domain.ActionSubscription, Keyword: "netflix", Name: "Netflix", Amount: 5590, Day: 3},
	}
	if len(actions) != len(want) || actions[0] != want[0] || actions[1] != want[1] {
		t.Fatalf("actions %+v", actions)
	}

	for in, cents := range map[any]domain.Money{"R$ 1.234,56": 123456, 55.9: 5590, "12.5": 1250} {
		if got, ok := moneyValue(in); !ok || got != cents {
			t.Fatalf("money %v: %d %v", in, got, ok)
		}
	}
}

func TestWeekFacts(t *testing.T) {
	day := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC) // a Monday
	at := func(d int) time.Time { return day.AddDate(0, 0, d) }
	spending := []domain.Transaction{
		{Date: at(-1), Amount: -3000, CategoryName: "Mercado"},
		{Date: at(-7), Amount: -1000},                          // first day of the week: counts
		{Date: at(-8), Amount: -500, CategoryName: "Mercado"},  // the week before
		{Date: at(0), Amount: -9999, CategoryName: "Mercado"},  // today: not yet
		{Date: at(-15), Amount: -7777, CategoryName: "Viagem"}, // too old
	}
	text, ok := weekFacts(spending, day)
	want := "Gastou R$ 40,00 nos últimos 7 dias (R$ 5,00 na semana anterior). Mais em: Mercado R$ 30,00, Sem categoria R$ 10,00."
	if !ok || text != want {
		t.Fatalf("got %q", text)
	}
	if _, ok := weekFacts(spending[2:3], day); ok {
		t.Fatal("a week without spending sends nothing")
	}
}
