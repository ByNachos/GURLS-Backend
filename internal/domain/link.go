package domain

import "time"

type Link struct {
	Alias          string
	UserID         int64 // ID владельца ссылки
	OriginalURL    string
	Title          string
	ExpiresAt      *time.Time
	ClickCount     int64
	ClicksByDevice map[string]int64
}
