package http

import (
	"GURLS-Backend/internal/auth"
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"GURLS-Backend/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// LinksHandler обработчик для работы со ссылками
type LinksHandler struct {
	storage           repository.Storage
	urlShortener      *service.URLShortenerService
	log               *zap.Logger
	baseURL           string
}

// NewLinksHandler создает новый обработчик ссылок
func NewLinksHandler(storage repository.Storage, urlShortener *service.URLShortenerService, log *zap.Logger, baseURL string) *LinksHandler {
	return &LinksHandler{
		storage:      storage,
		urlShortener: urlShortener,
		log:          log,
		baseURL:      baseURL,
	}
}

// CreateLinkRequest структура запроса создания ссылки
type CreateLinkRequest struct {
	OriginalURL string `json:"original_url"`
	Title       string `json:"title,omitempty"`
	CustomAlias string `json:"custom_alias,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

// CreateLinkResponse структура ответа создания ссылки
type CreateLinkResponse struct {
	Alias string `json:"alias"`
	ShortURL string `json:"short_url,omitempty"`
}

// LinkInfo информация о ссылке
type LinkInfo struct {
	Alias       string `json:"alias"`
	OriginalURL string `json:"original_url"`
	Title       string `json:"title,omitempty"`
	ClickCount  int64  `json:"click_count"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

// ListLinksResponse структура ответа списка ссылок
type ListLinksResponse struct {
	Links []LinkInfo `json:"links"`
}

// GetStatsResponse структура ответа статистики
type GetStatsResponse struct {
	Alias           string            `json:"alias"`
	OriginalURL     string            `json:"original_url"`
	ClickCount      int64             `json:"click_count"`
	Title           string            `json:"title,omitempty"`
	ExpiresAt       string            `json:"expires_at,omitempty"`
	ClicksByDevice  map[string]int64  `json:"clicks_by_device"`
	CreatedAt       string            `json:"created_at"`
}

// CreateLink создает новую короткую ссылку
//
//	@Summary		Create a short link
//	@Description	Create a new shortened URL
//	@Tags			Links
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CreateLinkRequest	true	"Link creation request"
//	@Success		201		{object}	CreateLinkResponse	"Link created successfully"
//	@Failure		400		{object}	map[string]string	"Invalid request data"
//	@Failure		401		{object}	map[string]string	"Authentication required"
//	@Failure		403		{object}	map[string]string	"Subscription limit reached"
//	@Failure		409		{object}	map[string]string	"Alias already exists"
//	@Router			/api/shorten [post]
func (h *LinksHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста (установлен JWT middleware)
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Проверяем лимиты подписки пользователя
	canCreate, err := h.checkSubscriptionLimits(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to check subscription limits", zap.Int64("user_id", userID), zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !canCreate {
		h.writeError(w, "Subscription limit reached. Please upgrade your plan to create more links.", http.StatusForbidden)
		return
	}

	var req CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Debug("invalid create link request", zap.Error(err))
		h.writeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Валидация URL
	if req.OriginalURL == "" {
		h.writeError(w, "Original URL is required", http.StatusBadRequest)
		return
	}

	// Создаем объект ссылки
	link := &domain.Link{
		UserID:      userID,
		OriginalURL: req.OriginalURL,
		IsActive:    true,
	}

	// Устанавливаем title если он предоставлен
	if req.Title != "" {
		link.Title = &req.Title
	}

	// Обрабатываем кастомный алиас
	if req.CustomAlias != "" {
		// Проверяем, доступны ли кастомные алиасы в подписке
		hasCustomAliasAccess, err := h.checkCustomAliasAccess(r.Context(), userID)
		if err != nil {
			h.log.Error("failed to check custom alias access", zap.Int64("user_id", userID), zap.Error(err))
			h.writeError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !hasCustomAliasAccess {
			h.writeError(w, "Custom aliases are not available in your current subscription plan. Please upgrade to use this feature.", http.StatusForbidden)
			return
		}
		link.Alias = req.CustomAlias
	}

	// Обрабатываем дату истечения
	if req.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			h.writeError(w, "Invalid expires_at format. Use RFC3339 format", http.StatusBadRequest)
			return
		}
		link.ExpiresAt = &expiresAt
	}

	// Используем сервис для создания ссылки
	var customAlias *string
	if req.CustomAlias != "" {
		customAlias = &req.CustomAlias
	}
	
	alias, err := h.urlShortener.Shorten(r.Context(), link, customAlias)
	if err != nil {
		if err == repository.ErrAliasExists {
			h.writeError(w, "Alias already exists", http.StatusConflict)
			return
		}
		h.log.Error("failed to create link", zap.Error(err))
		h.writeError(w, "Failed to create link", http.StatusInternalServerError)
		return
	}

	// Отправляем ответ
	response := CreateLinkResponse{
		Alias:    alias,
		ShortURL: h.baseURL + "/" + alias,
	}

	h.log.Info("created link", zap.String("alias", alias), zap.Int64("user_id", userID))
	h.writeJSON(w, response, http.StatusCreated)
}

// ListLinks возвращает список ссылок пользователя
func (h *LinksHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Получаем ссылки пользователя
	links, err := h.storage.ListUserLinks(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to list user links", zap.Int64("user_id", userID), zap.Error(err))
		h.writeError(w, "Failed to retrieve links", http.StatusInternalServerError)
		return
	}

	// Преобразуем в ответ
	linkInfos := make([]LinkInfo, len(links))
	for i, link := range links {
		linkInfo := LinkInfo{
			Alias:       link.Alias,
			OriginalURL: link.OriginalURL,
			ClickCount:  int64(link.ClickCount),
			CreatedAt:   link.CreatedAt.Format(time.RFC3339),
		}
		if link.Title != nil {
			linkInfo.Title = *link.Title
		}
		if link.ExpiresAt != nil {
			linkInfo.ExpiresAt = link.ExpiresAt.Format(time.RFC3339)
		}
		linkInfos[i] = linkInfo
	}

	response := ListLinksResponse{
		Links: linkInfos,
	}

	h.writeJSON(w, response, http.StatusOK)
}

// GetStats возвращает статистику по ссылке
func (h *LinksHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	// Извлекаем alias из URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[2] == "" {
		h.writeError(w, "Alias is required", http.StatusBadRequest)
		return
	}
	alias := pathParts[2]

	// Получаем ссылку
	link, err := h.storage.GetLink(r.Context(), alias)
	if err != nil {
		if err == repository.ErrAliasNotFound {
			h.writeError(w, "Link not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to get link for stats", zap.String("alias", alias), zap.Error(err))
		h.writeError(w, "Failed to retrieve link", http.StatusInternalServerError)
		return
	}

	// Проверяем права доступа (пользователь может видеть только свои ссылки)
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok || link.UserID != userID {
		h.writeError(w, "Access denied", http.StatusForbidden)
		return
	}

	// Получаем статистику по устройствам
	clicksByDevice, err := h.storage.GetClicksByDevice(r.Context(), link.ID)
	if err != nil {
		h.log.Error("failed to get clicks by device", zap.Int64("link_id", link.ID), zap.Error(err))
		clicksByDevice = make(map[string]int64) // Возвращаем пустую карту в случае ошибки
	}

	// Формируем ответ
	response := GetStatsResponse{
		Alias:           link.Alias,
		OriginalURL:     link.OriginalURL,
		ClickCount:      int64(link.ClickCount),
		ClicksByDevice:  clicksByDevice,
		CreatedAt:       link.CreatedAt.Format(time.RFC3339),
	}
	
	if link.Title != nil {
		response.Title = *link.Title
	}

	if link.ExpiresAt != nil {
		response.ExpiresAt = link.ExpiresAt.Format(time.RFC3339)
	}

	h.writeJSON(w, response, http.StatusOK)
}

// DeleteLink удаляет ссылку
//
//	@Summary		Delete a link
//	@Description	Delete a specific link by alias
//	@Tags			Links
//	@Security		BearerAuth
//	@Param			alias	path	string	true	"Link alias"
//	@Success		204		"Link deleted successfully"
//	@Failure		401		{object}	map[string]string	"Authentication required"
//	@Failure		403		{object}	map[string]string	"Access denied"
//	@Failure		404		{object}	map[string]string	"Link not found"
//	@Router			/api/links/{alias} [delete]
func (h *LinksHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	// Извлекаем alias из URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[2] == "" {
		h.writeError(w, "Alias is required", http.StatusBadRequest)
		return
	}
	alias := pathParts[2]

	// Получаем ID пользователя из контекста
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Проверяем, что ссылка существует и принадлежит пользователю
	link, err := h.storage.GetLink(r.Context(), alias)
	if err != nil {
		if err == repository.ErrAliasNotFound {
			h.writeError(w, "Link not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to get link for deletion", zap.String("alias", alias), zap.Error(err))
		h.writeError(w, "Failed to retrieve link", http.StatusInternalServerError)
		return
	}

	// Проверяем права доступа
	if link.UserID != userID {
		h.writeError(w, "Access denied", http.StatusForbidden)
		return
	}

	// Удаляем ссылку
	if err := h.storage.DeleteLink(r.Context(), alias); err != nil {
		h.log.Error("failed to delete link", zap.String("alias", alias), zap.Error(err))
		h.writeError(w, "Failed to delete link", http.StatusInternalServerError)
		return
	}

	h.log.Info("deleted link", zap.String("alias", alias), zap.Int64("user_id", userID))
	w.WriteHeader(http.StatusNoContent)
}

// Helper methods

func (h *LinksHandler) writeJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (h *LinksHandler) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// checkSubscriptionLimits проверяет лимиты подписки пользователя
func (h *LinksHandler) checkSubscriptionLimits(ctx context.Context, userID int64) (bool, error) {
	// Получаем пользователя с подпиской
	user, err := h.storage.GetUserByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	// Получаем информацию о подписке
	subscription, err := h.storage.GetSubscriptionType(ctx, user.SubscriptionTypeID)
	if err != nil {
		return false, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Если лимит не установлен (NULL), значит безлимитно
	if subscription.MaxLinksPerMonth == nil {
		return true, nil
	}

	// Получаем количество созданных ссылок в этом месяце
	links, err := h.storage.ListUserLinks(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user links: %w", err)
	}

	// Подсчитываем ссылки, созданные в текущем месяце
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	
	linksThisMonth := 0
	for _, link := range links {
		if link.CreatedAt.After(monthStart) {
			linksThisMonth++
		}
	}

	// Проверяем лимит
	return linksThisMonth < *subscription.MaxLinksPerMonth, nil
}

// checkCustomAliasAccess проверяет доступ к кастомным алиасам
func (h *LinksHandler) checkCustomAliasAccess(ctx context.Context, userID int64) (bool, error) {
	// Получаем пользователя с подпиской
	user, err := h.storage.GetUserByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	// Получаем информацию о подписке
	subscription, err := h.storage.GetSubscriptionType(ctx, user.SubscriptionTypeID)
	if err != nil {
		return false, fmt.Errorf("failed to get subscription: %w", err)
	}

	return subscription.CustomAliases, nil
}