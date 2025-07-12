package http

import (
	"GURLS-Backend/internal/auth"
	"GURLS-Backend/internal/repository"
	"GURLS-Backend/internal/service"
	"net/http"
	"strings"

	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
)

// Server HTTP сервер с обработчиками
type Server struct {
	authHandlers         *auth.AuthHandlers
	linksHandler         *LinksHandler
	redirectHandler      *RedirectHandler
	healthHandler        *HealthHandler
	paymentHandler       *PaymentHandler
	subscriptionHandler  *SubscriptionHandler
	authMiddleware       *auth.Middleware
	log                  *zap.Logger
}

// NewServer создает новый HTTP сервер
func NewServer(
	storage repository.Storage,
	urlShortener *service.URLShortenerService,
	paymentService *service.PaymentService,
	jwtService *auth.JWTService,
	passwordService *auth.PasswordService,
	log *zap.Logger,
	baseURL string,
) *Server {
	// Создаем handlers
	authHandlers := auth.NewAuthHandlers(storage, jwtService, passwordService, log)
	linksHandler := NewLinksHandler(storage, urlShortener, log, baseURL)
	redirectHandler := NewRedirectHandler(storage, log)
	healthHandler := NewHealthHandler(storage, log)
	paymentHandler := NewPaymentHandler(storage, paymentService, log)
	subscriptionHandler := NewSubscriptionHandler(storage, log)
	
	// Создаем middleware
	authMiddleware := auth.NewMiddleware(jwtService, log)

	return &Server{
		authHandlers:        authHandlers,
		linksHandler:        linksHandler,
		redirectHandler:     redirectHandler,
		healthHandler:       healthHandler,
		paymentHandler:      paymentHandler,
		subscriptionHandler: subscriptionHandler,
		authMiddleware:      authMiddleware,
		log:                 log,
	}
}

// SetupRoutes настраивает маршруты
func (s *Server) SetupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Health checks (без аутентификации)
	mux.HandleFunc("/health", s.healthHandler.Health)
	mux.HandleFunc("/ready", s.healthHandler.Ready)
	mux.HandleFunc("/metrics", s.healthHandler.Metrics)

	// Swagger документация
	mux.Handle("/api/v1/", httpSwagger.WrapHandler)

	// Auth endpoints (без аутентификации)
	mux.HandleFunc("/api/auth/register", s.withCORS(s.authHandlers.Register))
	mux.HandleFunc("/api/auth/login", s.withCORS(s.authHandlers.Login))

	// API endpoints (с аутентификацией)
	mux.HandleFunc("/api/shorten", s.withCORS(s.authMiddleware.RequireAuth(s.linksHandler.CreateLink)))
	mux.HandleFunc("/api/links", s.withCORS(s.authMiddleware.RequireAuth(s.linksHandler.ListLinks)))
	
	// Stats endpoint - обрабатываем через custom router
	mux.HandleFunc("/api/stats/", s.withCORS(s.authMiddleware.RequireAuth(s.linksHandler.GetStats)))
	
	// Delete endpoint - обрабатываем через custom router с авторизацией
	mux.HandleFunc("/api/links/", s.withCORS(s.authMiddleware.RequireAuth(s.handleLinksAPI)))

	// Payment endpoints (с аутентификацией)
	mux.HandleFunc("/api/payments/create", s.withCORS(s.authMiddleware.RequireAuth(s.paymentHandler.CreatePayment)))
	mux.HandleFunc("/api/payments/webhook", s.withCORS(s.paymentHandler.WebhookHandler)) // без аутентификации для webhook
	mux.HandleFunc("/api/payments/status/", s.withCORS(s.authMiddleware.RequireAuth(s.paymentHandler.GetPaymentStatus)))
	mux.HandleFunc("/api/payments", s.withCORS(s.authMiddleware.RequireAuth(s.paymentHandler.ListPayments)))

	// Subscription endpoints (с аутентификацией)
	mux.HandleFunc("/api/subscriptions/plans", s.withCORS(s.subscriptionHandler.ListSubscriptionPlans)) // без аутентификации
	mux.HandleFunc("/api/subscriptions/current", s.withCORS(s.authMiddleware.RequireAuth(s.subscriptionHandler.GetCurrentSubscription)))
	mux.HandleFunc("/api/subscriptions/upgrade", s.withCORS(s.authMiddleware.RequireAuth(s.subscriptionHandler.UpgradeSubscription)))

	// Redirect endpoint (без аутентификации) - должен быть последним
	mux.HandleFunc("/", s.redirectHandler.HandleRedirect)

	return mux
}

// handleLinksAPI обрабатывает /api/links/* endpoints с разными HTTP методами
func (s *Server) handleLinksAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.linksHandler.ListLinks(w, r)
	case http.MethodDelete:
		s.linksHandler.DeleteLink(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// withCORS добавляет CORS headers к обработчику
func (s *Server) withCORS(handler http.HandlerFunc) http.HandlerFunc {
	return s.authMiddleware.CORS(handler)
}

// Utility method для проверки системных путей
func isSystemPath(path string) bool {
	systemPaths := []string{
		"/api/",
		"/health",
		"/ready", 
		"/metrics",
		"/swagger/",
		"/docs/",
	}
	
	for _, systemPath := range systemPaths {
		if strings.HasPrefix(path, systemPath) {
			return true
		}
	}
	
	return false
}