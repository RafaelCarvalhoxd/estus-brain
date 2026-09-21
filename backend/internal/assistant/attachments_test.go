package assistant

import (
	"strings"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestIsLikelyText(t *testing.T) {
	if !isLikelyText([]byte("olá, isso é texto simples\ncom acentuação")) {
		t.Error("texto simples deveria ser reconhecido como texto")
	}
	if isLikelyText([]byte{0x50, 0x4b, 0x03, 0x04, 0x00, 0x00}) {
		t.Error("bytes com NUL não deveriam ser reconhecidos como texto")
	}
	if !isLikelyText(nil) {
		t.Error("vazio deveria ser tratado como texto (nada para recusar)")
	}
}

func TestIsSpreadsheet(t *testing.T) {
	cases := []struct {
		contentType, name string
		want              bool
	}{
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "planilha", true},
		{"application/octet-stream", "gastos.xlsx", true},
		{"application/octet-stream", "gastos.XLSX", true},
		{"text/csv", "gastos.csv", false},
		{"application/pdf", "relatorio.pdf", false},
	}
	for _, c := range cases {
		if got := isSpreadsheet(c.contentType, c.name); got != c.want {
			t.Errorf("isSpreadsheet(%q, %q) = %v, want %v", c.contentType, c.name, got, c.want)
		}
	}
}

func TestDescribeDocumentPlainText(t *testing.T) {
	doc := domain.Document{Name: "notas.txt", ContentType: "text/plain"}
	got := describeDocument(doc, []byte("conteúdo do arquivo"))
	if got != "conteúdo do arquivo" {
		t.Errorf("describeDocument texto simples = %q", got)
	}
}

func TestDescribeDocumentImageWithoutVision(t *testing.T) {
	doc := domain.Document{Name: "foto.png", ContentType: "image/png"}
	got := describeDocument(doc, []byte{0x89, 0x50, 0x4e, 0x47})
	if !strings.Contains(got, "foto.png") || !strings.Contains(got, "não consegue ver imagens") {
		t.Errorf("describeDocument imagem = %q, want a note naming the file and the limitation", got)
	}
}

func TestDescribeDocumentTruncatesLongText(t *testing.T) {
	doc := domain.Document{Name: "longo.txt", ContentType: "text/plain"}
	long := strings.Repeat("a", maxAttachmentChars*2)
	got := describeDocument(doc, []byte(long))
	if len(got) > maxAttachmentChars+20 { // truncate() may append a short marker
		t.Errorf("describeDocument não truncou: %d bytes", len(got))
	}
}

func TestRenderAndExtractSpreadsheetRoundTrip(t *testing.T) {
	rows := [][]string{
		{"Categoria", "Valor"},
		{"Mercado", "150,00"},
		{"Luz", "180,00"},
	}
	data, err := renderXLSX(rows)
	if err != nil {
		t.Fatalf("renderXLSX: %v", err)
	}
	text, err := extractSpreadsheetText(data)
	if err != nil {
		t.Fatalf("extractSpreadsheetText: %v", err)
	}
	for _, want := range []string{"Categoria", "Mercado", "180,00"} {
		if !strings.Contains(text, want) {
			t.Errorf("planilha extraída não contém %q:\n%s", want, text)
		}
	}
}

func TestRenderPDFProducesReadableBytes(t *testing.T) {
	data, err := renderPDF("Relatório de gastos\nMercado: R$ 150,00\nAcentuação: ção, ã, é")
	if err != nil {
		t.Fatalf("renderPDF: %v", err)
	}
	if len(data) == 0 || string(data[:4]) != "%PDF" {
		t.Fatalf("renderPDF não produziu um PDF válido (header = %q)", string(data[:min(4, len(data))]))
	}
	text, err := extractPDFText(data)
	if err != nil {
		t.Fatalf("extractPDFText: %v", err)
	}
	if !strings.Contains(text, "Mercado") {
		t.Errorf("texto extraído do PDF não contém 'Mercado': %q", text)
	}
}
