package telegram

import (
	"regexp"
	"testing"
	"time"
)

func TestNewPairingCode(t *testing.T) {
	code, err := NewPairingCode()
	if err != nil || !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
		t.Fatalf("code = %q, %v", code, err)
	}
}

func TestPairingMatches(t *testing.T) {
	now := time.Date(2026, time.September, 15, 10, 0, 0, 0, time.UTC)
	valid := now.Add(5 * time.Minute)
	expired := now.Add(-time.Second)
	cases := []struct {
		name    string
		code    string
		expires *time.Time
		text    string
		want    bool
	}{
		{"right code", "482913", &valid, "/start 482913", true},
		{"with bot name", "482913", &valid, "/start@estus_bot 482913", true},
		{"wrong code", "482913", &valid, "/start 111111", false},
		{"expired", "482913", &expired, "/start 482913", false},
		{"no code in force", "", &valid, "/start ", false},
		{"not a start command", "482913", &valid, "482913", false},
		{"extra words", "482913", &valid, "/start 482913 oi", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pairingMatches(tc.code, tc.expires, now, tc.text); got != tc.want {
				t.Fatalf("pairingMatches = %v, want %v", got, tc.want)
			}
		})
	}
}
