package main

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

type Article struct {
	Slug        string
	Title       string
	Description string
	Body        string
	TagList     []string

	AuthorID string

	CreatedAt time.Time
	UpdatedAt time.Time
}
