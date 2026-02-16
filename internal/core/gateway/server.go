package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/polyshift/microkernel/internal/core/config"
	"github.com/polyshift/microkernel/internal/core/middleware"
	"github.com/polyshift/microkernel/internal/core/plugin"
	pb "github.com/polyshift/microkernel/proto/plugin"
)

type Server struct {
	engine        *gin.Engine
	pluginMgr     *plugin.Manager
	serverConfig  config.ServerConfig
	authConfig    config.AuthConfig
	rateLimitCfg  config.RateLimitConfig
	pluginConfigs []config.PluginConfig
}

func NewServer(serverCfg config.ServerConfig, authCfg config.AuthConfig, rateLimitCfg config.RateLimitConfig, pluginConfigs []config.PluginConfig, mgr *plugin.Manager) *Server {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.RateLimit(rateLimitCfg))
	r.Use(middleware.APIKeyAuth(authCfg))

	s := &Server{
		engine:        r,
		pluginMgr:     mgr,
		serverConfig:  serverCfg,
		authConfig:    authCfg,
		rateLimitCfg:  rateLimitCfg,
		pluginConfigs: pluginConfigs,
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Admin API
	admin := s.engine.Group("/admin")
	{
		admin.GET("/plugins", s.listPlugins)
		admin.POST("/plugins", s.registerPlugin)
		admin.PUT("/plugins/:name/reload", s.reloadPlugin)
		admin.DELETE("/plugins/:name", s.unregisterPlugin)
		admin.GET("/plugins/:name/health", s.checkPluginHealth)
	}

	// 动态注册插件路由
	for _, pCfg := range s.pluginConfigs {
		pluginName := pCfg.Name
		for _, route := range pCfg.Routes {
			// 捕获闭包变量
			currentPluginName := pluginName
			// currentRoute := route

			// 注册到 Gin
			// 注意：这里简单假设 path 是 Gin 兼容的格式
			s.engine.Handle(route.Method, route.Path, func(c *gin.Context) {
				s.forwardToPlugin(c, currentPluginName)
			})
		}
	}

	// 404 Handler for dynamic plugins
	s.engine.NoRoute(func(c *gin.Context) {
		// Try to match dynamic plugins
		path := c.Request.URL.Path
		method := c.Request.Method

		plugins := s.pluginMgr.ListPlugins()
		for _, p := range plugins {
			for _, route := range p.Routes {
				// Simple exact match or prefix match logic can be added here
				// For now, assuming exact match for simplicity or basic parameter matching
				if route.Path == path && route.Method == method {
					s.forwardToPlugin(c, p.Name)
					return
				}
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "404 page not found"})
	})
}

func (s *Server) listPlugins(c *gin.Context) {
	plugins := s.pluginMgr.ListPlugins()
	c.JSON(http.StatusOK, gin.H{"plugins": plugins})
}

func (s *Server) reloadPlugin(c *gin.Context) {
	name := c.Param("name")
	if err := s.pluginMgr.ReloadPlugin(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "reloaded", "plugin": name})
}

func (s *Server) registerPlugin(c *gin.Context) {
	var cfg config.PluginConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.pluginMgr.StartPlugin(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "registered", "plugin": cfg.Name})
}

func (s *Server) unregisterPlugin(c *gin.Context) {
	name := c.Param("name")
	if err := s.pluginMgr.StopPlugin(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "unregistered", "plugin": name})
}

func (s *Server) checkPluginHealth(c *gin.Context) {
	name := c.Param("name")
	ok, err := s.pluginMgr.CheckPluginHealth(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	status := "healthy"
	if !ok {
		status = "unhealthy"
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "plugin": name})
}

func (s *Server) forwardToPlugin(c *gin.Context, pluginName string) {
	// 1. 获取插件实例
	pInstance, ok := s.pluginMgr.GetPlugin(pluginName)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "plugin not active"})
		return
	}

	// 2. 构建 RequestContext
	body, _ := io.ReadAll(c.Request.Body)

	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	reqCtx := &pb.RequestContext{
		RequestId: c.GetString("RequestID"),
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Headers:   headers,
		Body:      body,
		Query:     make(map[string]string),
		Params:    make(map[string]string),
	}

	// 填充 Query
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			reqCtx.Query[k] = v[0]
		}
	}

	// 填充 Params (Gin 的 Path Params)
	for _, param := range c.Params {
		reqCtx.Params[param.Key] = param.Value
	}

	// 3. 调用 gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := pInstance.Client.HandleRequest(ctx, reqCtx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("rpc call failed: %v", err)})
		return
	}

	// 4. 返回响应
	for k, v := range resp.Headers {
		c.Header(k, v)
	}
	c.Data(int(resp.StatusCode), resp.Headers["Content-Type"], resp.Body)
}

func (s *Server) Start() error {
	return s.engine.Run(fmt.Sprintf(":%d", s.serverConfig.Port))
}
