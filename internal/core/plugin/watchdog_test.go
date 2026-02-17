package plugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
	pb "github.com/polyshift/microkernel/proto/plugin"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

type mockPluginServer struct {
	pb.UnimplementedPluginServiceServer
	healthStatus pb.HealthStatus_Status
	initSuccess  bool
}

func (s *mockPluginServer) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
	if !s.initSuccess {
		return &pb.InitResponse{Success: false, ErrorMessage: "init failed"}, nil
	}
	return &pb.InitResponse{Success: true}, nil
}

func (s *mockPluginServer) HealthCheck(ctx context.Context, req *pb.Empty) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{Status: s.healthStatus}, nil
}

func (s *mockPluginServer) HandleRequest(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
	return &pb.ResponseContext{StatusCode: 200, Body: []byte("ok")}, nil
}

func (s *mockPluginServer) Shutdown(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func TestWatchdogRestart(t *testing.T) {
	// 1. Start mock gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	mockSrv := &mockPluginServer{
		healthStatus: pb.HealthStatus_SERVING,
		initSuccess:  true,
	}
	pb.RegisterPluginServiceServer(s, mockSrv)
	go s.Serve(lis)
	defer s.Stop()

	addr := lis.Addr().String()

	// 2. Create Manager with Watchdog config
	resilienceCfg := config.ResilienceConfig{
		Watchdog: config.WatchdogConfig{
			Enabled:    true,
			Interval:   "100ms",
			MaxRetries: 3,
			BaseDelay:  "10ms",
			MaxDelay:   "100ms",
		},
	}
	mgr := NewManager(resilienceCfg)

	// 3. Create Plugin Config using a script that prints the address
	scriptContent := fmt.Sprintf("#!/bin/sh\necho \"|PLUGIN_ADDR|%s|\"\nsleep 10", addr)
	scriptFile := "mock_plugin.sh"
	err = os.WriteFile(scriptFile, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}
	defer os.Remove(scriptFile)

	pluginCfg := config.PluginConfig{
		Name:       "test-plugin",
		Runtime:    "binary",
		Entrypoint: "./" + scriptFile,
	}

	// 4. Start Plugin manually first
	err = mgr.StartPlugin(pluginCfg)
	assert.NoError(t, err)

	// 5. Start Watchdog
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.StartWatchdog(ctx)

	// 6. Verify healthy initially
	time.Sleep(200 * time.Millisecond)
	p, ok := mgr.GetPlugin("test-plugin")
	assert.True(t, ok)
	p.mu.RLock()
	count := p.RestartCount
	p.mu.RUnlock()
	assert.Equal(t, 0, count)

	// 7. Make unhealthy
	mockSrv.healthStatus = pb.HealthStatus_NOT_SERVING
	
	// Wait for watchdog to detect and restart
	time.Sleep(300 * time.Millisecond)
	
	// Should have restarted
	p.mu.RLock()
	count = p.RestartCount
	p.mu.RUnlock()
	// Count might be 0 if it recovered immediately (Start -> Init -> Success -> CheckHealth -> Healthy?)
	// But MockServer is STILL NOT_SERVING.
	// So CheckHealth will fail again.
	// So Count should be > 0.
	assert.Greater(t, count, 0, "Should have restarted (count > 0)")

	// 8. Make healthy again
	mockSrv.healthStatus = pb.HealthStatus_SERVING
	
	// Wait for recovery
	time.Sleep(300 * time.Millisecond)
	
	p.mu.RLock()
	count = p.RestartCount
	p.mu.RUnlock()
	assert.Equal(t, 0, count, "Should reset count after success")
}
