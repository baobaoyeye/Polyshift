package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
	"github.com/polyshift/microkernel/internal/core/gateway"
	"github.com/polyshift/microkernel/internal/core/plugin"
	pb "github.com/polyshift/microkernel/proto/plugin"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// Mock Plugin Server
type mockPluginServer struct {
	pb.UnimplementedPluginServiceServer
	name string
}

func (s *mockPluginServer) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
	return &pb.InitResponse{Success: true}, nil
}

func (s *mockPluginServer) HealthCheck(ctx context.Context, req *pb.Empty) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{Status: pb.HealthStatus_SERVING}, nil
}

func (s *mockPluginServer) HandleRequest(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
	// Echo back the plugin name and path
	return &pb.ResponseContext{
		StatusCode: 200,
		Body:       []byte(fmt.Sprintf("Hello from %s, path: %s", s.name, req.Path)),
	}, nil
}

func (s *mockPluginServer) Shutdown(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func createMockPlugin(t *testing.T, name string) (string, func()) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	
	s := grpc.NewServer()
	pb.RegisterPluginServiceServer(s, &mockPluginServer{name: name})
	
	go s.Serve(lis)
	
	addr := lis.Addr().String()
	
	// Create script to output address
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "run.sh")
	// Use absolute path for script
	absScriptPath, _ := filepath.Abs(scriptPath)
	
	content := fmt.Sprintf("#!/bin/sh\necho \"|PLUGIN_ADDR|%s|\"\nsleep 10", addr)
	if err := os.WriteFile(absScriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	return absScriptPath, func() {
		s.Stop()
		lis.Close()
	}
}

func TestRouterIntegration(t *testing.T) {
	// 1. Setup Mock Plugins
	script1, cleanup1 := createMockPlugin(t, "plugin-1")
	defer cleanup1()

	script2, cleanup2 := createMockPlugin(t, "plugin-2")
	defer cleanup2()

	// 2. Setup Config
	pluginConfigs := []config.PluginConfig{
		{
			Name:       "plugin-1",
			Version:    "1.0.0",
			Runtime:    "binary",
			Entrypoint: script1,
			Routes: []config.RouteConfig{
				{Path: "/api/v1/users/:id", Method: "GET"},
				{Path: "/api/v1/static/*filepath", Method: "GET"},
			},
		},
		{
			Name:       "plugin-2",
			Version:    "1.0.0",
			Runtime:    "binary",
			Entrypoint: script2,
			Routes: []config.RouteConfig{
				{Path: "/api/v2/orders", Method: "POST"},
			},
		},
	}

	authCfg := config.AuthConfig{Enabled: false}
	rateLimitCfg := config.RateLimitConfig{Enabled: false}
	resilienceCfg := config.ResilienceConfig{
		Watchdog: config.WatchdogConfig{Enabled: false}, // Disable watchdog for this test
	}

	// 3. Start Manager
	mgr := plugin.NewManager(resilienceCfg)
	err := mgr.LoadPlugins(pluginConfigs)
	assert.NoError(t, err)

	// 4. Start Server on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	serverCfg := config.ServerConfig{Port: port}
	srv := gateway.NewServer(serverCfg, authCfg, rateLimitCfg, pluginConfigs, mgr)

	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	
	// Wait for server start
	time.Sleep(200 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 5. Test Cases
	tests := []struct {
		name         string
		method       string
		path         string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "Param Match Plugin 1",
			method:       "GET",
			path:         "/api/v1/users/123",
			expectedCode: 200,
			expectedBody: "Hello from plugin-1, path: /api/v1/users/123",
		},
		{
			name:         "Wildcard Match Plugin 1",
			method:       "GET",
			path:         "/api/v1/static/css/style.css",
			expectedCode: 200,
			expectedBody: "Hello from plugin-1, path: /api/v1/static/css/style.css",
		},
		{
			name:         "Exact Match Plugin 2",
			method:       "POST",
			path:         "/api/v2/orders",
			expectedCode: 200,
			expectedBody: "Hello from plugin-2, path: /api/v2/orders",
		},
		{
			name:         "No Match",
			method:       "GET",
			path:         "/api/v3/unknown",
			expectedCode: 404,
			expectedBody: "404 page not found",
		},
	}

	client := &http.Client{Timeout: 1 * time.Second}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, baseURL+tc.path, nil)
			assert.NoError(t, err)

			resp, err := client.Do(req)
			assert.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedCode, resp.StatusCode)

			if tc.expectedCode == 200 {
				body, _ := io.ReadAll(resp.Body)
				assert.Equal(t, tc.expectedBody, string(body))
			}
		})
	}
}
