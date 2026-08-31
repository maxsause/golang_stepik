package handler

import (
	"encoding/json"
	"net/http"
	"rwa/model"
	"rwa/utils"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserLoginRequest struct {
	User struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	} `json:"user"`
}

type UserResponse struct {
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Username  string    `json:"username"`
	Bio       string    `json:"bio"`
	Image     string    `json:"image"`
	Token     string    `json:"token"`
}

type UsersResponse struct {
	User UserResponse `json:"user"`
}

type UserRegisterRequest struct {
	User struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	} `json:"user"`
}

type UserUpdateRequest struct {
	User struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
		Bio      string `json:"bio"`
		Image    string `json:"image"`
	} `json:"user"`
}

func (h *Handler) UserLogin(w http.ResponseWriter, r *http.Request) {
	var req UserLoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.Storage.Mu.Lock()
	user, ok := h.Storage.Users[req.User.Email]
	h.Storage.Mu.Unlock()
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.User.Password))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token, err := h.createSession(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := UsersResponse{
		User: UserResponse{
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Username:  user.Username,
			Token:     token,
		},
	}
	if err = json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) UserLogout(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey).(*model.Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.Storage.Mu.Lock()
	delete(h.Storage.Sessions, session.ID)
	h.Storage.Mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) UserRegister(w http.ResponseWriter, r *http.Request) {
	var req UserRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.User.Email == "" || req.User.Username == "" || req.User.Password == "" {
		http.Error(w, "Email, username and password are required", http.StatusUnprocessableEntity)
		return
	}

	h.Storage.Mu.Lock()
	if _, exists := h.Storage.Users[req.User.Email]; exists {
		h.Storage.Mu.Unlock()

		http.Error(w, "User already exists", http.StatusUnprocessableEntity)
		return
	}

	for _, user := range h.Storage.Users {
		if user.Username == req.User.Username {
			h.Storage.Mu.Unlock()

			http.Error(w, "User already exists", http.StatusUnprocessableEntity)
			return
		}
	}
	h.Storage.Mu.Unlock()

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(req.User.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	now := time.Now()

	user := &model.User{
		ID:           utils.RandStringRunes(16),
		Email:        req.User.Email,
		Username:     req.User.Username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	h.Storage.Mu.Lock()
	h.Storage.Users[user.Email] = user
	h.Storage.Mu.Unlock()

	token, err := h.createSession(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := UsersResponse{
		User: UserResponse{
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Username:  user.Username,
			Token:     token,
		},
	}
	if err = json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) UserGet(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey).(*model.Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.Storage.Mu.Lock()
	var user *model.User
	for _, u := range h.Storage.Users {
		if u.ID == session.UserID {
			user = u
			break
		}
	}
	h.Storage.Mu.Unlock()

	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := UsersResponse{
		User: UserResponse{
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Username:  user.Username,
			Bio:       user.Bio,
			Image:     user.Image,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) UserUpdate(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey).(*model.Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req UserUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var passwordHash []byte
	if req.User.Password != "" {
		hash, err := bcrypt.GenerateFromPassword(
			[]byte(req.User.Password),
			bcrypt.DefaultCost,
		)
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}

		passwordHash = hash
	}

	h.Storage.Mu.Lock()
	var user *model.User

	for _, u := range h.Storage.Users {
		if u.ID == session.UserID {
			user = u
			break
		}
	}

	if user == nil {
		h.Storage.Mu.Unlock()
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	oldEmail := user.Email

	if req.User.Email != "" {
		user.Email = req.User.Email
	}

	if req.User.Username != "" {
		user.Username = req.User.Username
	}

	if req.User.Bio != "" {
		user.Bio = req.User.Bio
	}

	if req.User.Image != "" {
		user.Image = req.User.Image
	}

	if req.User.Password != "" {
		user.PasswordHash = passwordHash
	}

	user.UpdatedAt = time.Now()

	if oldEmail != user.Email {
		delete(h.Storage.Users, oldEmail)
		h.Storage.Users[user.Email] = user
	}

	h.Storage.Mu.Unlock()

	token, err := h.createSession(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := UsersResponse{
		User: UserResponse{
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Username:  user.Username,
			Bio:       user.Bio,
			Image:     user.Image,
			Token:     token,
		},
	}
	if err = json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) createSession(userID string) (string, error) {
	now := time.Now()

	session := &model.Session{
		ID:        utils.RandStringRunes(32),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}

	h.Storage.Mu.Lock()
	h.Storage.Sessions[session.ID] = session
	h.Storage.Mu.Unlock()

	token, err := h.Session.Create(session.ID)
	if err != nil {
		h.Storage.Mu.Lock()
		delete(h.Storage.Sessions, session.ID)
		h.Storage.Mu.Unlock()

		return "", err
	}

	return token, nil
}
