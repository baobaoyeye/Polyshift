package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

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
		if err := tracerShutdown(context.Background()); err != nil {
			slog.Error("Failed to shutdown tracer", "error", err)
		}
	}()

	meterShutdown, err := observability.InitMeter(cfg.Observability)
	if err != nil {
		slog.Error("Failed to init meter", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := meterShutdown(context.Background()); err != nil {
			slog.Error("Failed to shutdown meter", "error", err)
		}
	}()

	// 3. Init Plugin Manager
	pluginMgr := plugin.NewManager(cfg.Resilience)
	if err := pluginMgr.LoadPlugins(cfg.Plugins); err != nil {
		slog.Error("Failed to load plugins", "error", err)
		os.Exit(1)
	}

	// 4. Start Gateway
	server := gateway.NewServer(cfg.Server, cfg.Auth, cfg.RateLimit, cfg.Observability, cfg.Plugins, pluginMgr)
	slog.Info("Starting Microkernel Core", "port", cfg.Server.Port)
	if err := server.Start(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
