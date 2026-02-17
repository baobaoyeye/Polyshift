package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
	"github.com/polyshift/microkernel/internal/core/gateway"
	"github.com/polyshift/microkernel/internal/core/observability"
	"github.com/polyshift/microkernel/internal/core/plugin"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "path to config file")
	flag.Parse()

	// 1. Load Config
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. Init Observability
	observability.InitLogger(cfg.Observability)

	tracerShutdown, err := observability.InitTracer(cfg.Observability)
	if err != nil {
		slog.Error("Failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracerShutdown(ctx); err != nil {
			slog.Error("Failed to shutdown tracer", "error", err)
		}
	}()

	meterShutdown, err := observability.InitMeter(cfg.Observability)
	if err != nil {
		slog.Error("Failed to init meter", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := meterShutdown(ctx); err != nil {
			slog.Error("Failed to shutdown meter", "error", err)
		}
	}()

	// 3. Init Plugin Manager
	pluginMgr := plugin.NewManager(cfg.Resilience)
	if err := pluginMgr.LoadPlugins(cfg.Plugins); err != nil {
		slog.Error("Failed to load plugins", "error", err)
		os.Exit(1)
	}

	// Create a context for signal handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start Watchdog with the context
	pluginMgr.StartWatchdog(ctx)

	// 4. Start Gateway
	server := gateway.NewServer(cfg.Server, cfg.Auth, cfg.RateLimit, cfg.Observability, cfg.Plugins, pluginMgr)
	slog.Info("Starting Microkernel Core", "port", cfg.Server.Port)

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for signal
	<-ctx.Done()
	slog.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exiting")
}
