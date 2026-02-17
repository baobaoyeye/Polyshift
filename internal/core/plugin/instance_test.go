package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
	pb "github.com/polyshift/microkernel/proto/plugin"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
)

// MockClient implements pb.PluginServiceClient
type MockClient struct {
	shouldFail bool
}

func (m *MockClient) HandleRequest(ctx context.Context, in *pb.RequestContext, opts ...grpc.CallOption) (*pb.ResponseContext, error) {
	if m.shouldFail {
		return nil, errors.New("mock failure")
	}
	return &pb.ResponseContext{StatusCode: 200}, nil
}

func (m *MockClient) Init(ctx context.Context, in *pb.InitRequest, opts ...grpc.CallOption) (*pb.InitResponse, error) {
	return &pb.InitResponse{Success: true}, nil
}

func (m *MockClient) HealthCheck(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{Status: pb.HealthStatus_SERVING}, nil
}

func (m *MockClient) Shutdown(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func TestCircuitBreaker(t *testing.T) {
	mockClient := &MockClient{}
	
	// Config with low thresholds for testing
	resCfg := config.ResilienceConfig{
		CircuitBreaker: config.CircuitBreakerConfig{
			Enabled:          true,
			MaxRequests:      1,
			Interval:         "10s",
			Timeout:          "100ms", // Short timeout to test half-open
			ReadyToTripRatio: 0.5,
		},
	}
	
	pluginCfg := config.PluginConfig{Name: "test-plugin"}
	
	instance := NewPluginInstance(pluginCfg, resCfg)
	instance.Client = mockClient
	// We don't need instance.Conn for this test as we inject Client directly
	
	ctx := context.Background()
	req := &pb.RequestContext{}

	// 1. Success case
	_, err := instance.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if instance.cb.State() != gobreaker.StateClosed {
		t.Errorf("Expected StateClosed, got %v", instance.cb.State())
	}

	// 2. Trigger Failure to Trip
	mockClient.shouldFail = true
	
	// Fail 3 times (ReadyToTrip: requests >= 3)
	for i := 0; i < 3; i++ {
		instance.HandleRequest(ctx, req)
	}

	// Should be open now
	if instance.cb.State() != gobreaker.StateOpen {
		t.Errorf("Expected StateOpen, got %v", instance.cb.State())
	}

	// 3. Fail Fast (Open State)
	_, err = instance.HandleRequest(ctx, req)
	if err == nil || err.Error() != "circuit breaker is open" {
		t.Errorf("Expected 'circuit breaker is open', got %v", err)
	}

	// 4. Half-Open Test
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	
	// Next request should be allowed (Half-Open probe)
	// Let's make it succeed
	mockClient.shouldFail = false
	
	_, err = instance.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Expected success in Half-Open, got %v", err)
	}
	
	// Should be closed again
	if instance.cb.State() != gobreaker.StateClosed {
		t.Errorf("Expected StateClosed after success, got %v", instance.cb.State())
	}
}
