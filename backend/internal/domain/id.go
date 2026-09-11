package domain

import "github.com/google/uuid"

func newID() string {
	return uuid.NewString()
}

// NewID is exported for callers outside the domain package (services) that
// need to mint an ID before constructing a domain type, e.g. a Category.
func NewID() string {
	return newID()
}
