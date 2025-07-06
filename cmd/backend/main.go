package main

import (
	"GURLS-Backend/internal/analytics"
	"GURLS-Backend/internal/config"
	"GURLS-Backend/internal/database"
	grpcServer "GURLS-Backend/internal/grpc/server"
	"GURLS-Backend/internal/repository/postgres"
	"GURLS-Backend/internal/service"
	"GURLS-Backend/pkg/logger"
	"GURLS-Backend/pkg/useragent"
	"context"
	"fmt"
	lg "log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/rs/cors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.MustLoad()
	log := logger.New(cfg.Env)
	defer func() {
		if err := log.Sync(); err != nil {
			lg.Printf("ERROR: failed to sync zap logger: %v\n", err)
		}
	}()

	log.Info("starting GURLS-Backend gRPC server", zap.String("env", cfg.Env))

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

	// Initialize analytics processor
	analyticsConfig := analytics.DefaultConfig()
	analyticsProcessor := analytics.NewProcessor(storage, log, analyticsConfig)
	
	// Start analytics processor
	if err := analyticsProcessor.Start(); err != nil {
		log.Fatal("failed to start analytics processor", zap.Error(err))
	}
	
	log.Info("analytics processor started successfully")

	// Create and configure gRPC server
	gRPCServer := grpc.NewServer()
	grpcServer.Register(gRPCServer, log, urlShortenerService, storage, analyticsProcessor)

	// Start gRPC server (for Bot/Redirect)
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCServer.Port))
	if err != nil {
		log.Fatal("failed to listen for gRPC", zap.Error(err))
	}

	log.Info("gRPC server listening", zap.String("address", grpcLis.Addr().String()))

	// Start gRPC server in goroutine
	go func() {
		if err := gRPCServer.Serve(grpcLis); err != nil {
			log.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// Create gRPC-Web wrapper for Frontend
	grpcWebServer := grpcweb.WrapServer(gRPCServer,
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			// Allow all origins for development - should be restricted in production
			return true
		}),
	)

	// Setup CORS for browser requests
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",  // React dev server
			"http://localhost:8080",  // Production build
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Accept-Language", 
			"Content-Language",
			"Content-Type",
			"X-Grpc-Web",
			"X-User-Agent",
		},
		ExposedHeaders: []string{
			"Grpc-Status",
			"Grpc-Message",
			"Grpc-Status-Details-Bin",
		},
		AllowCredentials: false,
		MaxAge:          86400, // 24 hours
	})

	// Create HTTP server for gRPC-Web
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.GRPCServer.WebPort),
		Handler: corsHandler.Handler(grpcWebServer),
	}

	log.Info("gRPC-Web server listening", zap.Int("port", cfg.GRPCServer.WebPort))

	// Start gRPC-Web server in goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("gRPC-Web server failed", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down GURLS-Backend...")

	// Gracefully stop gRPC-Web server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown gRPC-Web server", zap.Error(err))
	} else {
		log.Info("gRPC-Web server stopped")
	}

	// Gracefully stop gRPC server
	gRPCServer.GracefulStop()
	log.Info("gRPC server stopped")

	// Stop analytics processor
	if err := analyticsProcessor.Stop(); err != nil {
		log.Error("failed to stop analytics processor gracefully", zap.Error(err))
	} else {
		log.Info("analytics processor stopped")
	}
}