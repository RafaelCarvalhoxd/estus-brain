package assistant

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

// Attachment is one file the person attached to a chat message, resolved
// once in Chat.Send so every provider gets something useful — real pixels
// for one that can see, extracted text for every other, instead of the
// attachment silently vanishing for whichever engine happens to be picked.
type Attachment struct {
	Name        string
	ContentType string
	// Text is what a provider with no vision reads instead of the image —
	// extracted from a PDF/spreadsheet/plain text file, or a note saying an
	// image came in and this engine can't see it.
	Text string
	// ImageBase64 and ImageMediaType are set only for an image a vision
	// provider (Anthropic or OpenAI's API today — see visionCapableProviders)
	// can actually see.
	ImageBase64    string
	ImageMediaType string
}

const (
	maxAttachmentBytes = 15 << 20 // covers any spreadsheet or PDF this app would see
	maxAttachmentChars = 8000     // keeps one file from crowding out the rest of the context
)

// visionCapableProviders send real image bytes; everyone else — including
// Claude Code and Codex, whose CLIs run with their own file tools switched
// off (see providers_cli.go) and so can't read a path even if given one —
// gets Attachment.Text's fallback note instead.
var visionCapableProviders = map[string]bool{"anthropic": true, "openai": true}

// resolveAttachment reads the uploaded document once and turns it into
// whatever providerID can use.
func resolveAttachment(ctx context.Context, documents *service.DocumentService, documentID, providerID string) (*Attachment, error) {
	if documents == nil {
		return nil, fmt.Errorf("módulo de documentos indisponível")
	}
	doc, file, err := documents.Open(ctx, documentID)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxAttachmentBytes))
	if err != nil {
		return nil, fmt.Errorf("ler anexo: %w", err)
	}

	att := &Attachment{Name: doc.Name, ContentType: doc.ContentType}
	if strings.HasPrefix(doc.ContentType, "image/") && visionCapableProviders[providerID] {
		att.ImageBase64 = base64.StdEncoding.EncodeToString(data)
		att.ImageMediaType = doc.ContentType
		return att, nil
	}
	att.Text = describeDocument(doc, data)
	return att, nil
}

// describeDocument renders any stored document as text: what documents_read
// hands the model, and what resolveAttachment falls back to for a provider
// that can't see the actual pixels of an image.
func describeDocument(doc domain.Document, data []byte) string {
	switch {
	case strings.HasPrefix(doc.ContentType, "image/"):
		return fmt.Sprintf("[a pessoa anexou uma imagem (%s), mas este motor não consegue ver imagens]", doc.Name)
	case doc.ContentType == "application/pdf":
		text, err := extractPDFText(data)
		if err != nil {
			return fmt.Sprintf("[não consegui ler o PDF %s: %v]", doc.Name, err)
		}
		return truncate(text, maxAttachmentChars)
	case isSpreadsheet(doc.ContentType, doc.Name):
		text, err := extractSpreadsheetText(data)
		if err != nil {
			return fmt.Sprintf("[não consegui ler a planilha %s: %v]", doc.Name, err)
		}
		return truncate(text, maxAttachmentChars)
	case isLikelyText(data):
		return truncate(string(data), maxAttachmentChars)
	default:
		return fmt.Sprintf("[anexo %s (%s) — formato não lido automaticamente]", doc.Name, doc.ContentType)
	}
}

// isLikelyText is a cheap binary/text sniff: a NUL byte in the first
// kilobyte is not something any text format legitimately contains.
func isLikelyText(data []byte) bool {
	sample := data
	if len(sample) > 1024 {
		sample = sample[:1024]
	}
	for _, b := range sample {
		if b == 0 {
			return false
		}
	}
	return true
}

func isSpreadsheet(contentType, name string) bool {
	if strings.Contains(contentType, "spreadsheet") || strings.Contains(contentType, "excel") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(name), ".xlsx")
}

func extractPDFText(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	reader, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	text, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(text), nil
}

func extractSpreadsheetText(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer f.Close()
	var b strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "## %s\n", sheet)
		for _, row := range rows {
			b.WriteString(strings.Join(row, "\t"))
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}
