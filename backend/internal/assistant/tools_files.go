package assistant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func (r *Registry) addFiles() {
	d := r.deps

	if d.Documents != nil {
		r.add(Tool{
			Name: "documents_search", Title: "Buscar documentos", Module: "documentos", ReadOnly: true,
			Description: "Procura arquivos guardados em Documentos pelo nome, em todas as pastas (vazio lista todos). Traz o id de cada um, para usar em documents_read, documents_move, documents_delete ou edit_image.",
			Input: object(map[string]any{
				"query":  str("Parte do nome do arquivo"),
				"folder": str("Só nesta pasta (nome, caminho ou id; Início = raiz)"),
			}),
			run: typed(func(ctx context.Context, in struct {
				Query  string `json:"query"`
				Folder string `json:"folder"`
			}) (any, error) {
				folders, err := d.Documents.ListFolders(ctx)
				if err != nil {
					return nil, err
				}
				paths := folderPaths(folders)
				ids := []*string{nil}
				for _, f := range folders {
					ids = append(ids, &f.ID)
				}
				if strings.TrimSpace(in.Folder) != "" {
					id, err := findFolder(folders, in.Folder)
					if err != nil {
						return nil, err
					}
					ids = []*string{id}
				}
				type row struct {
					ID     string `json:"id"`
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
						folder := rootFolder
						if id != nil {
							folder = paths[*id]
						}
						out = append(out, row{doc.ID, doc.Name, folder, doc.SizeBytes, doc.CreatedAt.In(r.deps.Location).Format(dayLayout)})
					}
				}
				return map[string]any{"documentos": out}, nil
			}),
		})

		r.add(Tool{
			Name: "documents_read", Title: "Ler documento", Module: "documentos", ReadOnly: true,
			Description: "Lê o conteúdo de um documento pelo id — texto direto, texto extraído de um PDF, ou as linhas de uma planilha. Use o id de um anexo da conversa ou de documents_search.",
			Input:       object(map[string]any{"document_id": str("Id do documento")}, "document_id"),
			run: typed(func(ctx context.Context, in struct {
				DocumentID string `json:"document_id"`
			}) (any, error) {
				doc, file, err := d.Documents.Open(ctx, in.DocumentID)
				if err != nil {
					return nil, err
				}
				defer file.Close()
				data, err := io.ReadAll(io.LimitReader(file, maxAttachmentBytes))
				if err != nil {
					return nil, fmt.Errorf("ler documento: %w", err)
				}
				return map[string]any{"nome": doc.Name, "conteudo": describeDocument(doc, data)}, nil
			}),
		})

		r.add(Tool{
			Name: "documents_write", Title: "Criar arquivo", Module: "documentos",
			Description: "Cria um arquivo em Documentos a partir de texto que você escreveu — .txt e .pdf a partir de 'content', .xlsx a partir de 'rows' (uma lista de linhas, cada linha uma lista de células). Use para gerar um relatório, reescrever um texto anexado, ou devolver uma planilha alterada.",
			Input: object(map[string]any{
				"name":    str("Nome do arquivo, sem extensão"),
				"format":  enum("Formato de saída", "txt", "pdf", "xlsx"),
				"content": str("Texto do arquivo, já escrito por você — para xlsx, mande uma string vazia"),
				"rows":    array("Linhas da planilha — só para xlsx", array("célula", str("valor da célula"))),
				"folder":  str("Pasta onde salvar (nome, caminho ou id); padrão Início"),
				// content marked required (not just required for txt/pdf): a
				// schema that only requires it conditionally let a small
				// model (confirmed live with Apple Intelligence) omit it
				// every time, hit "file is empty" from Document.Validate,
				// and retry identically instead of correcting.
			}, "name", "format", "content"),
			run: typed(func(ctx context.Context, in struct {
				Name    string     `json:"name"`
				Format  string     `json:"format"`
				Content string     `json:"content"`
				Rows    [][]string `json:"rows"`
				Folder  string     `json:"folder"`
			}) (any, error) {
				name := strings.TrimSpace(in.Name)
				if name == "" {
					return nil, invalid("nome é obrigatório")
				}
				var folderID *string
				if strings.TrimSpace(in.Folder) != "" {
					folders, err := d.Documents.ListFolders(ctx)
					if err != nil {
						return nil, err
					}
					if folderID, err = findFolder(folders, in.Folder); err != nil {
						return nil, err
					}
				}
				var (
					data             []byte
					contentType, ext string
					err              error
				)
				switch in.Format {
				case "txt":
					if strings.TrimSpace(in.Content) == "" {
						return nil, invalid("content é obrigatório para txt")
					}
					data, contentType, ext = []byte(in.Content), "text/plain", ".txt"
				case "pdf":
					if strings.TrimSpace(in.Content) == "" {
						return nil, invalid("content é obrigatório para pdf")
					}
					data, err = renderPDF(in.Content)
					contentType, ext = "application/pdf", ".pdf"
				case "xlsx":
					data, err = renderXLSX(in.Rows)
					contentType, ext = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx"
				default:
					return nil, invalid("format precisa ser txt, pdf ou xlsx")
				}
				if err != nil {
					return nil, fmt.Errorf("gerar arquivo: %w", err)
				}
				if !strings.HasSuffix(strings.ToLower(name), ext) {
					name += ext
				}
				doc, err := d.Documents.Save(ctx, folderID, name, contentType, bytes.NewReader(data))
				if err != nil {
					return nil, fmt.Errorf("salvar arquivo: %w", err)
				}
				return map[string]any{"document_id": doc.ID, "nome": doc.Name}, nil
			}),
		})

		r.add(Tool{
			Name: "documents_folders", Title: "Pastas de documentos", Module: "documentos", ReadOnly: true,
			Description: "Lista as pastas de Documentos com id e caminho completo. Início é a raiz, sem id.",
			Input:       object(map[string]any{}),
			run: typed(func(ctx context.Context, _ struct{}) (any, error) {
				folders, err := d.Documents.ListFolders(ctx)
				if err != nil {
					return nil, err
				}
				paths := folderPaths(folders)
				type row struct {
					ID   string `json:"id"`
					Path string `json:"pasta"`
				}
				out := make([]row, len(folders))
				for i, f := range folders {
					out[i] = row{f.ID, paths[f.ID]}
				}
				sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
				return map[string]any{"pastas": out}, nil
			}),
		})

		r.add(Tool{
			Name: "documents_folder_create", Title: "Nova pasta", Module: "documentos",
			Description: "Cria uma pasta em Documentos, na raiz ou dentro de outra pasta.",
			Input: object(map[string]any{
				"name":   str("Nome da pasta"),
				"parent": str("Pasta onde criar (nome, caminho ou id); padrão Início"),
			}, "name"),
			run: typed(func(ctx context.Context, in struct {
				Name   string `json:"name"`
				Parent string `json:"parent"`
			}) (any, error) {
				folders, err := d.Documents.ListFolders(ctx)
				if err != nil {
					return nil, err
				}
				parent, err := findFolder(folders, in.Parent)
				if err != nil {
					return nil, err
				}
				f, err := d.Documents.CreateFolder(ctx, parent, strings.TrimSpace(in.Name))
				if err != nil {
					return nil, err
				}
				return map[string]any{"id": f.ID, "pasta": folderPaths(append(folders, f))[f.ID]}, nil
			}),
		})

		r.add(Tool{
			Name: "documents_folder_rename", Title: "Renomear pasta", Module: "documentos",
			Description: "Renomeia uma pasta de Documentos.",
			Input: object(map[string]any{
				"folder": str("Pasta (nome, caminho ou id)"),
				"name":   str("Novo nome"),
			}, "folder", "name"),
			run: typed(func(ctx context.Context, in struct {
				Folder string `json:"folder"`
				Name   string `json:"name"`
			}) (any, error) {
				folders, err := d.Documents.ListFolders(ctx)
				if err != nil {
					return nil, err
				}
				id, err := findFolder(folders, in.Folder)
				if err != nil {
					return nil, err
				}
				if id == nil {
					return nil, invalid("Início não pode ser renomeado")
				}
				f, err := d.Documents.RenameFolder(ctx, *id, strings.TrimSpace(in.Name))
				if err != nil {
					return nil, err
				}
				return map[string]any{"id": f.ID, "nome": f.Name}, nil
			}),
		})

		r.add(Tool{
			Name: "documents_folder_delete", Title: "Excluir pasta", Module: "documentos", Destructive: true,
			Description: "Exclui uma pasta de Documentos. Só funciona com a pasta vazia — mova ou exclua o que estiver nela antes.",
			Input:       object(map[string]any{"folder": str("Pasta (nome, caminho ou id)")}, "folder"),
			run: typed(func(ctx context.Context, in struct {
				Folder string `json:"folder"`
			}) (any, error) {
				folders, err := d.Documents.ListFolders(ctx)
				if err != nil {
					return nil, err
				}
				id, err := findFolder(folders, in.Folder)
				if err != nil {
					return nil, err
				}
				if id == nil {
					return nil, invalid("Início não pode ser excluído")
				}
				if err := d.Documents.DeleteFolder(ctx, *id); err != nil {
					if errors.Is(err, domain.ErrConflict) {
						return nil, invalid("a pasta não está vazia; mova ou exclua o conteúdo antes")
					}
					return nil, err
				}
				return map[string]any{"ok": true}, nil
			}),
		})

		r.add(Tool{
			Name: "documents_move", Title: "Mover ou renomear documento", Module: "documentos",
			Description: "Move um documento para outra pasta e/ou muda o nome dele. Campo omitido fica como está.",
			Input: object(map[string]any{
				"document_id": str("Id do documento"),
				"folder":      str("Pasta de destino (nome, caminho ou id; Início = raiz)"),
				"name":        str("Novo nome do arquivo, com extensão"),
			}, "document_id"),
			run: typed(func(ctx context.Context, in struct {
				DocumentID string `json:"document_id"`
				Folder     string `json:"folder"`
				Name       string `json:"name"`
			}) (any, error) {
				doc, err := d.Documents.Get(ctx, strings.TrimSpace(in.DocumentID))
				if err != nil {
					return nil, err
				}
				folders, err := d.Documents.ListFolders(ctx)
				if err != nil {
					return nil, err
				}
				folderID := doc.FolderID
				if strings.TrimSpace(in.Folder) != "" {
					if folderID, err = findFolder(folders, in.Folder); err != nil {
						return nil, err
					}
				}
				name := doc.Name
				if strings.TrimSpace(in.Name) != "" {
					name = strings.TrimSpace(in.Name)
				}
				moved, err := d.Documents.Move(ctx, doc.ID, folderID, name)
				if err != nil {
					return nil, err
				}
				folder := rootFolder
				if moved.FolderID != nil {
					folder = folderPaths(folders)[*moved.FolderID]
				}
				return map[string]any{"document_id": moved.ID, "nome": moved.Name, "pasta": folder}, nil
			}),
		})

		r.add(Tool{
			Name: "documents_delete", Title: "Excluir documento", Module: "documentos", Destructive: true,
			Description: "Exclui um documento pelo id, com o arquivo.",
			Input:       object(map[string]any{"document_id": str("Id do documento")}, "document_id"),
			run: typed(func(ctx context.Context, in struct {
				DocumentID string `json:"document_id"`
			}) (any, error) {
				return map[string]any{"ok": true}, d.Documents.Delete(ctx, strings.TrimSpace(in.DocumentID))
			}),
		})
	}

	if d.Boards != nil {
		r.add(Tool{
			Name: "boards_list", Title: "Quadros", Module: "quadros", ReadOnly: true,
			Description: "Lista os quadros (desenhos e fluxos) com id, nome e data da última edição.",
			Input:       object(map[string]any{}),
			run: typed(func(ctx context.Context, _ struct{}) (any, error) {
				boards, err := d.Boards.List(ctx)
				if err != nil {
					return nil, err
				}
				type row struct {
					ID      string `json:"id"`
					Name    string `json:"nome"`
					Updated string `json:"editado"`
				}
				out := make([]row, len(boards))
				for i, b := range boards {
					out[i] = row{b.ID, b.Name, b.UpdatedAt.In(r.deps.Location).Format("2006-01-02 15:04")}
				}
				return map[string]any{"quadros": out}, nil
			}),
		})

		findBoard := func(ctx context.Context, query string) (domain.Board, error) {
			boards, err := d.Boards.List(ctx)
			if err != nil {
				return domain.Board{}, err
			}
			b, ok, names := match(boards, query, func(b domain.Board) string { return b.ID }, func(b domain.Board) string { return b.Name })
			if !ok {
				return domain.Board{}, invalid("quadro %q não encontrado (ou ambíguo); quadros: %s", query, strings.Join(names, ", "))
			}
			return b, nil
		}

		r.add(Tool{
			Name: "boards_create", Title: "Novo quadro", Module: "quadros",
			Description: "Cria um quadro em branco com um nome. O desenho é feito no app.",
			Input:       object(map[string]any{"name": str("Nome do quadro")}, "name"),
			run: typed(func(ctx context.Context, in struct {
				Name string `json:"name"`
			}) (any, error) {
				b, err := d.Boards.Create(ctx, in.Name)
				if err != nil {
					return nil, err
				}
				return map[string]any{"id": b.ID, "nome": b.Name}, nil
			}),
		})

		r.add(Tool{
			Name: "boards_rename", Title: "Renomear quadro", Module: "quadros",
			Description: "Muda o nome de um quadro.",
			Input: object(map[string]any{
				"board": str("Quadro (nome ou id)"),
				"name":  str("Novo nome"),
			}, "board", "name"),
			run: typed(func(ctx context.Context, in struct {
				Board string `json:"board"`
				Name  string `json:"name"`
			}) (any, error) {
				b, err := findBoard(ctx, in.Board)
				if err != nil {
					return nil, err
				}
				if err := d.Boards.Rename(ctx, b.ID, in.Name); err != nil {
					return nil, err
				}
				return map[string]any{"id": b.ID, "nome": strings.TrimSpace(in.Name)}, nil
			}),
		})

		r.add(Tool{
			Name: "boards_delete", Title: "Excluir quadro", Module: "quadros", Destructive: true,
			Description: "Exclui um quadro e o desenho dele.",
			Input:       object(map[string]any{"board": str("Quadro (nome ou id)")}, "board"),
			run: typed(func(ctx context.Context, in struct {
				Board string `json:"board"`
			}) (any, error) {
				b, err := findBoard(ctx, in.Board)
				if err != nil {
					return nil, err
				}
				if err := d.Boards.Delete(ctx, b.ID); err != nil {
					return nil, err
				}
				return map[string]any{"ok": true, "nome": b.Name}, nil
			}),
		})
	}
}

const rootFolder = "Início"

// folderPaths maps each folder id to its full path ("Casa/Contas"), so
// nested folders with the same name stay distinguishable. A cycle or a
// missing parent just stops the walk rather than looping.
func folderPaths(folders []domain.DocumentFolder) map[string]string {
	byID := make(map[string]domain.DocumentFolder, len(folders))
	for _, f := range folders {
		byID[f.ID] = f
	}
	out := make(map[string]string, len(folders))
	for _, f := range folders {
		parts := []string{f.Name}
		seen := map[string]bool{f.ID: true}
		for p := f.ParentID; p != nil && !seen[*p]; {
			parent, ok := byID[*p]
			if !ok {
				break
			}
			seen[parent.ID] = true
			parts = append([]string{parent.Name}, parts...)
			p = parent.ParentID
		}
		out[f.ID] = strings.Join(parts, "/")
	}
	return out
}

// findFolder resolves a folder by id, full path or name; empty or "Início"
// is the root (nil). Path is tried before bare name because two folders
// may share a name under different parents.
func findFolder(folders []domain.DocumentFolder, query string) (*string, error) {
	q := strings.Trim(strings.TrimSpace(query), "/")
	if q == "" || normalize(q) == normalize(rootFolder) {
		return nil, nil
	}
	paths := folderPaths(folders)
	for _, f := range folders {
		if f.ID == q || normalize(paths[f.ID]) == normalize(q) {
			return &f.ID, nil
		}
	}
	f, ok, _ := match(folders, q, func(f domain.DocumentFolder) string { return f.ID }, func(f domain.DocumentFolder) string { return f.Name })
	if !ok {
		all := make([]string, 0, len(paths))
		for _, p := range paths {
			all = append(all, p)
		}
		sort.Strings(all)
		return nil, invalid("pasta %q não encontrada (ou ambígua); pastas: %s", query, strings.Join(all, ", "))
	}
	return &f.ID, nil
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

// renderPDF lays text out as simple paragraphs, one per line of content —
// enough for a report or a rewritten document, not a layout tool. cp1252
// covers the accents Portuguese needs without embedding a font.
func renderPDF(content string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	for _, line := range strings.Split(content, "\n") {
		pdf.MultiCell(0, 6, tr(line), "", "", false)
	}
	if err := pdf.Error(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderXLSX(rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	for r, row := range rows {
		for c, cell := range row {
			colName, err := excelize.ColumnNumberToName(c + 1)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue("Sheet1", fmt.Sprintf("%s%d", colName, r+1), cell); err != nil {
				return nil, err
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
