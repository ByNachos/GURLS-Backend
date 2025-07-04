package domain

import "time"

// User представляет пользователя сервиса.
type User struct {
	ID        int64
	TgID      int64
	CreatedAt time.Time
}
