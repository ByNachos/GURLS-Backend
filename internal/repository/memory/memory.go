package memory

import (
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"context"
	"errors"
	"sync"
	"time"
)

type MemStorage struct {
	mu          sync.RWMutex
	links       map[string]*domain.Link
	usersByTgID map[int64]*domain.User
	userCounter int64
}

func New() *MemStorage {
	return &MemStorage{
		links:       make(map[string]*domain.Link),
		usersByTgID: make(map[int64]*domain.User),
	}
}

// --- User Methods ---

func (s *MemStorage) FindOrCreateUser(_ context.Context, tgID int64) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if user, exists := s.usersByTgID[tgID]; exists {
		return user, nil
	}

	s.userCounter++
	newUser := &domain.User{
		ID:        s.userCounter,
		TgID:      tgID,
		CreatedAt: time.Now(),
	}
	s.usersByTgID[tgID] = newUser

	return newUser, nil
}

func (s *MemStorage) GetUserByTGID(_ context.Context, tgID int64) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByTgID[tgID]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// --- Link Methods ---

func (s *MemStorage) SaveLink(_ context.Context, link *domain.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Проверяем, существует ли уже такой алиас
	if _, exists := s.links[link.Alias]; exists {
		return repository.ErrAliasExists
	}
	s.links[link.Alias] = link
	return nil
}

func (s *MemStorage) GetLink(_ context.Context, alias string) (*domain.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	link, ok := s.links[alias]
	if !ok {
		return nil, repository.ErrAliasNotFound
	}
	return link, nil
}

func (s *MemStorage) DeleteLink(_ context.Context, alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.links[alias]; !ok {
		return repository.ErrAliasNotFound
	}
	delete(s.links, alias)
	return nil
}

func (s *MemStorage) AliasExists(_ context.Context, alias string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.links[alias]
	return ok, nil
}

func (s *MemStorage) RecordClick(_ context.Context, alias string, deviceType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	link, ok := s.links[alias]
	if !ok {
		return repository.ErrAliasNotFound
	}
	if link.ClicksByDevice == nil {
		link.ClicksByDevice = make(map[string]int64)
	}
	link.ClickCount++
	link.ClicksByDevice[deviceType]++
	return nil
}

func (s *MemStorage) ListUserLinks(_ context.Context, userID int64) ([]*domain.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var userLinks []*domain.Link
	for _, link := range s.links {
		if link.UserID == userID {
			userLinks = append(userLinks, link)
		}
	}
	return userLinks, nil
}
