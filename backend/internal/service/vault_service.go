package service

import (
	"context"
	"fmt"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// VaultService owns the encrypt/decrypt boundary for the password manager:
// callers pass and receive plaintext, everything below this layer only
// ever sees ciphertext. Reveal assumes the caller (the HTTP handler) has
// already gated the call behind a successful WebAuthn assertion — this
// service has no notion of WebAuthn at all.
type VaultService struct {
	entries *postgres.VaultRepo
	key     [32]byte
}

func NewVaultService(entries *postgres.VaultRepo) (*VaultService, error) {
	key, err := domain.LoadEncryptionKeyFromEnv()
	if err != nil {
		return nil, err
	}
	return &VaultService{entries: entries, key: key}, nil
}

type VaultEntryInput struct {
	Title    string
	Username string
	Password string
	URL      string
	Notes    string
}

func (in VaultEntryInput) toEntry() domain.VaultEntry {
	return domain.VaultEntry{
		Title:    in.Title,
		Username: in.Username,
		URL:      in.URL,
		Notes:    in.Notes,
	}
}

func (s *VaultService) Create(ctx context.Context, in VaultEntryInput) (domain.VaultEntry, error) {
	e := in.toEntry()
	if err := e.Validate(); err != nil {
		return domain.VaultEntry{}, err
	}
	ciphertext, nonce, err := domain.EncryptPassword(s.key, in.Password)
	if err != nil {
		return domain.VaultEntry{}, fmt.Errorf("encrypt password: %w", err)
	}
	e.PasswordCiphertext = ciphertext
	e.PasswordNonce = nonce
	return s.entries.Create(ctx, e)
}

func (s *VaultService) List(ctx context.Context) ([]domain.VaultEntry, error) {
	return s.entries.List(ctx)
}

// Update leaves the stored password untouched when in.Password is empty —
// editing a title or username shouldn't force the caller to re-type (or
// re-reveal, via another Touch ID prompt) the password just to keep it.
func (s *VaultService) Update(ctx context.Context, id string, in VaultEntryInput) (domain.VaultEntry, error) {
	e := in.toEntry()
	e.ID = id
	if err := e.Validate(); err != nil {
		return domain.VaultEntry{}, err
	}

	if in.Password == "" {
		existing, err := s.entries.Get(ctx, id)
		if err != nil {
			return domain.VaultEntry{}, err
		}
		e.PasswordCiphertext = existing.PasswordCiphertext
		e.PasswordNonce = existing.PasswordNonce
	} else {
		ciphertext, nonce, err := domain.EncryptPassword(s.key, in.Password)
		if err != nil {
			return domain.VaultEntry{}, fmt.Errorf("encrypt password: %w", err)
		}
		e.PasswordCiphertext = ciphertext
		e.PasswordNonce = nonce
	}

	return s.entries.Update(ctx, e)
}

func (s *VaultService) Delete(ctx context.Context, id string) error {
	return s.entries.Delete(ctx, id)
}

// Reveal decrypts and returns the plaintext password for one entry. The
// WebAuthn check is the HTTP handler's responsibility — by the time this is
// called, the ceremony has already succeeded.
func (s *VaultService) Reveal(ctx context.Context, id string) (string, error) {
	e, err := s.entries.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return domain.DecryptPassword(s.key, e.PasswordCiphertext, e.PasswordNonce)
}
