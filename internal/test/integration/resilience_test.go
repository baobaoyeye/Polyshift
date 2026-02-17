package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
	"github.com/polyshift/microkernel/internal/core/gateway"
	"github.com/polyshift/microkernel/internal/core/plugin"
	pb "github.com/polyshift/microkernel/proto/plugin"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Controllable Mock Plugin Server
type flakyPluginServer struct {
	pb.UnimplementedPluginServiceServer
	mu           sync.Mutex
	fail         bool
	failureCount int
}

func (s *flakyPluginServer) setFail(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = fail
}

func (s *flakyPluginServer) getFailureCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failureCount
}

func (s *flakyPluginServer) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
	return &pb.InitResponse{Success: true}, nil
}

func (s *flakyPluginServer) HealthCheck(ctx context.Context, req *pb.Empty) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{Status: pb.HealthStatus_SERVING}, nil
}

func (s *flakyPluginServer) HandleRequest(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
	s.mu.Lock()
	fail := s.fail
	if fail {
		s.failureCount++
	}
	s.mu.Unlock()
	
	if fail {
		return nil, status.Error(codes.Internal, "simulated internal error")
	}
	
	return &pb.ResponseContext{
		StatusCode: 200,
		Body:       []byte("OK"),
	}, nil
}

func (s *flakyPluginServer) Shutdown(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func createFlakyMockPlugin(t *testing.T, name string) (string, *flakyPluginServer, func()) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	
	srv := &flakyPluginServer{}
	s := grpc.NewServer()
	pb.RegisterPluginServiceServer(s, srv)
	
	go s.Serve(lis)
	
	addr := lis.Addr().String()
	
	// Create script to output address
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "run_flaky.sh")
	absScriptPath, _ := filepath.Abs(scriptPath)
	
	content := fmt.Sprintf("#!/bin/sh\necho \"|PLUGIN_ADDR|%s|\"\nsleep 10", addr)
	if err := os.WriteFile(absScriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	return absScriptPath, srv, func() {
		s.Stop()
		lis.Close()
	}
}

func TestCircuitBreakerIntegration(t *testing.T) {
	// 1. Setup Flaky Plugin
	script, mockSrv, cleanup := createFlakyMockPlugin(t, "flaky-plugin")
	defer cleanup()

	// 2. Config with Circuit Breaker
	pluginConfigs := []config.PluginConfig{
		{
			Name:       "flaky-plugin",
			Version:    "1.0.0",
			Runtime:    "binary",
			Entrypoint: script,
			Routes: []config.RouteConfig{
				{Path: "/api/flaky", Method: "GET"},
			},
		},
	}

	resilienceCfg := config.ResilienceConfig{
		CircuitBreaker: config.CircuitBreakerConfig{
			Enabled:          true,
			MaxRequests:      1, // Half-open max requests
			Interval:         "10s",
			Timeout:          "200ms", // Short timeout for test
			ReadyToTripRatio: 0.5, // Trip if > 50% failure
		},
		Watchdog: config.WatchdogConfig{Enabled: false},
	}
	
	authCfg := config.AuthConfig{Enabled: false}
	rateLimitCfg := config.RateLimitConfig{Enabled: false}

	mgr := plugin.NewManager(resilienceCfg)
	err := mgr.LoadPlugins(pluginConfigs)
	assert.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	serverCfg := config.ServerConfig{Port: port}
	srv := gateway.NewServer(serverCfg, authCfg, rateLimitCfg, pluginConfigs, mgr)

	go func() {
		_ = srv.Start()
	}()
	time.Sleep(200 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/api/flaky", port)
	client := &http.Client{Timeout: 1 * time.Second}

	// 3. Send 3 successful requests to satisfy "Requests >= 3" condition for ReadyToTrip
	// But gobreaker ReadyToTrip counts failures/successes in the window.
	// If we send 3 successes, then 3 failures, total requests = 6, failures = 3, ratio = 0.5.
	// ReadyToTripRatio is 0.5. So it should trip.
	
	for i := 0; i < 3; i++ {
		resp, err := client.Get(baseURL)
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()
	}

	// 4. Trigger Failures
	mockSrv.setFail(true)
	
	// Send 3 failed requests to trip the breaker
	for i := 0; i < 3; i++ {
		resp, err := client.Get(baseURL)
		assert.NoError(t, err)
		// It might return 500 (Internal Server Error)
		assert.Equal(t, 500, resp.StatusCode)
		resp.Body.Close()
	}
	
	// 5. Verify Circuit Breaker Open
	// The next request should fail FAST (without reaching mock server)
	// We verify this by checking if mock server failure count increased.
	
	countBefore := mockSrv.getFailureCount()
	
	resp, err := client.Get(baseURL)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	
	// Check response body for "circuit breaker is open" if possible, but our handler returns JSON error
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "circuit breaker is open") {
		// It might be generic "rpc call failed: circuit breaker is open"
		t.Logf("Response body: %s", string(body))
	}
	
	countAfter := mockSrv.getFailureCount()
	
	// Assert that failureCount did NOT increase
	assert.Equal(t, countBefore, countAfter, "Circuit Breaker should be OPEN and block request")
	
	// 6. Verify Half-Open
	// Wait for Timeout (200ms)
	time.Sleep(300 * time.Millisecond)
	
	// Next request should go through (Half-Open allows 1 request)
	// Let's make it succeed
	mockSrv.setFail(false)
	
	resp, err = client.Get(baseURL)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()
	
	// Now Circuit Breaker should be Closed
	// Send another success
	resp, err = client.Get(baseURL)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()
}
