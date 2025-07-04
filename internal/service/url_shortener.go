package service

import (
	"GURLS-Backend/internal/config"
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"GURLS-Backend/pkg/random"
	"context"
	"fmt"
)

const maxRetries = 5

type URLShortenerService struct {
	storage repository.Storage
	config  *config.URLShortener
}

func NewURLShortener(storage repository.Storage, cfg *config.URLShortener) *URLShortenerService {
	return &URLShortenerService{
		storage: storage,
		config:  cfg,
	}
}

// Shorten теперь также обрабатывает кастомный алиас
func (s *URLShortenerService) Shorten(ctx context.Context, link *domain.Link, customAlias *string) (string, error) {
	var alias string
	if customAlias != nil && *customAlias != "" {
		alias = *customAlias
		exists, err := s.storage.AliasExists(ctx, alias)
		if err != nil {
			return "", fmt.Errorf("failed to check custom alias existence: %w", err)
		}
		if exists {
			return "", repository.ErrAliasExists
		}
	} else {
		var err error
		for i := 0; i < maxRetries; i++ {
			alias, err = random.NewRandomString(s.config.AliasLength)
			if err != nil {
				return "", fmt.Errorf("failed to generate alias: %w", err)
			}
			exists, err := s.storage.AliasExists(ctx, alias)
			if err != nil {
				return "", fmt.Errorf("failed to check alias existence: %w", err)
			}
			if !exists {
				break
			}
		}
	}

	link.Alias = alias

	if err := s.storage.SaveLink(ctx, link); err != nil {
		return "", fmt.Errorf("failed to save link: %w", err)
	}

	return alias, nil
}
