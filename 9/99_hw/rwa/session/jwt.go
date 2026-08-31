package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"rwa/model"
	"rwa/storage"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

type Manager struct {
	storage *storage.Storage
	secret  []byte
	ttl     time.Duration
}

func NewSessionManager(storage *storage.Storage, secret []byte, ttl time.Duration) *Manager {
	return &Manager{
		storage: storage,
		secret:  secret,
		ttl:     ttl,
	}
}

func (s *Manager) Create(userID string) (string, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(s.ttl)

	session := &model.Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	claims := Claims{
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

	s.storage.Mu.Lock()
	s.storage.Sessions[sessionID] = session
	s.storage.Mu.Unlock()

	return tokenString, nil
}

func (s *Manager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
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

	claims, ok := token.Claims.(*Claims)
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
