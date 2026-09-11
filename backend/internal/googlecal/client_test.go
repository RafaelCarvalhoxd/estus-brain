package googlecal

import (
	"os"
	"testing"
)

func TestOAuthConfigFromEnv_MissingVars(t *testing.T) {
	for _, key := range []string{"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL"} {
		old, had := os.LookupEnv(key)
		os.Unsetenv(key)
		if had {
			defer os.Setenv(key, old)
		}
	}

	_, err := OAuthConfigFromEnv()
	if err == nil {
		t.Fatal("expected an error when Google OAuth env vars are unset")
	}
}

func TestOAuthConfigFromEnv_AllSet(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/google/oauth/callback")

	cfg, err := OAuthConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClientID != "test-client-id" {
		t.Errorf("ClientID = %q", cfg.ClientID)
	}

	url := AuthURL(cfg, "state123")
	if url == "" {
		t.Error("expected a non-empty auth URL")
	}
}
