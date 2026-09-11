package domain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"time"
)

// VaultEntry is one password manager record. The password is never held in
// plaintext outside of a Reveal call — at rest and in every other code path
// it exists only as PasswordCiphertext/PasswordNonce. Notes is stored as
// plaintext: it's a lower-value target than the password itself, and
// encrypting it too would double the surface for no real gain here.
type VaultEntry struct {
	ID                 string
	Title              string
	Username           string
	URL                string
	Notes              string
	PasswordCiphertext []byte
	PasswordNonce      []byte
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (e VaultEntry) Validate() error {
	if e.Title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	return nil
}

// EncryptPassword seals plaintext with AES-256-GCM under key, returning the
// ciphertext and the freshly generated nonce that must be stored alongside
// it — GCM needs the exact same nonce back to decrypt.
func EncryptPassword(key [32]byte, plaintext string) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, nil, fmt.Errorf("new aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

// DecryptPassword reverses EncryptPassword. It fails if key, ciphertext or
// nonce don't match exactly what EncryptPassword produced — GCM's
// authentication tag makes tampering detectable rather than silently
// returning garbage.
func DecryptPassword(key [32]byte, ciphertext, nonce []byte) (string, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("new aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt password: %w", err)
	}
	return string(plaintext), nil
}

// LoadEncryptionKeyFromEnv reads and base64-decodes VAULT_ENCRYPTION_KEY. It
// lives in domain (not internal/config) because it's specific to this
// module — the rest of the app has no notion of an encryption key.
func LoadEncryptionKeyFromEnv() ([32]byte, error) {
	var key [32]byte
	raw := os.Getenv("VAULT_ENCRYPTION_KEY")
	if raw == "" {
		return key, fmt.Errorf("VAULT_ENCRYPTION_KEY is required")
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return key, fmt.Errorf("VAULT_ENCRYPTION_KEY is not valid base64: %w", err)
	}
	if len(decoded) != 32 {
		return key, fmt.Errorf("VAULT_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(decoded))
	}
	copy(key[:], decoded)
	return key, nil
}
