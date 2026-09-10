package main

import (
	"net/http"
	"rwa/internal/handler"
	"rwa/internal/model"
	"rwa/internal/session"
	"rwa/internal/storage"
	"time"
)

func GetApp() http.Handler {

	userStorage := storage.NewMemoryStorage[model.User]()
	sessionStorage := storage.NewMemoryStorage[model.Session]()
	articleStorage := storage.NewMemoryStorage[model.Article]()

	sessionManager := session.NewSessionManager(
		[]byte("key"),
		24*time.Hour,
	)

	h := &handler.Handler{
		Users:    userStorage,
		Sessions: sessionStorage,
		Articles: articleStorage,
		Session:  sessionManager,
	}

	mux := http.NewServeMux()
	mux.Handle(
		"GET /api/user",
		h.AuthMiddleware(http.HandlerFunc(h.UserGet)),
	)
	mux.Handle(
		"PUT /api/user",
		h.AuthMiddleware(http.HandlerFunc(h.UserUpdate)),
	)
	mux.Handle(
		"POST /api/user/logout",
		h.AuthMiddleware(http.HandlerFunc(h.UserLogout)),
	)
	mux.Handle(
		"POST /api/articles",
		h.AuthMiddleware(http.HandlerFunc(h.ArticlesCreate)),
	)

	mux.HandleFunc("POST /api/users", h.UserRegister)
	mux.HandleFunc("POST /api/users/login", h.UserLogin)
	mux.HandleFunc("GET /api/articles", h.ArticlesGetRecent)

	return mux
}
