package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/polyshift/microkernel/internal/core/config"
	"github.com/polyshift/microkernel/internal/core/middleware"
	"github.com/polyshift/microkernel/internal/core/plugin"
	"github.com/polyshift/microkernel/internal/core/router"
	pb "github.com/polyshift/microkernel/proto/plugin"
)

type Server struct {
	engine        *gin.Engine
	pluginMgr     *plugin.Manager
	serverConfig  config.ServerConfig
	authConfig    config.AuthConfig
	rateLimitCfg  config.RateLimitConfig
	obsCfg        config.ObservabilityConfig
	pluginConfigs []config.PluginConfig
	radixRouter   *router.Router
	httpServer    *http.Server
}

func NewServer(serverCfg config.ServerConfig, authCfg config.AuthConfig, rateLimitCfg config.RateLimitConfig, obsCfg config.ObservabilityConfig, pluginConfigs []config.PluginConfig, mgr *plugin.Manager) *Server {
	r := gin.New()
	r.Use(gin.Recovery())
	if obsCfg.Tracing.Enabled {
		r.Use(middleware.TracingMiddlewares("microkernel-gateway")...)
	}
	if obsCfg.Metrics.Enabled {
		r.Use(middleware.MetricsMiddleware())
	}
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
		obsCfg:        obsCfg,
		pluginConfigs: pluginConfigs,
		radixRouter:   router.New(),
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

	// 动态注册插件路由到 Radix Router
	for _, pCfg := range s.pluginConfigs {
		pluginName := pCfg.Name
		for _, route := range pCfg.Routes {
			// 捕获闭包变量
			currentPluginName := pluginName

			// 注册到 Radix Router
			s.radixRouter.Handle(route.Method, route.Path, func(w http.ResponseWriter, req *http.Request, params router.Params) {
				s.forwardToPlugin(w, req, params, currentPluginName)
			})
		}
	}

	// Delegate all other routes to Radix Router
	s.engine.NoRoute(func(c *gin.Context) {
		s.radixRouter.ServeHTTP(c.Writer, c.Request)
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

func (s *Server) forwardToPlugin(w http.ResponseWriter, req *http.Request, params router.Params, pluginName string) {
	// 1. 获取插件实例
	pInstance, ok := s.pluginMgr.GetPlugin(pluginName)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "plugin not active"})
		return
	}

	// 2. 构建 RequestContext
	body, _ := io.ReadAll(req.Body)

	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	reqCtx := &pb.RequestContext{
		RequestId: req.Header.Get("X-Request-ID"),
		Method:    req.Method,
		Path:      req.URL.Path,
		Headers:   headers,
		Body:      body,
		Query:     make(map[string]string),
		Params:    make(map[string]string),
	}

	// 填充 Query
	for k, v := range req.URL.Query() {
		if len(v) > 0 {
			reqCtx.Query[k] = v[0]
		}
	}

	// 填充 Params (Radix Router Params)
	for _, param := range params {
		reqCtx.Params[param.Key] = param.Value
	}

	// 3. 调用 gRPC
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()

	resp, err := pInstance.HandleRequest(ctx, reqCtx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("rpc call failed: %v", err)})
		return
	}

	// 4. 返回响应
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	w.WriteHeader(int(resp.StatusCode))
	w.Write(resp.Body)
}

func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.serverConfig.Port),
		Handler: s.engine,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
