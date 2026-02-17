package plugin

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/polyshift/microkernel/proto/plugin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// HandlerFunc 是处理业务请求的函数类型
type HandlerFunc func(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error)

type Server struct {
	pb.UnimplementedPluginServiceServer
	handler HandlerFunc
	server  *grpc.Server
	config  map[string]string
}

func NewServer() *Server {
	return &Server{
		server: grpc.NewServer(
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		),
		config: make(map[string]string),
	}
}

// RegisterHandler 注册业务处理函数
func (s *Server) RegisterHandler(h HandlerFunc) {
	s.handler = h
}

// GetConfig 获取配置
func (s *Server) GetConfig(key string) string {
	return s.config[key]
}

// Init 实现插件初始化逻辑
func (s *Server) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
	log.Println("Plugin initialized with config:", req.Config)
	s.config = req.Config
	return &pb.InitResponse{Success: true}, nil
}

// HandleRequest 处理核心转发过来的请求
func (s *Server) HandleRequest(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
	if s.handler == nil {
		return &pb.ResponseContext{
			StatusCode:   500,
			ErrorMessage: "Handler not registered",
		}, nil
	}
	return s.handler(ctx, req)
}

// HealthCheck 健康检查
func (s *Server) HealthCheck(ctx context.Context, req *pb.Empty) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{
		Status:  pb.HealthStatus_SERVING,
		Message: "OK",
	}, nil
}

// Shutdown 优雅停机
func (s *Server) Shutdown(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	go func() {
		s.server.GracefulStop()
		os.Exit(0)
	}()
	return &pb.Empty{}, nil
}

// Start 启动插件服务
func (s *Server) Start() error {
	// 监听随机端口
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	pb.RegisterPluginServiceServer(s.server, s)

	// 重要：将监听地址输出到 stdout，格式为 |PLUGIN_ADDR|<addr>|
	// Core 会解析这个输出来建立连接
	addr := lis.Addr().String()
	fmt.Printf("|PLUGIN_ADDR|%s|\n", addr)
	// 确保 stdout 立即刷新
	os.Stdout.Sync()

	log.Printf("Plugin server listening on %s", addr)
	return s.server.Serve(lis)
}
