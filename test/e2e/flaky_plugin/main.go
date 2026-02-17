package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	pb "github.com/polyshift/microkernel/proto/plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedPluginServiceServer
	mu           sync.Mutex
	shouldFail   bool
	failureCount int
}

func (s *server) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
	return &pb.InitResponse{Success: true}, nil
}

func (s *server) HealthCheck(ctx context.Context, req *pb.Empty) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{Status: pb.HealthStatus_SERVING}, nil
}

func (s *server) HandleRequest(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
	// Check for a special header to toggle failure mode dynamically
	// Headers are passed as is from http.Request.Header, so they are canonical (e.g. X-Simulate-Failure)

	// Stateless failure check
	if val, ok := req.Headers["X-Fail-Once"]; ok && val == "true" {
		return nil, status.Error(codes.Internal, "simulated internal error (stateless)")
	}

	if val, ok := req.Headers["X-Simulate-Failure"]; ok && val == "true" {
		s.mu.Lock()
		s.shouldFail = true
		s.mu.Unlock()
	}

	if val, ok := req.Headers["X-Simulate-Failure"]; ok && val == "false" {
		s.mu.Lock()
		s.shouldFail = false
		s.mu.Unlock()
	}

	s.mu.Lock()
	shouldFail := s.shouldFail
	if shouldFail {
		s.failureCount++
	}
	s.mu.Unlock()

	if shouldFail {
		return nil, status.Error(codes.Internal, "simulated internal error")
	}

	return &pb.ResponseContext{
		StatusCode: 200,
		Body:       []byte("OK"),
	}, nil
}

func (s *server) Shutdown(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func main() {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterPluginServiceServer(s, &server{})

	// Print address to stdout for PluginManager
	fmt.Printf("|PLUGIN_ADDR|%s|\n", lis.Addr().String())
	// Flush stdout to ensure manager sees it immediately
	os.Stdout.Sync()

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Wait for interrupt
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	s.GracefulStop()
}
