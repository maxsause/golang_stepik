package handler

import "rwa/internal/model"

type Storage[T any] interface {
	Begin()
	End()
	Get(key string) (T, bool)
	List() []T
	Create(key string, value T)
	Delete(key string)
}

type SessionManager interface {
	Create(sessionID string) (string, error)
	Parse(token string) (string, error)
}

type Handler struct {
	Users    Storage[model.User]
	Sessions Storage[model.Session]
	Articles Storage[model.Article]
	Session  SessionManager
}
