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

	_ "github.com/nassabiq/go-chi-boilerplate/docs/swagger"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
)

// @title Go Chi Boilerplate API
// @version 1.0
// @description Production-Grade REST API boilerplate built with Go, Chi, PostgreSQL, Redis, and NATS JetStream.
// @contact.name API Support
// @contact.url https://github.com/nassabiq/go-chi-boilerplate
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	app, err := InitializeApp()
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	defer app.NATS.Close()

	logger := app.Logger
	cfg := app.Config

	logger.Info("starting application", "name", cfg.App.Name, "environment", cfg.App.Env)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Http.Host, cfg.Http.Port),
		Handler:      app.Handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("http server started", "address", server.Addr)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("failed to shutdown server gracefully", "error", err)
	}

	_ = telemetry.ShutdownTracer(shutdownContext)

	logger.Info("application stopped")
}
