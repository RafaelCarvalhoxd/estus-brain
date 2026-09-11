package domain

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func randomKey(t *testing.T) [32]byte {
	t.Helper()
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestEncryptDecryptPasswordRoundTrip(t *testing.T) {
	cases := []string{
		"hunter2",
		"",
		"correct horse battery staple with spaces and 123!@#",
	}
	for _, plaintext := range cases {
		key := randomKey(t)
		ciphertext, nonce, err := EncryptPassword(key, plaintext)
		if err != nil {
			t.Fatalf("EncryptPassword(%q): %v", plaintext, err)
		}
		got, err := DecryptPassword(key, ciphertext, nonce)
		if err != nil {
			t.Fatalf("DecryptPassword(%q): %v", plaintext, err)
		}
		if got != plaintext {
			t.Fatalf("round trip mismatch: got %q, want %q", got, plaintext)
		}
	}
}

func TestDecryptPasswordWrongKeyFails(t *testing.T) {
	key := randomKey(t)
	wrongKey := randomKey(t)
	ciphertext, nonce, err := EncryptPassword(key, "correct-password")
	if err != nil {
		t.Fatalf("EncryptPassword: %v", err)
	}
	if _, err := DecryptPassword(wrongKey, ciphertext, nonce); err == nil {
		t.Fatal("expected decryption with wrong key to fail, got nil error")
	}
}

func TestDecryptPasswordTamperedCiphertextFails(t *testing.T) {
	key := randomKey(t)
	ciphertext, nonce, err := EncryptPassword(key, "correct-password")
	if err != nil {
		t.Fatalf("EncryptPassword: %v", err)
	}
	tampered := append([]byte(nil), ciphertext...)
	tampered[0] ^= 0xFF
	if _, err := DecryptPassword(key, tampered, nonce); err == nil {
		t.Fatal("expected decryption of tampered ciphertext to fail, got nil error")
	}
}

func TestLoadEncryptionKeyFromEnv(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		t.Setenv("VAULT_ENCRYPTION_KEY", "")
		if _, err := LoadEncryptionKeyFromEnv(); err == nil {
			t.Fatal("expected error when VAULT_ENCRYPTION_KEY is unset")
		}
	})

	t.Run("wrong length", func(t *testing.T) {
		t.Setenv("VAULT_ENCRYPTION_KEY", "c2hvcnQ=") // "short" base64
		if _, err := LoadEncryptionKeyFromEnv(); err == nil {
			t.Fatal("expected error when key does not decode to 32 bytes")
		}
	})

	t.Run("valid", func(t *testing.T) {
		key := randomKey(t)
		t.Setenv("VAULT_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key[:]))
		got, err := LoadEncryptionKeyFromEnv()
		if err != nil {
			t.Fatalf("LoadEncryptionKeyFromEnv: %v", err)
		}
		if got != key {
			t.Fatal("decoded key does not match original")
		}
	})
}
