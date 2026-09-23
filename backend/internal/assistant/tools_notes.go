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

	// resolveNotebook turns a notebook name into its id; empty or "Geral"
	// is no notebook (nil).
	resolveNotebook := func(ctx context.Context, name string) (*string, error) {
		if strings.TrimSpace(name) == "" || normalize(name) == "geral" || d.NoteCategories == nil {
			return nil, nil
		}
		cats, err := d.NoteCategories.List(ctx)
		if err != nil {
			return nil, err
		}
		c, ok, names := match(cats, name, func(c domain.NoteCategory) string { return c.ID }, func(c domain.NoteCategory) string { return c.Name })
		if !ok {
			return nil, invalid("caderno %q não encontrado; cadernos: %s", name, strings.Join(names, ", "))
		}
		return &c.ID, nil
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
			var err error
			if note.CategoryID, err = resolveNotebook(ctx, in.Notebook); err != nil {
				return nil, err
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
		Name: "notes_update", Title: "Editar nota", Module: "notas",
		Description: "Altera uma nota pelo id: título, texto (substitui o texto inteiro), caderno ou fixação. Só os campos enviados mudam; para só acrescentar texto use notes_append.",
		Input: object(map[string]any{
			"id":       str("Id da nota"),
			"title":    str("Novo título"),
			"text":     str("Novo texto completo; quebras de linha viram parágrafos"),
			"notebook": str("Mover para este caderno (Geral = sem caderno)"),
			"pinned":   boolean("Fixar (true) ou desafixar (false)"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID       string  `json:"id"`
			Title    *string `json:"title"`
			Text     *string `json:"text"`
			Notebook *string `json:"notebook"`
			Pinned   *bool   `json:"pinned"`
		}) (any, error) {
			n, err := d.Notes.Get(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			if in.Title != nil {
				n.Title = strings.TrimSpace(*in.Title)
			}
			if in.Text != nil {
				n.Body = strings.TrimSpace(*in.Text)
				n.Content = textDoc(*in.Text)
			}
			if in.Notebook != nil {
				if n.CategoryID, err = resolveNotebook(ctx, *in.Notebook); err != nil {
					return nil, err
				}
			}
			if in.Pinned != nil {
				n.Pinned = *in.Pinned
			}
			saved, err := d.Notes.Update(ctx, n.ID, n)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": saved.ID, "titulo": saved.Title, "caderno": notebookName(ctx, saved.CategoryID), "fixada": saved.Pinned}, nil
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

	if d.NoteCategories != nil {
		r.addNotebooks()
	}
}

// notebookColors is the notes screen's palette (frontend/app/notas/actions.ts),
// so a notebook made by chat looks like one made in the app.
var notebookColors = []string{"#8b5cf6", "#3b7fe0", "#10a37f", "#e0972f", "#d94f70", "#0f9aa8", "#7cc242"}

// nextNotebookColor takes the first palette color no notebook has yet, as
// the app does.
func nextNotebookColor(used []string) string {
	taken := map[string]bool{}
	for _, c := range used {
		taken[strings.ToLower(c)] = true
	}
	for _, c := range notebookColors {
		if !taken[c] {
			return c
		}
	}
	return notebookColors[len(used)%len(notebookColors)]
}

func (r *Registry) addNotebooks() {
	d := r.deps

	find := func(ctx context.Context, query string) (domain.NoteCategory, error) {
		cats, err := d.NoteCategories.List(ctx)
		if err != nil {
			return domain.NoteCategory{}, err
		}
		c, ok, names := match(cats, query, func(c domain.NoteCategory) string { return c.ID }, func(c domain.NoteCategory) string { return c.Name })
		if !ok {
			return domain.NoteCategory{}, invalid("caderno %q não encontrado (ou ambíguo); cadernos: %s", query, strings.Join(names, ", "))
		}
		return c, nil
	}

	r.add(Tool{
		Name: "notes_notebooks", Title: "Cadernos", Module: "notas", ReadOnly: true,
		Description: "Lista os cadernos de notas com id e quantas notas cada um tem. Geral agrupa as notas sem caderno.",
		Input:       object(map[string]any{}),
		run: typed(func(ctx context.Context, _ struct{}) (any, error) {
			cats, err := d.NoteCategories.List(ctx)
			if err != nil {
				return nil, err
			}
			count := map[string]int{}
			if d.Notes != nil {
				notes, err := d.Notes.List(ctx)
				if err != nil {
					return nil, err
				}
				for _, n := range notes {
					if n.CategoryID == nil {
						count[""]++
					} else {
						count[*n.CategoryID]++
					}
				}
			}
			type row struct {
				ID    string `json:"id,omitempty"`
				Name  string `json:"caderno"`
				Color string `json:"cor,omitempty"`
				Notes int    `json:"notas"`
			}
			out := []row{{Name: "Geral", Notes: count[""]}}
			for _, c := range cats {
				out = append(out, row{c.ID, c.Name, c.Color, count[c.ID]})
			}
			return map[string]any{"cadernos": out}, nil
		}),
	})

	r.add(Tool{
		Name: "notes_notebook_create", Title: "Novo caderno", Module: "notas",
		Description: "Cria um caderno de notas.",
		Input: object(map[string]any{
			"name":  str("Nome do caderno"),
			"color": str("Cor em hex (#rrggbb); padrão a próxima da paleta do app"),
		}, "name"),
		run: typed(func(ctx context.Context, in struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}) (any, error) {
			name := strings.TrimSpace(in.Name)
			if normalize(name) == "geral" {
				return nil, invalid("Geral já existe: é onde ficam as notas sem caderno")
			}
			color := strings.TrimSpace(in.Color)
			if color == "" {
				cats, err := d.NoteCategories.List(ctx)
				if err != nil {
					return nil, err
				}
				used := make([]string, len(cats))
				for i, c := range cats {
					used[i] = c.Color
				}
				color = nextNotebookColor(used)
			}
			c, err := d.NoteCategories.Create(ctx, name, color)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": c.ID, "caderno": c.Name, "cor": c.Color}, nil
		}),
	})

	r.add(Tool{
		Name: "notes_notebook_rename", Title: "Renomear caderno", Module: "notas",
		Description: "Muda o nome e/ou a cor de um caderno de notas.",
		Input: object(map[string]any{
			"notebook": str("Caderno (nome ou id)"),
			"name":     str("Novo nome; vazio mantém"),
			"color":    str("Nova cor em hex (#rrggbb); vazio mantém"),
		}, "notebook"),
		run: typed(func(ctx context.Context, in struct {
			Notebook string `json:"notebook"`
			Name     string `json:"name"`
			Color    string `json:"color"`
		}) (any, error) {
			c, err := find(ctx, in.Notebook)
			if err != nil {
				return nil, err
			}
			name, color := c.Name, c.Color
			if strings.TrimSpace(in.Name) != "" {
				name = strings.TrimSpace(in.Name)
			}
			if strings.TrimSpace(in.Color) != "" {
				color = strings.TrimSpace(in.Color)
			}
			saved, err := d.NoteCategories.Update(ctx, c.ID, name, color)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": saved.ID, "caderno": saved.Name, "cor": saved.Color}, nil
		}),
	})

	r.add(Tool{
		Name: "notes_notebook_delete", Title: "Excluir caderno", Module: "notas", Destructive: true,
		Description: "Exclui um caderno. As notas dele não são apagadas: vão para Geral.",
		Input:       object(map[string]any{"notebook": str("Caderno (nome ou id)")}, "notebook"),
		run: typed(func(ctx context.Context, in struct {
			Notebook string `json:"notebook"`
		}) (any, error) {
			c, err := find(ctx, in.Notebook)
			if err != nil {
				return nil, err
			}
			if err := d.NoteCategories.Delete(ctx, c.ID); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "caderno": c.Name}, nil
		}),
	})
}
