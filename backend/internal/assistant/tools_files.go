package assistant

import (
	"context"
	"strings"
)

func (r *Registry) addFiles() {
	d := r.deps

	if d.Documents != nil {
		r.add(Tool{
			Name: "documents_search", Title: "Buscar documentos", Module: "documentos", ReadOnly: true,
			Description: "Procura arquivos guardados em Documentos pelo nome, em todas as pastas.",
			Input:       object(map[string]any{"query": str("Parte do nome do arquivo")}, "query"),
			run: typed(func(ctx context.Context, in struct {
				Query string `json:"query"`
			}) (any, error) {
				folders, err := d.Documents.ListFolders(ctx)
				if err != nil {
					return nil, err
				}
				names := map[string]string{}
				ids := []*string{nil}
				for _, f := range folders {
					names[f.ID] = f.Name
					ids = append(ids, &f.ID)
				}
				type row struct {
					Name   string `json:"arquivo"`
					Folder string `json:"pasta"`
					Size   int64  `json:"bytes"`
					Added  string `json:"enviado"`
				}
				q := normalize(in.Query)
				out := []row{}
				for _, id := range ids {
					docs, err := d.Documents.List(ctx, id)
					if err != nil {
						return nil, err
					}
					for _, doc := range docs {
						if q != "" && !strings.Contains(normalize(doc.Name), q) {
							continue
						}
						folder := "Início"
						if id != nil {
							folder = names[*id]
						}
						out = append(out, row{doc.Name, folder, doc.SizeBytes, doc.CreatedAt.In(r.deps.Location).Format(dayLayout)})
					}
				}
				return map[string]any{"documentos": out}, nil
			}),
		})
	}

	if d.Boards != nil {
		r.add(Tool{
			Name: "boards_list", Title: "Quadros", Module: "quadros", ReadOnly: true,
			Description: "Lista os quadros (desenhos e fluxos) pelo nome e data da última edição.",
			Input:       object(map[string]any{}),
			run: typed(func(ctx context.Context, _ struct{}) (any, error) {
				boards, err := d.Boards.List(ctx)
				if err != nil {
					return nil, err
				}
				type row struct {
					Name    string `json:"nome"`
					Updated string `json:"editado"`
				}
				out := make([]row, len(boards))
				for i, b := range boards {
					out[i] = row{b.Name, b.UpdatedAt.In(r.deps.Location).Format("2006-01-02 15:04")}
				}
				return map[string]any{"quadros": out}, nil
			}),
		})
	}
}

// addOverview is the "how's my day" tool: one call gathering today across
// modules, so a model (or the chat) can answer broad questions cheaply.
func (r *Registry) addOverview() {
	r.add(Tool{
		Name: "overview_today", Title: "Resumo de hoje", Module: "geral", ReadOnly: true,
		Description: "Um panorama de hoje: gastos do mês, contas em aberto e atrasadas, lembretes, compromissos, hábitos, treino e dieta.",
		Input:       object(map[string]any{}),
		run: typed(func(ctx context.Context, _ struct{}) (any, error) {
			out := map[string]any{"hoje": r.today().Format(dayLayout), "agora": r.now().Format("15:04")}
			for key, name := range map[string]string{
				"gastos_do_mes": "finance_month_summary",
				"contas":        "bills_summary",
				"lembretes":     "reminders_list",
				"compromissos":  "agenda_list",
				"habitos":       "habits_today",
				"treino":        "training_day",
				"dieta":         "diet_day",
			} {
				if _, ok := r.byName[name]; !ok {
					continue
				}
				args := `{}`
				if name == "agenda_list" {
					args = `{"days": 2}`
				}
				res, err := r.Call(ctx, name, []byte(args))
				if err != nil {
					out[key] = map[string]string{"erro": err.Error()}
					continue
				}
				if name == "finance_month_summary" {
					if m, ok := res.(map[string]any); ok {
						res = map[string]any{"total": m["total"], "mes_anterior": m["mes_anterior"]}
					}
				}
				out[key] = res
			}
			return out, nil
		}),
	})
}
