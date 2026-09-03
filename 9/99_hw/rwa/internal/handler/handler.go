package handler

import (
	"rwa/internal/storage"
)

type SessionManager interface {
	Create(sessionID string) (string, error)
	Parse(token string) (string, error)
}
type Handler struct {
	Storage *storage.Storage
	Session SessionManager
}
