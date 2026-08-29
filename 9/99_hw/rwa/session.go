package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Session struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionClaims struct {
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

type SessionManager struct {
	storage *Storage
	secret  []byte
	ttl     time.Duration
}

func NewSessionManager(storage *Storage, secret []byte, ttl time.Duration) *SessionManager {
	return &SessionManager{
		storage: storage,
		secret:  secret,
		ttl:     ttl,
	}
}

func (s *SessionManager) Create(userID string) (string, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(s.ttl)

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	claims := SessionClaims{
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	s.storage.mu.Lock()
	s.storage.sessions[sessionID] = session
	s.storage.mu.Unlock()

	return tokenString, nil
}

func (s *SessionManager) ParseToken(tokenString string) (*SessionClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&SessionClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf(
					"unexpected signing method: %v",
					token.Header["alg"],
				)
			}

			return s.secret, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*SessionClaims)
	if !ok || claims.SessionID == "" {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}

func generateSessionID() (string, error) {
	buf := make([]byte, 16)

	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}
