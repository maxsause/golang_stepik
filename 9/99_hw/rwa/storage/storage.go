package storage

import (
	"rwa/model"
	"sync"
)

type Storage struct {
	Mu       sync.Mutex
	Users    map[string]*model.User
	Sessions map[string]*model.Session
	Article  map[string]*model.Article
}

func NewStorage() *Storage {
	return &Storage{
		Users:    make(map[string]*model.User),
		Sessions: make(map[string]*model.Session),
		Article:  make(map[string]*model.Article),
	}
}
