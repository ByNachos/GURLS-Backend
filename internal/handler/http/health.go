package http

import (
	"GURLS-Backend/internal/repository"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// HealthHandler обработчик health checks
type HealthHandler struct {
	storage repository.Storage
	log     *zap.Logger
}

// NewHealthHandler создает новый health handler
func NewHealthHandler(storage repository.Storage, log *zap.Logger) *HealthHandler {
	return &HealthHandler{
		storage: storage,
		log:     log,
	}
}

// HealthResponse структура ответа health check
type HealthResponse struct {
	Status       string    `json:"status"`
	Timestamp    time.Time `json:"timestamp"`
	Version      string    `json:"version"`
	DatabaseStatus string  `json:"database_status"`
	Uptime       string    `json:"uptime,omitempty"`
}

var startTime = time.Now()

// Health основной health check endpoint
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Проверяем состояние базы данных
	dbStatus := "healthy"
	
	// Простая проверка - пытаемся получить несуществующую ссылку
	_, err := h.storage.GetLink(ctx, "health-check-non-existent")
	if err != nil && err != repository.ErrAliasNotFound {
		// Если ошибка не "не найдено", значит проблемы с БД
		dbStatus = "unhealthy"
		h.log.Error("database health check failed", zap.Error(err))
	}

	status := "healthy"
	statusCode := http.StatusOK

	if dbStatus == "unhealthy" {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	response := HealthResponse{
		Status:         status,
		Timestamp:      time.Now(),
		Version:        "1.0.0", // Можно вынести в конфигурацию
		DatabaseStatus: dbStatus,
		Uptime:         time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("failed to encode health response", zap.Error(err))
	}

	if status == "healthy" {
		h.log.Debug("health check passed")
	} else {
		h.log.Warn("health check failed", zap.String("database_status", dbStatus))
	}
}

// Ready readiness probe endpoint (упрощенная версия)
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Простая проверка готовности - можем ли мы обработать запросы
	response := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("failed to encode ready response", zap.Error(err))
	}
}

// Metrics простой endpoint с метриками (может быть расширен)
func (h *HealthHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"uptime_seconds": time.Since(startTime).Seconds(),
		"timestamp":      time.Now(),
		"version":        "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		h.log.Error("failed to encode metrics response", zap.Error(err))
	}
}