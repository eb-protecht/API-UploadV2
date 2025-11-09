package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"upload-service/configs"
	//"upload-service/controllers"
	"upload-service/middleware"
	"upload-service/routes"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger first
	configs.InitLogger()
	logger := configs.LogWithContext("api-uploadv2", "startup")

	logger.Info("Starting API-UploadV2 service initialization")

	router := mux.NewRouter()

	// Add middleware
	router.Use(middleware.LoggingMiddleware)
	router.Use(middleware.RecoveryMiddleware)

	logger.Info("Middleware configured")

	// Initialize database connections with logging
	logger.Info("Connecting to databases...")

	if err := initializeDatabases(logger); err != nil {
		logger.Fatal("Failed to initialize databases", "error", err)
		return
	}

	logger.Info("Connecting to Redis...")
	if err := configs.ConnectREDISDB(); err != nil {
		logger.Fatal("Failed to connect to Redis", "error", err)
		return
	}
	logger.Info("Redis connected successfully")

	logger.Info("Connecting to AWS S3...")
	if err := initializeAWS(logger); err != nil {
		logger.Fatal("Failed to initialize AWS S3", "error", err)
		return
	}

	// Start live stream monitor TODO STOP FOR NOW
	//go controllers.MonitorLiveStreams()
	logger.Info("Live stream monitor started")

	// Register routes with logging
	logger.Info("Registering API routes...")
	registerRoutes(router, logger)

	// Health check endpoints
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}).Methods("GET")

	router.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Ready")
	}).Methods("GET")

	// Get port configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "3006"
	}

	// Create server with timeouts
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("API-UploadV2 service started", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", "error", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	} else {
		logger.Info("Server shutdown complete")
	}

}

func initializeAWS(logger *logrus.Entry) error {
	start := time.Now()
	err := configs.ConnectAWS()
	if err != nil {
		logger.Error("AWS S3 connection failed", "error", err, "duration", time.Since(start))
		return fmt.Errorf("aws s3 connection failed: %w", err)
	}
	logger.Info("AWS S3 connected successfully",
		"region", configs.EnvAWSRegion(),
		"raw_bucket", configs.EnvRawBucket(),
		"duration", time.Since(start))
	return nil
}

func initializeDatabases(logger *logrus.Entry) error {
	// Connect to MongoDB
	start := time.Now()
	err := connectMongoDB()
	if err != nil {
		logger.Error("MongoDB connection failed", "error", err, "duration", time.Since(start))
		return fmt.Errorf("mongodb connection failed: %w", err)
	}
	logger.Info("MongoDB connected successfully", "duration", time.Since(start))

	// Connect to PostgreSQL
	start = time.Now()
	err = connectPostgreSQL()
	if err != nil {
		logger.Error("PostgreSQL connection failed", "error", err, "duration", time.Since(start))
		return fmt.Errorf("postgresql connection failed: %w", err)
	}
	logger.Info("PostgreSQL connected successfully", "duration", time.Since(start))

	return nil
}

func connectMongoDB() error {
	// Try to connect with retry logic
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		err := configs.ConnectDB()
		if err == nil {
			return nil
		}
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		} else {
			return err
		}
	}
	return fmt.Errorf("failed to connect after %d retries", maxRetries)
}

func connectPostgreSQL() error {
	return configs.ConnectPSQLDatabase()
}


func registerRoutes(router *mux.Router, logger *logrus.Entry) {
	// Register all route groups with logging
	routes.ContentRoutes(router)
	logger.Info("Content routes registered")

	routes.CommentRoutes(router)
	logger.Info("Comment routes registered")

	routes.LikesRoutes(router)
	logger.Info("Likes routes registered")

	routes.FavoritesRoutes(router)
	logger.Info("Favorites routes registered")

	routes.TransferRoutes(router)
	logger.Info("Transfer routes registered")

	routes.PromotionRoutes(router)
	logger.Info("Promotion routes registered")

	routes.FeedbackRoutes(router)
	logger.Info("Feedback routes registered")

	routes.MediaURLRoutes(router)
	logger.Info("Media URL routes registered")

	// Add static file serving for media files
	router.PathPrefix("/files/").Handler(http.StripPrefix("/files/",
		http.FileServer(http.Dir("/app/media/"))))
	logger.Info("Static file serving routes registered")
}
