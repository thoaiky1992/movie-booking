package domain

import (
	"errors"
	"strings"
)

var ErrInvalidEmailAddress = errors.New("invalid email address")

type EmailAddress struct{ value string }

func NewEmailAddress(value string) (EmailAddress, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return EmailAddress{}, ErrInvalidEmailAddress
	}
	return EmailAddress{value: value}, nil
}

func (email EmailAddress) String() string { return email.value }
