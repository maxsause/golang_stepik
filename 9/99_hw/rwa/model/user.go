package model

import "time"

type User struct {
	ID           string
	Email        string
	Username     string
	PasswordHash []byte

	Bio   string
	Image string

	CreatedAt time.Time
	UpdatedAt time.Time
}
