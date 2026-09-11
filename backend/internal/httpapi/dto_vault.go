package httpapi

import (
	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

// vaultEntryDTO never carries a password field, in either direction: list
// and get responses can't leak plaintext because there's nowhere in the
// struct to put it.
type vaultEntryDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	URL       string `json:"url"`
	Notes     string `json:"notes,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

func toVaultEntryDTO(e domain.VaultEntry) vaultEntryDTO {
	return vaultEntryDTO{
		ID:        e.ID,
		Title:     e.Title,
		Username:  e.Username,
		URL:       e.URL,
		Notes:     e.Notes,
		UpdatedAt: e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

type vaultEntryRequest struct {
	Title    string `json:"title"`
	Username string `json:"username"`
	Password string `json:"password"`
	URL      string `json:"url"`
	Notes    string `json:"notes"`
}

func (r vaultEntryRequest) toInput() service.VaultEntryInput {
	return service.VaultEntryInput{
		Title:    r.Title,
		Username: r.Username,
		Password: r.Password,
		URL:      r.URL,
		Notes:    r.Notes,
	}
}

// revealResponseDTO is the one and only place a plaintext password crosses
// the API boundary — returned by RevealFinish, and only after a successful
// WebAuthn assertion.
type revealResponseDTO struct {
	Password string `json:"password"`
}

type vaultWebAuthnStatusDTO struct {
	Registered bool `json:"registered"`
}

// vaultCeremonyBeginResponse bundles the WebAuthn options a browser needs
// for navigator.credentials.create()/.get() with the ephemeral session
// token it must echo back (as a query parameter) to the matching finish
// endpoint. The corresponding finish endpoints take the raw
// PublicKeyCredential JSON as their request body — the go-webauthn library
// parses that body itself — so there is no finish request DTO.
type vaultCeremonyBeginResponse struct {
	Options       any    `json:"options"`
	RevealSession string `json:"reveal_session"`
}
