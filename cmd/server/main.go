package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/Dhanya562004/quickcart-go/internal/cart"
	"github.com/Dhanya562004/quickcart-go/internal/catalog"
	"github.com/Dhanya562004/quickcart-go/internal/config"
	"github.com/Dhanya562004/quickcart-go/internal/db"
	"github.com/Dhanya562004/quickcart-go/internal/order"
	"github.com/Dhanya562004/quickcart-go/internal/platform/database"
	"github.com/Dhanya562004/quickcart-go/internal/platform/logger"
	"github.com/Dhanya562004/quickcart-go/internal/platform/middleware"
	"github.com/Dhanya562004/quickcart-go/internal/queue"
	"github.com/Dhanya562004/quickcart-go/internal/worker"
	"github.com/gin-gonic/gin"
)

var (
	// Version can be injected at build time via -ldflags="-X main.Version=..."
	Version = "1.0.0-phase3"
)

func main() {
	// 1. Load application configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize structured logging
	log := logger.New(string(cfg.Environment), cfg.LogLevel, os.Stdout)
	slog.SetDefault(log)

	log.Info("Starting QuickCart Engine",
		slog.String("version", Version),
		slog.String("environment", string(cfg.Environment)),
		slog.String("log_level", cfg.LogLevel),
	)

	// 3. Initialize database connection pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbConn, err := database.NewPool(ctx, cfg, log)
	if err != nil {
		log.Error("Database initialization failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbConn.Close()

	// Initialize sqlc queries wrapper
	queries := db.New(dbConn.Pool)

	// 4. Initialize Queue and Concurrency Worker Pool
	jobQueue := queue.NewChannelQueue(1000)
	workerPool := worker.NewWorkerPool(worker.PoolConfig{
		WorkerCount:  cfg.WorkerPoolSize,
		DrainTimeout: cfg.WorkerDrainTimeout,
	}, jobQueue, log)

	workerPool.RegisterHandler(queue.JobTypeOrderConfirm, worker.DefaultOrderConfirmHandler(log))
	workerPool.RegisterHandler(queue.JobTypeReservationReclaim, worker.DefaultReservationReclaimHandler(log))

	if err := workerPool.Start(ctx); err != nil {
		log.Error("Failed to start background worker pool", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// 5. Setup Gin HTTP router
	router := SetupRouter(cfg, log, dbConn, queries)

	// 6. Configure standard HTTP server with production timeouts
	srv := &http.Server{
		Addr:              cfg.Address(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 7. Start server in a background goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Info("HTTP server listening", slog.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 8. Trapping OS signals for graceful shutdown
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErrors:
		log.Error("Critical server error", slog.String("error", err.Error()))
		os.Exit(1)

	case sig := <-shutdownChan:
		log.Info("Shutdown signal received, initiating graceful teardown", slog.String("signal", sig.String()))

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()

		// Step 1: Gracefully drain active HTTP connections
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("HTTP graceful shutdown failed, forcing close", slog.String("error", err.Error()))
			_ = srv.Close()
		}

		// Step 2: Gracefully drain background worker pool
		if err := workerPool.Stop(shutdownCtx); err != nil {
			log.Error("Worker pool shutdown failed", slog.String("error", err.Error()))
		}

		log.Info("QuickCart server gracefully terminated")
	}
}

// SetupRouter creates and configures the Gin engine and endpoints.
func SetupRouter(cfg *config.Config, log *slog.Logger, dbConn *database.DB, queries db.Querier) *gin.Engine {
	if cfg.Environment == config.EnvProduction {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	// Global middleware chain
	r.Use(middleware.RequestID())
	r.Use(middleware.StructuredLogger(log))
	r.Use(middleware.Recovery(log))
	r.Use(middleware.CORS())

	// Liveness check (K8s / Load Balancer) - returns 200 if process is running
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "alive",
			"timestamp": time.Now().UTC(),
		})
	})

	// Readiness check (K8s / Load Balancer) - returns 200 only if DB ping succeeds
	r.GET("/readyz", func(c *gin.Context) {
		pingCtx, pingCancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer pingCancel()

		dbStatus := "connected"
		var dbErr string

		if dbConn == nil || dbConn.Ping(pingCtx) != nil {
			dbStatus = "disconnected"
			if dbConn != nil && dbConn.Ping(pingCtx) != nil {
				dbErr = dbConn.Ping(pingCtx).Error()
			} else {
				dbErr = "database connection pool uninitialized"
			}

			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "not_ready",
				"database":  dbStatus,
				"error":     dbErr,
				"timestamp": time.Now().UTC(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"database":  dbStatus,
			"timestamp": time.Now().UTC(),
		})
	})

	// System Information endpoint
	v1 := r.Group("/api/v1")
	{
		v1.GET("/system/info", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"service":     "QuickCart Concurrent Engine",
				"version":     Version,
				"environment": cfg.Environment,
				"timestamp":   time.Now().UTC(),
			})
		})

		// Register Catalog, Cart, and Order domain modules when database is connected
		if dbConn != nil && dbConn.Pool != nil && queries != nil {
			catRepo := catalog.NewRepository(queries)
			catSvc := catalog.NewService(catRepo)
			catHandler := catalog.NewHandler(catSvc)
			catHandler.RegisterRoutes(v1)

			cartRepo := cart.NewRepository(queries)
			cartSvc := cart.NewService(cartRepo)
			cartHandler := cart.NewHandler(cartSvc)
			cartHandler.RegisterRoutes(v1)

			orderRepo := order.NewRepository(dbConn.Pool)
			orderSvc := order.NewService(orderRepo, cartSvc)
			orderHandler := order.NewHandler(orderSvc)
			orderHandler.RegisterRoutes(v1)
		}
	}

	return r
}
