package main

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const sessionContextKey contextKey = "session"

func (h *handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		const prefix = "Token "
		if !strings.HasPrefix(auth, prefix) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(auth, prefix)
		if tokenString == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := h.session.ParseToken(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		h.storage.mu.Lock()
		session, ok := h.storage.sessions[claims.SessionID]
		h.storage.mu.Unlock()

		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(
			r.Context(),
			sessionContextKey,
			session,
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
