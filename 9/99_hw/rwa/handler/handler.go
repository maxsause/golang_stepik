package handler

import (
	"rwa/session"
	"rwa/storage"
)

type Handler struct {
	Storage *storage.Storage
	Session *session.Manager
}
