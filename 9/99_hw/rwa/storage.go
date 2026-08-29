package main

import "sync"

type Storage struct {
	mu       sync.Mutex
	users    map[string]*User
	sessions map[string]*Session
	article  map[string]*Article
}

func NewStorage() *Storage {
	return &Storage{
		users:    make(map[string]*User),
		sessions: make(map[string]*Session),
		article:  make(map[string]*Article),
	}
}
