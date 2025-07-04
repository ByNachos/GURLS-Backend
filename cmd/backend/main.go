package main

import (
	"GURLS-Backend/internal/config"
	grpcServer "GURLS-Backend/internal/grpc/server"
	"GURLS-Backend/internal/repository/memory"
	"GURLS-Backend/internal/service"
	"GURLS-Backend/pkg/logger"
	"fmt"
	lg "log"
	"net"
	"os"
	"os/signal"
	"syscall"

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

	// Initialize storage and service
	storage := memory.New()
	urlShortenerService := service.NewURLShortener(storage, &cfg.URLShortener)

	// Create and configure gRPC server
	gRPCServer := grpc.NewServer()
	grpcServer.Register(gRPCServer, log, urlShortenerService, storage)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCServer.Port))
	if err != nil {
		log.Fatal("failed to listen for gRPC", zap.Error(err))
	}

	log.Info("gRPC server listening", zap.String("address", lis.Addr().String()))

	// Start server in goroutine
	go func() {
		if err := gRPCServer.Serve(lis); err != nil {
			log.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down GURLS-Backend...")

	gRPCServer.GracefulStop()
	log.Info("gRPC server stopped")
}