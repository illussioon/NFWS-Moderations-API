package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"anti-nsfw-service/internal/config"
	"anti-nsfw-service/internal/handlers"
	"anti-nsfw-service/internal/middleware"
	"anti-nsfw-service/internal/services"
)

func main() {
	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.LoadConfig(logger)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// Initialize NSFW service
	nsfwService := services.NewNSFWService(cfg, logger)

	// Check if service is ready
	if !nsfwService.IsReady() {
		logger.Fatal("Service is not ready - no models loaded")
	}

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	modelHandler := handlers.NewModelHandler(nsfwService)
	scanHandler := handlers.NewScanHandler(nsfwService)
	statsHandler := handlers.NewStatsHandler(nsfwService)

	// Initialize middlewares
	authMiddleware := middleware.NewAuthMiddleware(cfg)
	loggerMiddleware := middleware.NewLoggerMiddleware(logger)
	recoveryMiddleware := middleware.NewRecoveryMiddleware(logger)
	corsMiddleware := middleware.NewCORSMiddleware()
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(
		logger,
		100, // limit: 100 requests
		time.Minute, // per minute
		time.Hour, // block for 1 hour if exceeded
	)

	// Set Gin to release mode
	if cfg.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize router
	router := gin.New()

	// Apply global middlewares
	router.Use(loggerMiddleware.LoggerToFile())
	router.Use(recoveryMiddleware.RecoveryWithZap())
	router.Use(corsMiddleware.SetupCORS())
	router.Use(rateLimitMiddleware.RateLimit())

	// Health and readiness endpoints (no auth required)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// Protected endpoints (auth required)
	protected := router.Group("/")
	protected.Use(authMiddleware.AuthRequired())
	{
		// Model endpoints
		protected.GET("/models", modelHandler.GetModels)

		// Scan endpoints
		protected.POST("/scan", scanHandler.Scan)
		protected.POST("/scan/multipart", scanHandler.ScanMultipart)
		protected.POST("/scan/batch", scanHandler.ScanBatch)
		protected.POST("/scan/detect", scanHandler.Detect)

		// Stats endpoints
		protected.GET("/stats", statsHandler.GetStats)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Run server in a goroutine
	go func() {
		logger.Info("Starting server", zap.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	} else {
		logger.Info("Server exited gracefully")
	}
}

// initLogger initializes the logger with proper configuration
func initLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)

	return config.Build()
}