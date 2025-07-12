// Package main provides the entry point for the GURLS URL Shortener service.
//
//	@title			GURLS URL Shortener API
//	@version		1.0.0
//	@description	A minimalistic URL shortener service with subscription-based features.
//	@termsOfService	http://gurls.ru/terms/
//
//	@contact.name	GURLS Support
//	@contact.email	support@gurls.ru
//
//	@license.name	MIT
//	@license.url	https://opensource.org/licenses/MIT
//
//	@host		localhost:8080
//	@BasePath	/
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT Authorization header. Format: "Bearer {token}"
//
//	@externalDocs.description	OpenAPI Specification
//	@externalDocs.url			https://swagger.io/resources/open-api/
package main

import (
	"GURLS-Backend/internal/auth"
	"GURLS-Backend/internal/config"
	"GURLS-Backend/internal/database"
	httpHandler "GURLS-Backend/internal/handler/http"
	"GURLS-Backend/internal/repository/postgres"
	"GURLS-Backend/internal/service"
	"GURLS-Backend/pkg/logger"
	"GURLS-Backend/pkg/useragent"
	"context"
	lg "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	_ "GURLS-Backend/docs" // Import swagger docs
)

func main() {
	cfg := config.MustLoad()
	log := logger.New(cfg.Env)
	defer func() {
		if err := log.Sync(); err != nil {
			lg.Printf("ERROR: failed to sync zap logger: %v\n", err)
		}
	}()

	log.Info("starting GURLS unified service (web-only architecture)", zap.String("env", cfg.Env))

	// Initialize database connection
	db, err := database.NewConnection(&cfg.Database, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer func() {
		if err := database.Close(db, log); err != nil {
			log.Error("failed to close database connection", zap.Error(err))
		}
	}()

	// Run database migrations if enabled
	if cfg.Database.AutoMigrate {
		log.Info("running database migrations (auto_migrate: true)")
		if err := database.AutoMigrate(db, log); err != nil {
			log.Fatal("failed to run database migrations", zap.Error(err))
		}
	} else {
		log.Info("skipping database migrations (auto_migrate: false)")
	}

	// Seed initial data if enabled
	if cfg.Database.SeedData {
		log.Info("seeding database with initial data (seed_data: true)")
		if err := database.SeedData(db, log); err != nil {
			log.Fatal("failed to seed database", zap.Error(err))
		}
	} else {
		log.Info("skipping database seeding (seed_data: false)")
	}

	// Initialize User-Agent parser
	regexesPath := "assets/regexes.yaml"
	if err := useragent.InitGlobalParser(regexesPath, log); err != nil {
		log.Warn("failed to initialize User-Agent parser, using fallback", zap.Error(err))
	}

	// Initialize storage and service
	storage := postgres.New(db, log)
	urlShortenerService := service.NewURLShortener(storage, &cfg.URLShortener)
	
	// Initialize Payment service
	paymentService := service.NewPaymentService(storage, &cfg.Payment, log)

	// Initialize JWT service for authentication
	jwtConfig := &auth.JWTConfig{
		SecretKey:            []byte("your-secret-key-here"), // TODO: Move to config
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 24 * time.Hour * 7, // 7 days
		Issuer:               "GURLS-Backend",
	}
	jwtService := auth.NewJWTService(jwtConfig)
	passwordService := auth.NewPasswordService()

	// Create unified HTTP server
	httpAPIServer := httpHandler.NewServer(
		storage,
		urlShortenerService,
		paymentService,
		jwtService,
		passwordService,
		log,
		cfg.URLShortener.BaseURL,
	)

	// Setup routes
	httpMux := httpAPIServer.SetupRoutes()

	// Create single HTTP server (port 8080) - according to FEATURE.md requirements
	unifiedHTTPServer := &http.Server{
		Addr:         ":8080",
		Handler:      httpMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Info("starting unified HTTP server (web-only architecture)", zap.String("address", ":8080"))

	// Start unified HTTP server in goroutine
	go func() {
		if err := unifiedHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("unified HTTP server failed", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down GURLS unified service...")

	// Gracefully stop HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	// Shutdown unified HTTP server
	if err := unifiedHTTPServer.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown unified HTTP server", zap.Error(err))
	} else {
		log.Info("unified HTTP server stopped")
	}
}