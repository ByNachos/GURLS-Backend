package auth

import (
	"GURLS-Backend/internal/repository"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// AuthHandlers обработчики аутентификации
type AuthHandlers struct {
	storage         repository.Storage
	jwtService      *JWTService
	passwordService *PasswordService
	log             *zap.Logger
}

// NewAuthHandlers создает новые обработчики аутентификации
func NewAuthHandlers(storage repository.Storage, jwtService *JWTService, passwordService *PasswordService, log *zap.Logger) *AuthHandlers {
	return &AuthHandlers{
		storage:         storage,
		jwtService:      jwtService,
		passwordService: passwordService,
		log:             log,
	}
}

// RegisterRequest структура запроса регистрации
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest структура запроса входа
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse структура ответа аутентификации
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         UserInfo `json:"user"`
}

// UserInfo информация о пользователе
type UserInfo struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// ErrorResponse структура ошибки
type ErrorResponse struct {
	Error string `json:"error"`
}

// Register обработчик регистрации
//
//	@Summary		Register a new user
//	@Description	Create a new user account
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RegisterRequest	true	"Registration request"
//	@Success		201		{object}	AuthResponse	"User registered successfully"
//	@Failure		400		{object}	map[string]string	"Invalid request data"
//	@Failure		409		{object}	map[string]string	"User already exists"
//	@Router			/api/auth/register [post]
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Debug("invalid registration request", zap.Error(err))
		h.writeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Валидация email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if !isValidEmail(req.Email) {
		h.writeError(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	// Валидация пароля
	if err := IsValidPassword(req.Password); err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Проверяем, не существует ли уже пользователь с таким email
	existingUser, err := h.storage.GetUserByEmail(r.Context(), req.Email)
	if err == nil && existingUser != nil {
		h.writeError(w, "User with this email already exists", http.StatusConflict)
		return
	}

	// Хешируем пароль
	hashedPassword, err := h.passwordService.HashPassword(req.Password)
	if err != nil {
		h.log.Error("failed to hash password", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Создаем пользователя
	user, err := h.storage.CreateUser(r.Context(), req.Email, hashedPassword)
	if err != nil {
		h.log.Error("failed to create user", zap.String("email", req.Email), zap.Error(err))
		h.writeError(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Генерируем токены
	accessToken, err := h.jwtService.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		h.log.Error("failed to generate access token", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		h.log.Error("failed to generate refresh token", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Отправляем ответ
	response := AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: UserInfo{
			ID:            user.ID,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
		},
	}

	h.log.Info("user registered successfully", zap.Int64("user_id", user.ID), zap.String("email", req.Email))
	h.writeJSON(w, response, http.StatusCreated)
}

// Login обработчик входа
//
//	@Summary		Login user
//	@Description	Authenticate user and receive JWT tokens
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest	true	"Login request"
//	@Success		200		{object}	AuthResponse	"Login successful"
//	@Failure		400		{object}	map[string]string	"Invalid request data"
//	@Failure		401		{object}	map[string]string	"Invalid credentials"
//	@Router			/api/auth/login [post]
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Debug("invalid login request", zap.Error(err))
		h.writeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Нормализуем email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Находим пользователя
	user, err := h.storage.FindUserByEmailAndPassword(r.Context(), req.Email)
	if err != nil {
		h.log.Debug("user not found for login", zap.String("email", req.Email))
		h.writeError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Проверяем пароль
	if err := h.passwordService.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		h.log.Debug("invalid password for user", zap.String("email", req.Email))
		h.writeError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Обновляем время последнего входа
	now := time.Now()
	user.LastLoginAt = &now
	if err := h.storage.UpdateUser(r.Context(), user); err != nil {
		h.log.Warn("failed to update last login time", zap.Int64("user_id", user.ID), zap.Error(err))
	}

	// Генерируем токены
	accessToken, err := h.jwtService.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		h.log.Error("failed to generate access token", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		h.log.Error("failed to generate refresh token", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Отправляем ответ
	response := AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: UserInfo{
			ID:            user.ID,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
		},
	}

	h.log.Info("user logged in successfully", zap.Int64("user_id", user.ID), zap.String("email", req.Email))
	h.writeJSON(w, response, http.StatusOK)
}

// Helper methods

func (h *AuthHandlers) writeJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (h *AuthHandlers) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func isValidEmail(email string) bool {
	// Простая валидация email
	return strings.Contains(email, "@") && len(email) > 3 && len(email) < 255
}