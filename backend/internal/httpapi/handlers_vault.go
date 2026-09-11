package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

// VaultHandlers serves the password manager module: plain CRUD for
// vault entries plus the WebAuthn ceremonies that gate revealing one. It's
// intentionally a separate handler struct from Handlers so this module's
// routes can be reviewed and evolved without touching the ledger's.
type VaultHandlers struct {
	vault    *service.VaultService
	webauthn *service.WebAuthnService
}

func NewVaultHandlers(vault *service.VaultService, webauthn *service.WebAuthnService) *VaultHandlers {
	return &VaultHandlers{vault: vault, webauthn: webauthn}
}

func (h *VaultHandlers) List(w http.ResponseWriter, r *http.Request) {
	entries, err := h.vault.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]vaultEntryDTO, len(entries))
	for i, e := range entries {
		out[i] = toVaultEntryDTO(e)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *VaultHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req vaultEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	entry, err := h.vault.Create(r.Context(), req.toInput())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toVaultEntryDTO(entry))
}

// Update handles PUT /api/vault/{id}.
func (h *VaultHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req vaultEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	entry, err := h.vault.Update(r.Context(), id, req.toInput())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toVaultEntryDTO(entry))
}

// Delete handles DELETE /api/vault/{id}.
func (h *VaultHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.vault.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// WebAuthnStatus handles GET /api/vault/webauthn-status.
func (h *VaultHandlers) WebAuthnStatus(w http.ResponseWriter, r *http.Request) {
	registered, err := h.webauthn.HasCredential(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vaultWebAuthnStatusDTO{Registered: registered})
}

// RegisterBegin handles POST /api/vault/webauthn/register/begin — the
// one-time "Configurar Touch ID" ceremony.
func (h *VaultHandlers) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	options, session, err := h.webauthn.BeginRegistration(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vaultCeremonyBeginResponse{Options: options, RevealSession: session})
}

// RegisterFinish handles POST /api/vault/webauthn/register/finish?session=...
// The request body is the raw navigator.credentials.create() response;
// go-webauthn parses it directly off r.
func (h *VaultHandlers) RegisterFinish(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	if session == "" {
		writeError(w, fmt.Errorf("%w: session query parameter is required", domain.ErrValidation))
		return
	}
	if err := h.webauthn.FinishRegistration(r.Context(), session, r); err != nil {
		writeError(w, fmt.Errorf("%w: %v", domain.ErrValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// RevealBegin handles POST /api/vault/{id}/reveal/begin.
func (h *VaultHandlers) RevealBegin(w http.ResponseWriter, r *http.Request) {
	options, session, err := h.webauthn.BeginAssertion(r.Context())
	if err != nil {
		writeError(w, fmt.Errorf("%w: %v", domain.ErrValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, vaultCeremonyBeginResponse{Options: options, RevealSession: session})
}

// RevealFinish handles POST /api/vault/{id}/reveal/finish?session=...  The
// request body is the raw navigator.credentials.get() response. Only on a
// verified assertion does this decrypt and return the plaintext password.
func (h *VaultHandlers) RevealFinish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session := r.URL.Query().Get("session")
	if session == "" {
		writeError(w, fmt.Errorf("%w: session query parameter is required", domain.ErrValidation))
		return
	}
	if err := h.webauthn.FinishAssertion(r.Context(), session, r); err != nil {
		writeError(w, fmt.Errorf("%w: %v", domain.ErrValidation, err))
		return
	}
	password, err := h.vault.Reveal(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revealResponseDTO{Password: password})
}
