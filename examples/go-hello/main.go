package main

import (
	"context"
	"fmt"
	"log"

	"github.com/polyshift/microkernel/proto/plugin"
	sdk "github.com/polyshift/microkernel/sdk/go/pkg/plugin"
)

func main() {
	// 创建一个新的 Plugin Server
	server := sdk.NewServer()

	// 注册 Handler 处理函数
	server.RegisterHandler(func(ctx context.Context, req *plugin.RequestContext) (*plugin.ResponseContext, error) {
		log.Printf("Received request for path: %s", req.Path)

		// 简单的业务逻辑：根据 path 返回不同的结果
		var message string
		if req.Path == "/api/hello" {
			greeting := server.GetConfig("greeting")
			if greeting == "" {
				greeting = "Hello"
			}
			message = fmt.Sprintf("%s from Go Plugin!", greeting)
		} else {
			message = fmt.Sprintf("Echo: %s", req.Path)
		}

		return &plugin.ResponseContext{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: []byte(fmt.Sprintf(`{"message": "%s"}`, message)),
		}, nil
	})

	// 启动服务
	if err := server.Start(); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
