package telegram

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	pairingTTL = 10 * time.Minute
	// After this many wrong tries the code is thrown away, so it can't be
	// guessed by flooding the bot.
	maxPairingAttempts = 5
)

// NewPairingCode is a random six-digit code.
func NewPairingCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("pairing code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// pairingMatches reports whether text is "/start <code>" with the code in force.
func pairingMatches(code string, expires *time.Time, now time.Time, text string) bool {
	fields := strings.Fields(text)
	if code == "" || expires == nil || !now.Before(*expires) || len(fields) != 2 {
		return false
	}
	if cmd := fields[0]; cmd != "/start" && !strings.HasPrefix(cmd, "/start@") {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(fields[1]), []byte(code)) == 1
}
