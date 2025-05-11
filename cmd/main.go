package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go_vk/internal/config"
	"go_vk/internal/grpc_server"
	"go_vk/internal/logger"
	"go_vk/pkg/subpub"
)

const (
	defaultShutdownTimeout = time.Second * 15
)

func main() {
	// Configuration loading
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Logger setup
	appLogger, err := logger.SetupLogger(cfg.LoggerConfig)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	defer appLogger.Sync()
	logger := appLogger.Sugar()

	logger.Info("Application starting...")
	logger.Debugw("Loaded configuration", "config", cfg)

	// SubPub System Initialization
	sp := subpub.NewSubPub(logger.Named("subpub"), cfg.SubPubConfig)
	logger.Info("SubPub initialized.")

	// gRPC GRPCServerConfig Initialization
	srv, err := grpc_server.New(cfg.GRPCServerConfig, sp, logger.Named("grpc_server"))
	if err != nil {
		logger.Fatalf("Failed to create gRPC server: %v", err)
	}

	go func() {
		if err := srv.Start(); err != nil {
			logger.Errorf("gRPC server encountered an error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Infof("Received signal: %s. Starting graceful shutdown...", sig.String())

	shutdownTimeout := cfg.GRPCServerConfig.Timeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
		logger.Infof("Using default shutdown timeout: %s", shutdownTimeout)
	} else {
		logger.Infof("Using shutdown timeout from config: %s", shutdownTimeout)
	}

	// Stop gRPC server
	srv.Stop(shutdownTimeout)

	closeCtx, cancelClose := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelClose()

	logger.Info("Closing SubPub...")
	if err := sp.Close(closeCtx); err != nil {
		logger.Errorw("Error during SubPub close", "error", err)
	} else {
		logger.Info("SubPub closed.")
	}

	logger.Info("Application shut down gracefully.")
}
