package repository

import (
	"GURLS-Backend/internal/domain"
	"context"
	"errors"
)

var (
	ErrAliasNotFound = errors.New("alias not found")
	ErrAliasExists   = errors.New("alias already exists")
)

type Storage interface {
	// User methods
	FindOrCreateUser(ctx context.Context, tgID int64) (*domain.User, error)
	GetUserByTGID(ctx context.Context, tgID int64) (*domain.User, error)

	// Link methods
	SaveLink(ctx context.Context, link *domain.Link) error
	GetLink(ctx context.Context, alias string) (*domain.Link, error)
	DeleteLink(ctx context.Context, alias string) error
	AliasExists(ctx context.Context, alias string) (bool, error)
	RecordClick(ctx context.Context, alias string, deviceType string) error
	ListUserLinks(ctx context.Context, userID int64) ([]*domain.Link, error)
}
