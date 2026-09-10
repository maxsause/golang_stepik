package handler

import (
	"encoding/json"
	"net/http"
	"rwa/internal/model"
	"rwa/internal/utils"
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
	h.Users.Begin()
	user, ok := h.Users.Get(req.User.Email)
	h.Users.End()
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

	h.Sessions.Begin()
	h.Sessions.Delete(session.ID)
	h.Sessions.End()

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) UserRegister(w http.ResponseWriter, r *http.Request) {
	var req UserRegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.User.Email == "" || req.User.Username == "" || req.User.Password == "" {
		http.Error(
			w,
			"Email, username and password are required",
			http.StatusUnprocessableEntity,
		)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(req.User.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	now := time.Now()

	user := model.User{
		ID:           utils.RandStringRunes(16),
		Email:        req.User.Email,
		Username:     req.User.Username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	h.Users.Begin()

	if _, exists := h.Users.Get(req.User.Email); exists {
		h.Users.End()

		http.Error(
			w,
			"User already exists",
			http.StatusUnprocessableEntity,
		)
		return
	}

	users := h.Users.List()

	for _, existingUser := range users {
		if existingUser.Username == req.User.Username {
			h.Users.End()

			http.Error(
				w,
				"User already exists",
				http.StatusUnprocessableEntity,
			)
			return
		}
	}

	h.Users.Create(user.Email, user)

	h.Users.End()

	token, err := h.createSession(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := UsersResponse{
		User: UserResponse{
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Username:  user.Username,
			Token:     token,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}

func (h *Handler) UserGet(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey).(*model.Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.Users.Begin()

	var user model.User
	found := false

	for _, u := range h.Users.List() {
		if u.ID == session.UserID {
			user = u
			found = true
			break
		}
	}

	h.Users.End()

	if !found {
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

	h.Users.Begin()

	users := h.Users.List()

	var user model.User
	found := false

	for _, u := range users {
		if u.ID == session.UserID {
			user = u
			found = true
			break
		}
	}

	if !found {
		h.Users.End()

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

	h.Users.Delete(oldEmail)
	h.Users.Create(user.Email, user)

	h.Users.End()

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

	session := model.Session{
		ID:        utils.RandStringRunes(32),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}

	h.Sessions.Begin()
	h.Sessions.Create(session.ID, session)
	h.Sessions.End()

	token, err := h.Session.Create(session.ID)
	if err != nil {
		h.Sessions.Begin()
		h.Sessions.Delete(session.ID)
		h.Sessions.End()

		return "", err
	}

	return token, nil
}
