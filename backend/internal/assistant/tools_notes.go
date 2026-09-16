package assistant

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// textDoc turns plain text into the notes editor's document: one paragraph
// per line, so a note written by a model opens nicely in the app.
func textDoc(text string) json.RawMessage {
	type node map[string]any
	var content []node
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			content = append(content, node{"type": "paragraph"})
			continue
		}
		content = append(content, node{"type": "paragraph", "content": []node{{"type": "text", "text": line}}})
	}
	b, _ := json.Marshal(node{"type": "doc", "content": content})
	return b
}

func (r *Registry) addNotes() {
	d := r.deps
	if d.Notes == nil {
		return
	}

	notebookName := func(ctx context.Context, id *string) string {
		if id == nil || d.NoteCategories == nil {
			return "Geral"
		}
		cats, err := d.NoteCategories.List(ctx)
		if err != nil {
			return ""
		}
		for _, c := range cats {
			if c.ID == *id {
				return c.Name
			}
		}
		return "Geral"
	}

	r.add(Tool{
		Name: "notes_search", Title: "Buscar notas", Module: "notas", ReadOnly: true,
		Description: "Busca notas por texto no título ou no conteúdo (vazio lista as mais recentes). Traz um trecho de cada.",
		Input: object(map[string]any{
			"query":    str("Texto a procurar"),
			"notebook": str("Filtrar por caderno"),
			"limit":    integer("Máximo de notas; padrão 10"),
		}),
		run: typed(func(ctx context.Context, in struct {
			Query    string `json:"query"`
			Notebook string `json:"notebook"`
			Limit    int    `json:"limit"`
		}) (any, error) {
			notes, err := d.Notes.List(ctx)
			if err != nil {
				return nil, err
			}
			limit := in.Limit
			if limit <= 0 || limit > 50 {
				limit = 10
			}
			q := normalize(in.Query)
			type row struct {
				ID       string `json:"id"`
				Title    string `json:"titulo"`
				Notebook string `json:"caderno"`
				Excerpt  string `json:"trecho"`
				Updated  string `json:"atualizada"`
				Pinned   bool   `json:"fixada,omitempty"`
			}
			out := []row{}
			for _, n := range notes {
				if q != "" && !strings.Contains(normalize(n.Title+" "+n.Body), q) {
					continue
				}
				nb := notebookName(ctx, n.CategoryID)
				if in.Notebook != "" && normalize(nb) != normalize(in.Notebook) {
					continue
				}
				out = append(out, row{n.ID, n.Title, nb, truncate(strings.Join(strings.Fields(n.Body), " "), 160), n.UpdatedAt.In(r.deps.Location).Format("2006-01-02 15:04"), n.Pinned})
				if len(out) >= limit {
					break
				}
			}
			return map[string]any{"notas": out}, nil
		}),
	})

	r.add(Tool{
		Name: "notes_read", Title: "Ler nota", Module: "notas", ReadOnly: true,
		Description: "Lê o texto completo de uma nota pelo id.",
		Input:       object(map[string]any{"id": str("Id da nota")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			n, err := d.Notes.Get(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": n.ID, "titulo": n.Title, "caderno": notebookName(ctx, n.CategoryID), "texto": truncate(n.Body, 12000)}, nil
		}),
	})

	r.add(Tool{
		Name: "notes_create", Title: "Nova nota", Module: "notas",
		Description: "Cria uma nota com título e texto, opcionalmente num caderno existente.",
		Input: object(map[string]any{
			"title":    str("Título"),
			"text":     str("Texto da nota; quebras de linha viram parágrafos"),
			"notebook": str("Nome do caderno (vazio = Geral)"),
			"pinned":   boolean("Fixar no topo"),
		}, "title"),
		run: typed(func(ctx context.Context, in struct {
			Title    string `json:"title"`
			Text     string `json:"text"`
			Notebook string `json:"notebook"`
			Pinned   bool   `json:"pinned"`
		}) (any, error) {
			note := domain.Note{Title: strings.TrimSpace(in.Title), Body: strings.TrimSpace(in.Text), Content: textDoc(in.Text), Pinned: in.Pinned}
			if strings.TrimSpace(in.Notebook) != "" && normalize(in.Notebook) != "geral" && d.NoteCategories != nil {
				cats, err := d.NoteCategories.List(ctx)
				if err != nil {
					return nil, err
				}
				c, ok, names := match(cats, in.Notebook, func(c domain.NoteCategory) string { return c.ID }, func(c domain.NoteCategory) string { return c.Name })
				if !ok {
					return nil, invalid("caderno %q não encontrado; cadernos: %s", in.Notebook, strings.Join(names, ", "))
				}
				note.CategoryID = &c.ID
			}
			saved, err := d.Notes.Create(ctx, note)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": saved.ID, "titulo": saved.Title, "caderno": notebookName(ctx, saved.CategoryID)}, nil
		}),
	})

	r.add(Tool{
		Name: "notes_append", Title: "Acrescentar à nota", Module: "notas",
		Description: "Acrescenta texto ao fim de uma nota existente, sem apagar o que já tem.",
		Input: object(map[string]any{
			"id":   str("Id da nota"),
			"text": str("Texto a acrescentar"),
		}, "id", "text"),
		run: typed(func(ctx context.Context, in struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		}) (any, error) {
			n, err := d.Notes.Get(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			if len(n.Content) > 0 {
				var doc map[string]any
				if err := json.Unmarshal(n.Content, &doc); err == nil {
					extra := map[string]any{}
					_ = json.Unmarshal(textDoc(in.Text), &extra)
					existing, _ := doc["content"].([]any)
					added, _ := extra["content"].([]any)
					doc["content"] = append(existing, added...)
					n.Content, _ = json.Marshal(doc)
				}
			} else {
				n.Content = textDoc(strings.TrimRight(n.Body, "\n") + "\n" + in.Text)
			}
			n.Body = strings.TrimSpace(n.Body + "\n" + in.Text)
			saved, err := d.Notes.Update(ctx, n.ID, n)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": saved.ID, "titulo": saved.Title}, nil
		}),
	})

	r.add(Tool{
		Name: "notes_delete", Title: "Excluir nota", Module: "notas", Destructive: true,
		Description: "Exclui uma nota pelo id.",
		Input:       object(map[string]any{"id": str("Id da nota")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			return map[string]any{"ok": true}, d.Notes.Delete(ctx, in.ID)
		}),
	})
}
