package storage

import "sync"

type MemoryStorage[T any] struct {
	mu   sync.Mutex
	data map[string]T
}

func NewMemoryStorage[T any]() *MemoryStorage[T] {
	return &MemoryStorage[T]{
		data: make(map[string]T),
	}
}

func (s *MemoryStorage[T]) Begin() {
	s.mu.Lock()
}

func (s *MemoryStorage[T]) End() {
	s.mu.Unlock()
}

func (s *MemoryStorage[T]) Get(key string) (T, bool) {
	value, ok := s.data[key]
	return value, ok
}

func (s *MemoryStorage[T]) List() []T {
	result := make([]T, 0, len(s.data))

	for _, value := range s.data {
		result = append(result, value)
	}

	return result
}

func (s *MemoryStorage[T]) Create(key string, value T) {
	s.data[key] = value
}

func (s *MemoryStorage[T]) Delete(key string) {
	delete(s.data, key)
}
