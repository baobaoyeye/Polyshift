package plugin

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
	pb "github.com/polyshift/microkernel/proto/plugin"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PluginInstance struct {
	Config config.PluginConfig
	Client pb.PluginServiceClient
	Conn   *grpc.ClientConn
	cmd    *exec.Cmd
	addr   string
	cb     *gobreaker.CircuitBreaker

	LastHeartbeat   time.Time
	RestartCount    int
	NextRestartTime time.Time
	mu              sync.RWMutex
}

func NewPluginInstance(cfg config.PluginConfig, resilienceCfg config.ResilienceConfig) *PluginInstance {
	p := &PluginInstance{
		Config:        cfg,
		LastHeartbeat: time.Now(),
	}

	if resilienceCfg.CircuitBreaker.Enabled {
		st := gobreaker.Settings{
			Name:        cfg.Name,
			MaxRequests: resilienceCfg.CircuitBreaker.MaxRequests,
			Interval:    parseDuration(resilienceCfg.CircuitBreaker.Interval, 60*time.Second),
			Timeout:     parseDuration(resilienceCfg.CircuitBreaker.Timeout, 60*time.Second),
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= 3 && failureRatio >= resilienceCfg.CircuitBreaker.ReadyToTripRatio
			},
			OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
				slog.Info("Circuit Breaker state changed", "name", name, "from", from, "to", to)
			},
		}
		p.cb = gobreaker.NewCircuitBreaker(st)
	}

	return p
}

func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Warn("Failed to parse duration", "input", s, "error", err, "using_default", defaultVal)
		return defaultVal
	}
	return d
}

// HandleRequest wraps the gRPC call with circuit breaker logic
func (p *PluginInstance) HandleRequest(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.Client == nil {
		return nil, fmt.Errorf("plugin client not initialized")
	}

	if p.cb == nil {
		return p.Client.HandleRequest(ctx, req)
	}

	result, err := p.cb.Execute(func() (interface{}, error) {
		return p.Client.HandleRequest(ctx, req)
	})

	if err != nil {
		return nil, err
	}

	return result.(*pb.ResponseContext), nil
}

// Start 启动插件进程并建立连接
func (p *PluginInstance) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Prevent double start
	if p.Client != nil {
		return fmt.Errorf("plugin already started")
	}

	// 1. 启动子进程
	slog.Info("Starting plugin", "name", p.Config.Name, "runtime", p.Config.Runtime, "entry", p.Config.Entrypoint)

	// 根据 Runtime 选择命令
	var cmd *exec.Cmd
	if p.Config.Runtime == "go" {
		// Go 插件，假设是预编译好的二进制文件或 go run
		// 这里为了演示，我们假设 Entrypoint 是源码目录，使用 go run 运行
		// 实际上应该编译成二进制文件
		cmd = exec.Command("go", "run", p.Config.Entrypoint)
	} else if p.Config.Runtime == "python" {
		cmd = exec.Command("python3", p.Config.Entrypoint)
	} else if p.Config.Runtime == "nodejs" {
		cmd = exec.Command("node", p.Config.Entrypoint)
	} else if p.Config.Runtime == "java" {
		cmd = exec.Command("java", "-jar", p.Config.Entrypoint)
	} else if p.Config.Runtime == "cpp" || p.Config.Runtime == "binary" {
		// C++ or other binary plugins
		cmd = exec.Command(p.Config.Entrypoint)
	} else {
		// 默认为直接执行可执行文件
		cmd = exec.Command(p.Config.Entrypoint)
	}

	// 设置环境变量
	cmd.Env = os.Environ()

	// 获取 stdout 管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}
	// 将 stderr 重定向到父进程 stderr，方便调试
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start plugin process: %v", err)
	}
	p.cmd = cmd

	// 2. 读取监听地址
	// 插件启动后应输出 |PLUGIN_ADDR|<addr>|
	addrChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// slog.Debug("plugin stdout", "name", p.Config.Name, "line", line)
			if strings.HasPrefix(line, "|PLUGIN_ADDR|") && strings.HasSuffix(line, "|") {
				addr := strings.TrimSuffix(strings.TrimPrefix(line, "|PLUGIN_ADDR|"), "|")
				addrChan <- addr
				break
			}
		}
	}()

	select {
	case addr := <-addrChan:
		p.addr = addr
		slog.Info("Plugin listening", "name", p.Config.Name, "addr", addr)
	case <-time.After(5 * time.Second):
		// 启动超时
		_ = p.stopInternal()
		return fmt.Errorf("plugin start timeout")
	}

	// 3. 建立 gRPC 连接
	conn, err := grpc.NewClient(p.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		_ = p.stopInternal()
		return fmt.Errorf("failed to connect to plugin: %v", err)
	}
	p.Conn = conn
	p.Client = pb.NewPluginServiceClient(conn)

	// 4. 调用 Init
	configMap := make(map[string]string)
	configMap["version"] = p.Config.Version
	for k, v := range p.Config.Params {
		configMap[k] = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := p.Client.Init(ctx, &pb.InitRequest{
		Config: configMap,
	})
	if err != nil {
		_ = p.stopInternal()
		return fmt.Errorf("plugin init failed: %v", err)
	}
	if !resp.Success {
		_ = p.stopInternal()
		return fmt.Errorf("plugin init returned error: %s", resp.ErrorMessage)
	}

	slog.Info("Plugin initialized successfully", "name", p.Config.Name)
	return nil
}

func (p *PluginInstance) CheckHealth() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Client == nil {
		return false, fmt.Errorf("plugin client not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	resp, err := p.Client.HealthCheck(ctx, &pb.Empty{})
	if err != nil {
		return false, err
	}
	if resp.Status == pb.HealthStatus_SERVING {
		p.LastHeartbeat = time.Now()
		return true, nil
	}
	return false, nil
}

func (p *PluginInstance) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopInternal()
}

func (p *PluginInstance) stopInternal() error {
	// 1. 尝试调用 Shutdown RPC
	if p.Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := p.Client.Shutdown(ctx, &pb.Empty{})
		if err != nil {
			slog.Warn("Failed to gracefully shutdown plugin", "name", p.Config.Name, "error", err)
		}
		p.Client = nil // Reset client
	}

	// 2. 关闭 gRPC 连接
	if p.Conn != nil {
		p.Conn.Close()
		p.Conn = nil
	}

	// 3. 确保进程退出
	if p.cmd != nil && p.cmd.Process != nil {
		// 先尝试 Wait 看是否已经退出
		done := make(chan error, 1)
		go func() {
			_, err := p.cmd.Process.Wait()
			done <- err
		}()

		select {
		case <-done:
			// 进程已退出
		case <-time.After(100 * time.Millisecond):
			// 超时未退出，强制 Kill
			slog.Warn("Force killing plugin process", "name", p.Config.Name)
			_ = p.cmd.Process.Kill()
		}
		p.cmd = nil
	}
	return nil
}
