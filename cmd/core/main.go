package main

import (
	"log"

	"github.com/polyshift/microkernel/internal/core/config"
	"github.com/polyshift/microkernel/internal/core/gateway"
	"github.com/polyshift/microkernel/internal/core/plugin"
)

func main() {
	// 1. Load Config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Init Plugin Manager
	pluginMgr := plugin.NewManager()
	if err := pluginMgr.LoadPlugins(cfg.Plugins); err != nil {
		log.Fatalf("Failed to load plugins: %v", err)
	}

	// 3. Start Gateway
	server := gateway.NewServer(cfg.Server, cfg.Auth, cfg.RateLimit, cfg.Plugins, pluginMgr)
	log.Printf("Starting Microkernel Core on port %d...", cfg.Server.Port)
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
