package http

import (
	"GURLS-Backend/internal/repository"
	"net"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// RedirectHandler обработчик редиректов
type RedirectHandler struct {
	storage repository.Storage
	log     *zap.Logger
}

// NewRedirectHandler создает новый обработчик редиректов
func NewRedirectHandler(storage repository.Storage, log *zap.Logger) *RedirectHandler {
	return &RedirectHandler{
		storage: storage,
		log:     log,
	}
}

// HandleRedirect обрабатывает редирект по alias
func (h *RedirectHandler) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	// Извлекаем alias из URL path
	alias := strings.TrimPrefix(r.URL.Path, "/")
	
	// Проверяем, что это не системные endpoints
	if alias == "" || strings.HasPrefix(alias, "api/") || 
	   strings.HasPrefix(alias, "health") || strings.HasPrefix(alias, "ready") ||
	   strings.HasPrefix(alias, "metrics") {
		http.NotFound(w, r)
		return
	}

	// Извлекаем информацию для аналитики
	ipAddress := extractIPAddress(r)
	userAgent := r.UserAgent()
	referer := r.Referer()

	// Используем atomic метод для получения ссылки и записи клика
	link, err := h.storage.GetLinkAndRecordClick(r.Context(), alias, &ipAddress, &userAgent, &referer)
	if err != nil {
		if err == repository.ErrAliasNotFound {
			h.log.Debug("alias not found", zap.String("alias", alias))
			http.NotFound(w, r)
			return
		}
		h.log.Error("failed to process redirect", zap.String("alias", alias), zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Определяем тип устройства для дополнительной аналитики
	deviceType := detectDeviceType(userAgent)
	
	// Логируем успешный редирект
	h.log.Info("successful redirect", 
		zap.String("alias", alias),
		zap.String("original_url", link.OriginalURL),
		zap.String("ip", ipAddress),
		zap.String("device_type", deviceType),
		zap.String("user_agent", userAgent))

	// Выполняем редирект
	http.Redirect(w, r, link.OriginalURL, http.StatusFound)
}

// extractIPAddress извлекает IP адрес из запроса с учетом прокси
func extractIPAddress(r *http.Request) string {
	// Проверяем заголовки прокси в порядке приоритета
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For может содержать список IP через запятую
		ips := strings.Split(ip, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}

	if ip := r.Header.Get("X-Client-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}

	// Fallback к RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// detectDeviceType определяет тип устройства по User-Agent
func detectDeviceType(userAgent string) string {
	userAgentLower := strings.ToLower(userAgent)

	// Проверяем мобильные устройства
	mobileKeywords := []string{
		"mobile", "android", "iphone", "ipod", "blackberry", 
		"windows phone", "webos", "opera mini",
	}
	for _, keyword := range mobileKeywords {
		if strings.Contains(userAgentLower, keyword) {
			return "mobile"
		}
	}

	// Проверяем планшеты
	tabletKeywords := []string{
		"tablet", "ipad", "kindle", "silk", "playbook",
	}
	for _, keyword := range tabletKeywords {
		if strings.Contains(userAgentLower, keyword) {
			return "tablet"
		}
	}

	// TODO: Если есть User-Agent parser, используем его
	// В будущем можно добавить более точное определение устройств

	// По умолчанию считаем desktop
	return "desktop"
}