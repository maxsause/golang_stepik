package model

import "time"

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
