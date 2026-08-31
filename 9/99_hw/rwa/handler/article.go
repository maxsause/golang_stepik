package handler

import (
	"encoding/json"
	"net/http"
	"rwa/model"
	"rwa/utils"
	"sort"
	"time"
)

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

func (h *Handler) ArticlesGetRecent(w http.ResponseWriter, r *http.Request) {
	authorFilter := r.URL.Query().Get("author")
	tagFilter := r.URL.Query().Get("tag")

	h.Storage.Mu.Lock()
	articles := make([]*model.Article, 0, len(h.Storage.Article))
	for _, article := range h.Storage.Article {
		articles = append(articles, article)
	}

	users := make([]*model.User, 0, len(h.Storage.Users))
	for _, user := range h.Storage.Users {
		users = append(users, user)
	}
	h.Storage.Mu.Unlock()

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].CreatedAt.Before(articles[j].CreatedAt)
	})

	response := ArticlesResponse{
		Articles: make([]ArticleResponse, 0),
	}

	for _, article := range articles {
		var author *model.User

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

func (h *Handler) ArticlesCreate(w http.ResponseWriter, r *http.Request) {
	s, ok := r.Context().Value(sessionContextKey).(*model.Session)
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

	h.Storage.Mu.Lock()
	var author *model.User
	for _, user := range h.Storage.Users {
		if user.ID == s.UserID {
			author = user
			break
		}
	}
	h.Storage.Mu.Unlock()

	if author == nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	now := time.Now()

	slug := utils.RandStringRunes(16)

	article := &model.Article{
		Slug:        slug,
		Title:       req.Article.Title,
		Description: req.Article.Description,
		Body:        req.Article.Body,
		TagList:     req.Article.TagList,
		AuthorID:    author.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	h.Storage.Mu.Lock()
	h.Storage.Article[article.Slug] = article
	h.Storage.Mu.Unlock()

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
