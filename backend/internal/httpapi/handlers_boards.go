package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type boardSummaryDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Preview   string `json:"preview"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type boardDTO struct {
	boardSummaryDTO
	Scene json.RawMessage `json:"scene"`
}

func toBoardSummaryDTO(b domain.Board) boardSummaryDTO {
	return boardSummaryDTO{
		ID:        b.ID,
		Name:      b.Name,
		Preview:   b.Preview,
		CreatedAt: b.CreatedAt.Format(time.RFC3339),
		UpdatedAt: b.UpdatedAt.Format(time.RFC3339),
	}
}

type BoardHandlers struct {
	boards *service.BoardService
}

func NewBoardHandlers(boards *service.BoardService) *BoardHandlers {
	return &BoardHandlers{boards: boards}
}

func (h *BoardHandlers) List(w http.ResponseWriter, r *http.Request) {
	boards, err := h.boards.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]boardSummaryDTO, len(boards))
	for i, b := range boards {
		out[i] = toBoardSummaryDTO(b)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *BoardHandlers) Get(w http.ResponseWriter, r *http.Request) {
	b, err := h.boards.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, boardDTO{boardSummaryDTO: toBoardSummaryDTO(b), Scene: b.Scene})
}

type boardNameRequest struct {
	Name string `json:"name"`
}

func (h *BoardHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req boardNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	b, err := h.boards.Create(r.Context(), req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toBoardSummaryDTO(b))
}

func (h *BoardHandlers) Rename(w http.ResponseWriter, r *http.Request) {
	var req boardNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	if err := h.boards.Rename(r.Context(), r.PathValue("id"), req.Name); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

type saveSceneRequest struct {
	Scene   json.RawMessage `json:"scene"`
	Preview string          `json:"preview"`
}

// SaveScene is called by the editor's autosave. The body cap sits above the
// scene limit so an oversized board gets a clear validation error, not a
// dropped connection.
func (h *BoardHandlers) SaveScene(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxBoardSceneBytes+domain.MaxBoardPreviewBytes+(1<<20))
	var req saveSceneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, fmt.Errorf("%w: board is larger than %d MB", domain.ErrValidation, domain.MaxBoardSceneBytes>>20))
			return
		}
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	if err := h.boards.SaveScene(r.Context(), r.PathValue("id"), req.Scene, req.Preview); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *BoardHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.boards.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
