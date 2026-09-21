package assistant

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
)

func (r *Registry) addFiles() {
	d := r.deps

	if d.Documents != nil {
		r.add(Tool{
			Name: "documents_search", Title: "Buscar documentos", Module: "documentos", ReadOnly: true,
			Description: "Procura arquivos guardados em Documentos pelo nome, em todas as pastas. Traz o id de cada um, para usar em documents_read ou edit_image.",
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
						folder := "Início"
						if id != nil {
							folder = names[*id]
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
			}) (any, error) {
				name := strings.TrimSpace(in.Name)
				if name == "" {
					return nil, invalid("nome é obrigatório")
				}
				var (data []byte
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
				doc, err := d.Documents.Save(ctx, nil, name, contentType, bytes.NewReader(data))
				if err != nil {
					return nil, fmt.Errorf("salvar arquivo: %w", err)
				}
				return map[string]any{"document_id": doc.ID, "nome": doc.Name}, nil
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
