package handler

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const sessionContextKey contextKey = "session"

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
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

		sessionID, err := h.Session.Parse(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		h.Storage.Mu.Lock()
		session, ok := h.Storage.Sessions[sessionID]
		h.Storage.Mu.Unlock()

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
