package benchmark

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/polyshift/microkernel/internal/core/router"
	pb "github.com/polyshift/microkernel/proto/plugin"
)

// MockPlugin for benchmarking
type mockPlugin struct{}

func (m *mockPlugin) ID() string   { return "bench-plugin" }
func (m *mockPlugin) Name() string { return "bench-plugin" }
func (m *mockPlugin) HandleRequest(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
	return &pb.ResponseContext{
		StatusCode: 200,
		Body:       []byte("OK"),
	}, nil
}

// Add missing methods to satisfy Plugin interface
func (m *mockPlugin) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
	return &pb.InitResponse{Success: true}, nil
}
func (m *mockPlugin) HealthCheck(ctx context.Context, req *pb.Empty) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{Status: pb.HealthStatus_SERVING}, nil
}
func (m *mockPlugin) Shutdown(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

// Removed undefined State methods as they are not needed for this benchmark
// and the interface doesn't seem to enforce them in the current context
// or they are internal details not exposed in the public interface being mocked.

func BenchmarkRouterMatch_1000Routes(b *testing.B) {
	r := router.New()

	// Add 1000 routes
	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("/api/v1/resource/%d", i)
		r.GET(path, func(w http.ResponseWriter, req *http.Request, params router.Params) {})
	}

	// Benchmark matching a route in the middle
	target := "/api/v1/resource/500"
	req, _ := http.NewRequest("GET", target, nil)
	w := &mockResponseWriter{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterMatch_Wildcard(b *testing.B) {
	r := router.New()
	r.GET("/api/v1/users/:id", func(w http.ResponseWriter, req *http.Request, params router.Params) {})
	r.GET("/api/v1/files/*filepath", func(w http.ResponseWriter, req *http.Request, params router.Params) {})

	b.Run("ParamMatch", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/api/v1/users/123", nil)
		w := &mockResponseWriter{}
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("WildcardMatch", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/api/v1/files/images/logo.png", nil)
		w := &mockResponseWriter{}
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})
}

type mockResponseWriter struct{}

func (m *mockResponseWriter) Header() http.Header        { return http.Header{} }
func (m *mockResponseWriter) Write([]byte) (int, error)  { return 0, nil }
func (m *mockResponseWriter) WriteHeader(statusCode int) {}
