package plugin

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
	pb "github.com/polyshift/microkernel/proto/plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PluginInstance struct {
	Config config.PluginConfig
	Client pb.PluginServiceClient
	Conn   *grpc.ClientConn
	cmd    *exec.Cmd
	addr   string
}

func NewPluginInstance(cfg config.PluginConfig) *PluginInstance {
	return &PluginInstance{
		Config: cfg,
	}
}

// Start 启动插件进程并建立连接
func (p *PluginInstance) Start() error {
	// 1. 启动子进程
	log.Printf("Starting plugin: %s (runtime=%s, entry=%s)", p.Config.Name, p.Config.Runtime, p.Config.Entrypoint)

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
			// log.Printf("[%s] stdout: %s", p.Config.Name, line)
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
		log.Printf("Plugin %s listening on %s", p.Config.Name, addr)
	case <-time.After(5 * time.Second):
		// 启动超时
		_ = p.Stop()
		return fmt.Errorf("plugin start timeout")
	}

	// 3. 建立 gRPC 连接
	conn, err := grpc.NewClient(p.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = p.Stop()
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
		_ = p.Stop()
		return fmt.Errorf("plugin init failed: %v", err)
	}
	if !resp.Success {
		_ = p.Stop()
		return fmt.Errorf("plugin init returned error: %s", resp.ErrorMessage)
	}

	log.Printf("Plugin %s initialized successfully", p.Config.Name)
	return nil
}

func (p *PluginInstance) CheckHealth() (bool, error) {
	if p.Client == nil {
		return false, fmt.Errorf("plugin client not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	resp, err := p.Client.HealthCheck(ctx, &pb.Empty{})
	if err != nil {
		return false, err
	}
	return resp.Status == pb.HealthStatus_SERVING, nil
}

func (p *PluginInstance) Stop() error {
	// 1. 尝试调用 Shutdown RPC
	if p.Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := p.Client.Shutdown(ctx, &pb.Empty{})
		if err != nil {
			log.Printf("Failed to gracefully shutdown plugin %s: %v", p.Config.Name, err)
		}
	}

	// 2. 关闭 gRPC 连接
	if p.Conn != nil {
		p.Conn.Close()
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
			return nil
		case <-time.After(100 * time.Millisecond):
			// 超时未退出，强制 Kill
			log.Printf("Force killing plugin process %s", p.Config.Name)
			return p.cmd.Process.Kill()
		}
	}
	return nil
}
