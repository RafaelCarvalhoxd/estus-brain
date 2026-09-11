package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// vaultOwnerID is a fixed WebAuthn user handle: Estus Vault has exactly one
// user, so there's no users table and no lookup by username — every
// ceremony operates on this one constant identity.
var vaultOwnerID = uuid.MustParse("6f8f8c9e-2b3a-4a3b-9b8d-7d9b2e5b8a01")

const sessionTTL = 2 * time.Minute

// WebAuthnService wraps the go-webauthn library for the single-user Touch
// ID / Face ID gate. Ephemeral challenge SessionData lives only in memory,
// keyed by a random session token — a browser round trip to the
// authenticator takes seconds, not the lifetime of the process, so there's
// no need to survive a restart.
type WebAuthnService struct {
	webAuthn    *webauthn.WebAuthn
	credentials *postgres.WebAuthnCredentialRepo
	sessions    sync.Map // string -> sessionEntry
}

type sessionEntry struct {
	data      webauthn.SessionData
	expiresAt time.Time
}

func NewWebAuthnService(credentials *postgres.WebAuthnCredentialRepo) (*WebAuthnService, error) {
	rpID := getEnvDefault("WEBAUTHN_RP_ID", "localhost")
	rpOrigin := getEnvDefault("WEBAUTHN_RP_ORIGIN", "http://localhost:3000")
	rpDisplayName := getEnvDefault("WEBAUTHN_RP_DISPLAY_NAME", "Estus Vault")

	w, err := webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpDisplayName,
		RPOrigins:     []string{rpOrigin},
	})
	if err != nil {
		return nil, fmt.Errorf("configure webauthn: %w", err)
	}
	return &WebAuthnService{webAuthn: w, credentials: credentials}, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// vaultUser adapts the stored credentials to the library's webauthn.User
// interface for the single fixed identity.
type vaultUser struct {
	credentials []webauthn.Credential
}

func (u vaultUser) WebAuthnID() []byte                         { return vaultOwnerID[:] }
func (u vaultUser) WebAuthnName() string                       { return "vault" }
func (u vaultUser) WebAuthnDisplayName() string                { return "Você" }
func (u vaultUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func toWebAuthnCredential(r postgres.WebAuthnCredentialRecord) webauthn.Credential {
	transports := make([]protocol.AuthenticatorTransport, len(r.Transports))
	for i, t := range r.Transports {
		transports[i] = protocol.AuthenticatorTransport(t)
	}
	return webauthn.Credential{
		ID:              r.CredentialID,
		PublicKey:       r.PublicKey,
		AttestationType: r.AttestationType,
		Transport:       transports,
		Authenticator:   webauthn.Authenticator{SignCount: r.SignCount},
	}
}

func (s *WebAuthnService) loadUser(ctx context.Context) (vaultUser, error) {
	records, err := s.credentials.List(ctx)
	if err != nil {
		return vaultUser{}, fmt.Errorf("load credentials: %w", err)
	}
	creds := make([]webauthn.Credential, len(records))
	for i, rec := range records {
		creds[i] = toWebAuthnCredential(rec)
	}
	return vaultUser{credentials: creds}, nil
}

// HasCredential reports whether Touch ID has already been configured.
func (s *WebAuthnService) HasCredential(ctx context.Context) (bool, error) {
	records, err := s.credentials.List(ctx)
	if err != nil {
		return false, fmt.Errorf("load credentials: %w", err)
	}
	return len(records) > 0, nil
}

func (s *WebAuthnService) putSession(session webauthn.SessionData) string {
	token := uuid.NewString()
	s.sessions.Store(token, sessionEntry{data: session, expiresAt: time.Now().Add(sessionTTL)})
	return token
}

// takeSession consumes (delete-on-use) and validates the expiry of a
// previously stored challenge.
func (s *WebAuthnService) takeSession(token string) (webauthn.SessionData, error) {
	v, ok := s.sessions.LoadAndDelete(token)
	if !ok {
		return webauthn.SessionData{}, fmt.Errorf("reveal session not found or already used")
	}
	entry := v.(sessionEntry)
	if time.Now().After(entry.expiresAt) {
		return webauthn.SessionData{}, fmt.Errorf("reveal session expired")
	}
	return entry.data, nil
}

// BeginRegistration starts the one-time "Configurar Touch ID" ceremony.
func (s *WebAuthnService) BeginRegistration(ctx context.Context) (*protocol.CredentialCreation, string, error) {
	user, err := s.loadUser(ctx)
	if err != nil {
		return nil, "", err
	}
	creation, session, err := s.webAuthn.BeginRegistration(user)
	if err != nil {
		return nil, "", fmt.Errorf("begin registration: %w", err)
	}
	return creation, s.putSession(*session), nil
}

// FinishRegistration verifies the browser's navigator.credentials.create()
// response and persists the new credential.
func (s *WebAuthnService) FinishRegistration(ctx context.Context, sessionToken string, r *http.Request) error {
	session, err := s.takeSession(sessionToken)
	if err != nil {
		return err
	}
	user, err := s.loadUser(ctx)
	if err != nil {
		return err
	}
	cred, err := s.webAuthn.FinishRegistration(user, session, r)
	if err != nil {
		return fmt.Errorf("finish registration: %w", err)
	}

	transports := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		transports[i] = string(t)
	}
	return s.credentials.Create(ctx, postgres.WebAuthnCredentialRecord{
		CredentialID:    cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		Transports:      transports,
		SignCount:       cred.Authenticator.SignCount,
	})
}

// BeginAssertion starts a reveal ceremony against the registered
// credential(s).
func (s *WebAuthnService) BeginAssertion(ctx context.Context) (*protocol.CredentialAssertion, string, error) {
	user, err := s.loadUser(ctx)
	if err != nil {
		return nil, "", err
	}
	if len(user.credentials) == 0 {
		return nil, "", fmt.Errorf("no touch id credential registered")
	}
	assertion, session, err := s.webAuthn.BeginLogin(user)
	if err != nil {
		return nil, "", fmt.Errorf("begin assertion: %w", err)
	}
	return assertion, s.putSession(*session), nil
}

// FinishAssertion verifies the browser's navigator.credentials.get()
// response. On success, the caller (the reveal HTTP handler) may proceed
// to decrypt and return the password.
func (s *WebAuthnService) FinishAssertion(ctx context.Context, sessionToken string, r *http.Request) error {
	session, err := s.takeSession(sessionToken)
	if err != nil {
		return err
	}
	user, err := s.loadUser(ctx)
	if err != nil {
		return err
	}
	cred, err := s.webAuthn.FinishLogin(user, session, r)
	if err != nil {
		return fmt.Errorf("finish assertion: %w", err)
	}
	if err := s.credentials.UpdateSignCount(ctx, cred.ID, cred.Authenticator.SignCount); err != nil {
		return fmt.Errorf("update sign count: %w", err)
	}
	return nil
}
