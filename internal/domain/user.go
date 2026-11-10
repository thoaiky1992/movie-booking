package domain

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	Email     string
	FullName  string
	CreatedAt time.Time
}

// NewUser validates and normalises registration input.
func NewUser(email, fullName string, now time.Time) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	fullName = strings.TrimSpace(fullName)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if fullName == "" || len(fullName) > 200 {
		return nil, fmt.Errorf("%w: full_name is required (max 200 chars)", ErrInvalidInput)
	}
	return &User{ID: uuid.New(), Email: email, FullName: fullName, CreatedAt: now}, nil
}
