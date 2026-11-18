package domain

import "errors"

var ErrInvalidUserID = errors.New("invalid user ID")

type UserID int64

func NewUserID(value int64) (UserID, error) {
	if value <= 0 {
		return 0, ErrInvalidUserID
	}
	return UserID(value), nil
}
