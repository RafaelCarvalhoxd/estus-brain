package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The password check runs before the service is touched, so a nil service
// is enough to prove a wrong or missing password never reaches decryption.
func TestRevealRejectsWrongPassword(t *testing.T) {
	cases := map[string]struct {
		configured string
		body       string
	}{
		"wrong password":     {configured: "segredo", body: `{"password":"outra"}`},
		"missing password":   {configured: "segredo", body: `{}`},
		"no password in env": {configured: "", body: `{"password":""}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewVaultHandlers(nil, tc.configured)
			req := httptest.NewRequest(http.MethodPost, "/api/vault/abc/reveal", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			h.Reveal(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}
