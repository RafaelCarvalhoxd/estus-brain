package postgres

import (
	"context"
	"crypto/rand"
	"os"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestVaultRepoCreateListGetRoundTrip(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	db, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	repo := NewVaultRepo(db)

	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	const plaintext = "s3cr3t-p@ssw0rd"
	ciphertext, nonce, err := domain.EncryptPassword(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptPassword: %v", err)
	}

	created, err := repo.Create(ctx, domain.VaultEntry{
		Title:              "Teste Estus Vault",
		Username:           "usuario@example.com",
		URL:                "https://example.com",
		Notes:              "nota de teste",
		PasswordCiphertext: ciphertext,
		PasswordNonce:      nonce,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() {
		if err := repo.Delete(ctx, created.ID); err != nil {
			t.Logf("cleanup delete: %v", err)
		}
	}()

	if created.ID == "" {
		t.Fatal("expected a generated ID")
	}

	entries, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.ID != created.ID {
			continue
		}
		found = true
		if e.PasswordCiphertext != nil || e.PasswordNonce != nil {
			t.Fatal("List leaked ciphertext/nonce bytes, expected metadata only")
		}
		if e.Title != "Teste Estus Vault" {
			t.Fatalf("unexpected title in list: %q", e.Title)
		}
	}
	if !found {
		t.Fatal("created entry not present in List result")
	}

	fetched, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, err := domain.DecryptPassword(key, fetched.PasswordCiphertext, fetched.PasswordNonce)
	if err != nil {
		t.Fatalf("DecryptPassword: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round trip mismatch: got %q, want %q", got, plaintext)
	}
}
