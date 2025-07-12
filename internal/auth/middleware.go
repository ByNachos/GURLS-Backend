package auth

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

// ContextKey тип для ключей контекста
type ContextKey string

const (
	// UserIDKey ключ для получения ID пользователя из контекста
	UserIDKey ContextKey = "user_id"
	// UserEmailKey ключ для получения email пользователя из контекста
	UserEmailKey ContextKey = "user_email"
)

// Middleware JWT middleware для HTTP обработчиков
type Middleware struct {
	jwtService *JWTService
	log        *zap.Logger
}

// NewMiddleware создает новый JWT middleware
func NewMiddleware(jwtService *JWTService, log *zap.Logger) *Middleware {
	return &Middleware{
		jwtService: jwtService,
		log:        log,
	}
}

// RequireAuth middleware для проверки JWT токена
func (m *Middleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.log.Debug("missing authorization header")
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		tokenString := ExtractTokenFromBearer(authHeader)
		if tokenString == "" {
			m.log.Debug("invalid authorization header format")
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			m.log.Debug("invalid token", zap.Error(err))
			if err == ErrExpiredToken {
				http.Error(w, "Token expired", http.StatusUnauthorized)
			} else {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
			}
			return
		}

		// Добавляем информацию о пользователе в контекст
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		
		m.log.Debug("authenticated user", 
			zap.Int64("user_id", claims.UserID),
			zap.String("email", claims.Email))

		// Передаем управление следующему обработчику
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// OptionalAuth middleware для опциональной проверки JWT токена
func (m *Middleware) OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// Если токен не предоставлен, просто передаем управление дальше
			next.ServeHTTP(w, r)
			return
		}

		tokenString := ExtractTokenFromBearer(authHeader)
		if tokenString == "" {
			// Неверный формат, но не критично для опционального middleware
			next.ServeHTTP(w, r)
			return
		}

		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			// Неверный токен, но для опционального middleware это не критично
			m.log.Debug("optional auth: invalid token", zap.Error(err))
			next.ServeHTTP(w, r)
			return
		}

		// Добавляем информацию о пользователе в контекст
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// GetUserIDFromContext извлекает ID пользователя из контекста
func GetUserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(UserIDKey).(int64)
	return userID, ok
}

// GetUserEmailFromContext извлекает email пользователя из контекста
func GetUserEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmailKey).(string)
	return email, ok
}

// CORS middleware для обработки CORS запросов
func (m *Middleware) CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		
		// Список разрешенных origins для разработки
		allowedOrigins := []string{
			"http://localhost:3000",  // React dev server
			"http://127.0.0.1:3000",
			"http://localhost:8080",  // Production build
			"http://127.0.0.1:8080",
		}

		// Проверяем origin
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Обработка preflight OPTIONS запросов
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}