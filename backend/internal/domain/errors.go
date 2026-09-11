package domain

import "errors"

// Sentinel errors returned by services and repositories. HTTP handlers map
// these to status codes; nothing above the domain layer should invent new
// error semantics for the same failure.
var (
	ErrNotFound          = errors.New("resource not found")
	ErrValidation        = errors.New("validation failed")
	ErrConflict          = errors.New("resource already exists")
	ErrCreditCardMissing = errors.New("credit_card_id is required for credit payment method")
)
