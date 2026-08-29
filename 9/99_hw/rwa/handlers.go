package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type handler struct {
	storage *Storage
	session *SessionManager
}

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

func (h *handler) UserLogin(w http.ResponseWriter, r *http.Request) {
	var req UserLoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.storage.mu.Lock()
	user, ok := h.storage.users[req.User.Email]
	h.storage.mu.Unlock()
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.User.Password))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token, err := h.session.Create(user.ID)
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

func (h *handler) UserLogout(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey).(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.storage.mu.Lock()
	delete(h.storage.sessions, session.ID)
	h.storage.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (h *handler) UserRegister(w http.ResponseWriter, r *http.Request) {
	var req UserRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.User.Email == "" || req.User.Username == "" || req.User.Password == "" {
		http.Error(w, "Email, username and password are required", http.StatusUnprocessableEntity)
		return
	}

	h.storage.mu.Lock()
	if _, exists := h.storage.users[req.User.Email]; exists {
		h.storage.mu.Unlock()

		http.Error(w, "User already exists", http.StatusUnprocessableEntity)
		return
	}

	for _, user := range h.storage.users {
		if user.Username == req.User.Username {
			h.storage.mu.Unlock()

			http.Error(w, "User already exists", http.StatusUnprocessableEntity)
			return
		}
	}
	h.storage.mu.Unlock()

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(req.User.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	now := time.Now()

	user := &User{
		ID:           RandStringRunes(16),
		Email:        req.User.Email,
		Username:     req.User.Username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	h.storage.mu.Lock()
	h.storage.users[user.Email] = user
	h.storage.mu.Unlock()

	token, err := h.session.Create(user.ID)
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

func (h *handler) UserGet(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey).(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.storage.mu.Lock()
	var user *User
	for _, u := range h.storage.users {
		if u.ID == session.UserID {
			user = u
			break
		}
	}
	h.storage.mu.Unlock()

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

func (h *handler) UserUpdate(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey).(*Session)
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

	h.storage.mu.Lock()
	var user *User

	for _, u := range h.storage.users {
		if u.ID == session.UserID {
			user = u
			break
		}
	}

	if user == nil {
		h.storage.mu.Unlock()
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
		delete(h.storage.users, oldEmail)
		h.storage.users[user.Email] = user
	}

	h.storage.mu.Unlock()

	token, err := h.session.Create(user.ID)
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

type ArticleAuthorResponse struct {
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	Image     string `json:"image"`
	Following bool   `json:"following"`
}

type ArticleResponse struct {
	Author         ArticleAuthorResponse `json:"author"`
	Body           string                `json:"body"`
	CreatedAt      time.Time             `json:"createdAt"`
	Description    string                `json:"description"`
	Favorited      bool                  `json:"favorited"`
	FavoritesCount int                   `json:"favoritesCount"`
	Slug           string                `json:"slug"`
	TagList        []string              `json:"tagList"`
	Title          string                `json:"title"`
	UpdatedAt      time.Time             `json:"updatedAt"`
}

type ArticlesResponse struct {
	Articles      []ArticleResponse `json:"articles"`
	ArticlesCount int               `json:"articlesCount"`
}

type ArticleCreateRequest struct {
	Article struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Body        string   `json:"body"`
		TagList     []string `json:"tagList"`
	} `json:"article"`
}

type SingleArticleResponse struct {
	Article ArticleResponse `json:"article"`
}

func (h *handler) ArticlesGetRecent(w http.ResponseWriter, r *http.Request) {
	authorFilter := r.URL.Query().Get("author")
	tagFilter := r.URL.Query().Get("tag")

	h.storage.mu.Lock()
	articles := make([]*Article, 0, len(h.storage.article))
	for _, article := range h.storage.article {
		articles = append(articles, article)
	}

	users := make([]*User, 0, len(h.storage.users))
	for _, user := range h.storage.users {
		users = append(users, user)
	}
	h.storage.mu.Unlock()

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].CreatedAt.Before(articles[j].CreatedAt)
	})

	response := ArticlesResponse{
		Articles: make([]ArticleResponse, 0),
	}

	for _, article := range articles {
		var author *User

		for _, user := range users {
			if user.ID == article.AuthorID {
				author = user
				break
			}
		}

		if author == nil {
			continue
		}

		if authorFilter != "" && author.Username != authorFilter {
			continue
		}

		if tagFilter != "" {
			found := false

			for _, tag := range article.TagList {
				if tag == tagFilter {
					found = true
					break
				}
			}

			if !found {
				continue
			}
		}

		response.Articles = append(response.Articles, ArticleResponse{
			Slug:        article.Slug,
			Title:       article.Title,
			Description: article.Description,
			Body:        article.Body,
			TagList:     article.TagList,
			CreatedAt:   article.CreatedAt,
			UpdatedAt:   article.UpdatedAt,

			Author: ArticleAuthorResponse{
				Username: author.Username,
				Bio:      author.Bio,
				Image:    author.Image,
			},
		})
	}

	response.ArticlesCount = len(response.Articles)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}

func (h *handler) ArticlesCreate(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey).(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ArticleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Article.Title == "" || req.Article.Description == "" || req.Article.Body == "" {
		http.Error(w, "title, description and body are required", http.StatusUnprocessableEntity)
		return
	}

	h.storage.mu.Lock()
	var author *User
	for _, user := range h.storage.users {
		if user.ID == session.UserID {
			author = user
			break
		}
	}
	h.storage.mu.Unlock()

	if author == nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	now := time.Now()

	slug := RandStringRunes(16)

	article := &Article{
		Slug:        slug,
		Title:       req.Article.Title,
		Description: req.Article.Description,
		Body:        req.Article.Body,
		TagList:     req.Article.TagList,
		AuthorID:    author.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	h.storage.mu.Lock()
	h.storage.article[article.Slug] = article
	h.storage.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := SingleArticleResponse{
		Article: ArticleResponse{
			Slug:        article.Slug,
			Title:       article.Title,
			Description: article.Description,
			Body:        article.Body,
			TagList:     article.TagList,
			CreatedAt:   article.CreatedAt,
			UpdatedAt:   article.UpdatedAt,

			Author: ArticleAuthorResponse{
				Username: author.Username,
				Bio:      author.Bio,
				Image:    author.Image,
			},
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
