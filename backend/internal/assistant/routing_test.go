package assistant

import (
	"slices"
	"testing"
)

// The message that exposed the problem: the owner asked for a habit, the
// small model was only ever handed the finance tools, and it booked a R$1,00
// expense called "Leitura" instead — then said the habit had been created.
func TestModulesForTheMessageThatCreatedAnExpenseInsteadOfAHabit(t *testing.T) {
	got := modulesFor("Crie um habito de ler todos os dias por 1h")
	if !slices.Contains(got, "habitos") {
		t.Errorf("modulesFor() = %v, want it to include habitos", got)
	}
	if slices.Contains(got, "financeiro") {
		t.Errorf("modulesFor() = %v, must not offer financeiro for a habit request", got)
	}
}

func TestModulesForRoutesEachSubject(t *testing.T) {
	cases := []struct {
		message string
		want    string
	}{
		{"gastei 50 no mercado", "financeiro"},
		{"quanto gastei esse mês?", "financeiro"},
		{"paguei a conta de luz", "contas"},
		{"quais contas vencem essa semana?", "contas"},
		{"me lembra de ligar pro dentista amanhã", "lembretes"},
		{"tenho algum compromisso quinta?", "agenda"},
		{"marca meu treino de peito de hoje", "treino"},
		{"o que eu comi no almoço?", "dieta"},
		{"cria uma nota sobre a reunião", "notas"},
		{"abre o quadro do fluxo de vendas", "quadros"},
		{"onde está meu comprovante de residência?", "documentos"},
		{"beber 2 litros de água todo dia", "habitos"},
	}
	for _, tc := range cases {
		t.Run(tc.message, func(t *testing.T) {
			if got := modulesFor(tc.message); !slices.Contains(got, tc.want) {
				t.Errorf("modulesFor(%q) = %v, want it to include %s", tc.message, got, tc.want)
			}
		})
	}
}

// "geral" holds the day-overview tool, which is useful whatever the subject.
func TestModulesForAlwaysIncludesGeral(t *testing.T) {
	for _, message := range []string{"gastei 50", "cria um hábito", "", "asdfgh"} {
		if got := modulesFor(message); !slices.Contains(got, "geral") {
			t.Errorf("modulesFor(%q) = %v, want it to include geral", message, got)
		}
	}
}

// A message about two subjects gets both, rather than the router guessing.
func TestModulesForKeepsEverySubjectItRecognises(t *testing.T) {
	got := modulesFor("paguei a conta de luz e marquei o hábito de leitura")
	for _, want := range []string{"contas", "habitos"} {
		if !slices.Contains(got, want) {
			t.Errorf("modulesFor() = %v, want it to include %s", got, want)
		}
	}
}

// Nothing recognisable falls back to the previous behaviour — the everyday
// case — rather than to an empty toolbox.
func TestModulesForFallsBackToMoneyWhenNothingMatches(t *testing.T) {
	got := modulesFor("me conta uma piada")
	for _, want := range []string{"geral", "financeiro", "contas"} {
		if !slices.Contains(got, want) {
			t.Errorf("modulesFor() = %v, want the fallback to include %s", got, want)
		}
	}
}

// Accents and case are how people actually type; the router must not care.
func TestModulesForIgnoresAccentsAndCase(t *testing.T) {
	for _, message := range []string{"CRIE UM HÁBITO", "crie um habito", "Crie um Hábito"} {
		if got := modulesFor(message); !slices.Contains(got, "habitos") {
			t.Errorf("modulesFor(%q) = %v, want habitos", message, got)
		}
	}
}
