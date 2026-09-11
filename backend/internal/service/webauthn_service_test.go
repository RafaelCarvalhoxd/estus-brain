package service

import (
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

// testUser is a minimal webauthn.User with no registered credentials — the
// state of the world during the initial "Configurar Touch ID" ceremony.
type testUser struct{}

func (testUser) WebAuthnID() []byte                         { return vaultOwnerID[:] }
func (testUser) WebAuthnName() string                       { return "vault" }
func (testUser) WebAuthnDisplayName() string                { return "Você" }
func (testUser) WebAuthnCredentials() []webauthn.Credential { return nil }

// TestBeginRegistrationProducesChallenge exercises the same webauthn.New
// configuration NewWebAuthnService builds, without needing a real Postgres
// connection: BeginRegistration only reads the (in-memory) user, so a stub
// with no credentials is enough to verify the library hands back valid,
// non-empty ceremony options.
func TestBeginRegistrationProducesChallenge(t *testing.T) {
	w, err := webauthn.New(&webauthn.Config{
		RPID:          "localhost",
		RPDisplayName: "Estus Vault",
		RPOrigins:     []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("webauthn.New: %v", err)
	}

	creation, session, err := w.BeginRegistration(testUser{})
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	if creation == nil {
		t.Fatal("expected non-nil CredentialCreation")
	}
	if creation.Response.Challenge.String() == "" {
		t.Fatal("expected a non-empty challenge")
	}
	if creation.Response.RelyingParty.ID != "localhost" {
		t.Fatalf("unexpected RP ID: %q", creation.Response.RelyingParty.ID)
	}
	if session == nil || session.Challenge == "" {
		t.Fatal("expected non-empty session challenge")
	}
}
