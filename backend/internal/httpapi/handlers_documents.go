package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type DocumentHandlers struct {
	docs *service.DocumentService
}

func NewDocumentHandlers(docs *service.DocumentService) *DocumentHandlers {
	return &DocumentHandlers{docs: docs}
}

// An empty folder id means the root of the tree, so "" and "missing" have to
// mean the same thing everywhere a folder is referenced.
func optionalFolderID(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return &raw
}

func (h *DocumentHandlers) ListFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := h.docs.ListFolders(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]documentFolderDTO, len(folders))
	for i, f := range folders {
		out[i] = toDocumentFolderDTO(f)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *DocumentHandlers) CreateFolder(w http.ResponseWriter, r *http.Request) {
	var req documentFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	f, err := h.docs.CreateFolder(r.Context(), req.ParentID, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDocumentFolderDTO(f))
}

func (h *DocumentHandlers) RenameFolder(w http.ResponseWriter, r *http.Request) {
	var req documentFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	f, err := h.docs.RenameFolder(r.Context(), r.PathValue("id"), req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocumentFolderDTO(f))
}

func (h *DocumentHandlers) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	if err := h.docs.DeleteFolder(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *DocumentHandlers) List(w http.ResponseWriter, r *http.Request) {
	docs, err := h.docs.List(r.Context(), optionalFolderID(r.URL.Query().Get("folder_id")))
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]documentDTO, len(docs))
	for i, d := range docs {
		out[i] = toDocumentDTO(d)
	}
	writeJSON(w, http.StatusOK, out)
}

// Count feeds the one-line status the home screen shows for this module,
// which shouldn't have to download the whole listing to say "12 arquivos".
func (h *DocumentHandlers) Count(w http.ResponseWriter, r *http.Request) {
	n, err := h.docs.Count(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": n})
}

func (h *DocumentHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	// A hard ceiling on the request body, above the per-file limit so the
	// service can still report "too large" rather than the connection
	// dying mid-upload.
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxDocumentBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, fmt.Errorf("%w: invalid upload", domain.ErrValidation))
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, fmt.Errorf("%w: no file in the upload", domain.ErrValidation))
		return
	}
	defer file.Close()

	name := header.Filename
	if given := strings.TrimSpace(r.FormValue("name")); given != "" {
		name = given
	}

	doc, err := h.docs.Save(r.Context(), optionalFolderID(r.FormValue("folder_id")), name, header.Header.Get("Content-Type"), file)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDocumentDTO(doc))
}

// Content streams the file itself. This is the one endpoint that answers
// with something other than JSON.
func (h *DocumentHandlers) Content(w http.ResponseWriter, r *http.Request) {
	doc, file, err := h.docs.Open(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", doc.SizeBytes))
	// Quotes and backslashes are the only characters that can break out of
	// the quoted-string form of this header.
	escaped := strings.NewReplacer(`"`, "", `\`, "").Replace(doc.Name)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", escaped))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, file); err != nil {
		slog.Error("stream document", "id", doc.ID, "error", err)
	}
}

func (h *DocumentHandlers) Move(w http.ResponseWriter, r *http.Request) {
	var req moveDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	doc, err := h.docs.Move(r.Context(), r.PathValue("id"), req.FolderID, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocumentDTO(doc))
}

func (h *DocumentHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.docs.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
