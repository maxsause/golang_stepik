package main

import (
	"net/http"
	"time"
)

func GetApp() http.Handler {

	storage := NewStorage()
	session := NewSessionManager(storage, []byte("key"), 24*time.Hour)

	h := &handler{
		storage: storage,
		session: session,
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
