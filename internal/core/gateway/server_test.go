package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/polyshift/microkernel/internal/core/config"
	"github.com/polyshift/microkernel/internal/core/plugin"
)

func TestServerRouting(t *testing.T) {
	// Setup config
	serverCfg := config.ServerConfig{Port: 8080}
	authCfg := config.AuthConfig{Enabled: false}
	rateLimitCfg := config.RateLimitConfig{Enabled: false}

	pluginConfigs := []config.PluginConfig{
		{
			Name: "test-plugin",
			Routes: []config.RouteConfig{
				{Method: "GET", Path: "/api/test"},
				{Method: "POST", Path: "/api/data/:id"},
				{Method: "GET", Path: "/files/*filepath"},
			},
		},
	}

	mgr := plugin.NewManager(config.ResilienceConfig{})

	server := NewServer(serverCfg, authCfg, rateLimitCfg, pluginConfigs, mgr)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string // partial match
	}{
		{
			name:       "Health Check",
			method:     "GET",
			path:       "/health",
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ok"`,
		},
		{
			name:       "Static Route (Plugin not active)",
			method:     "GET",
			path:       "/api/test",
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"error":"plugin not active"`,
		},
		{
			name:       "Param Route (Plugin not active)",
			method:     "POST",
			path:       "/api/data/123",
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"error":"plugin not active"`,
		},
		{
			name:       "Wildcard Route (Plugin not active)",
			method:     "GET",
			path:       "/files/css/style.css",
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"error":"plugin not active"`,
		},
		{
			name:       "Not Found",
			method:     "GET",
			path:       "/api/unknown",
			wantStatus: http.StatusNotFound,
			wantBody:   "404 page not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, nil)

			server.engine.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			body := strings.TrimSpace(w.Body.String())
			if !strings.Contains(body, tt.wantBody) {
				t.Errorf("expected body to contain %q, got %q", tt.wantBody, body)
			}
		})
	}
}
