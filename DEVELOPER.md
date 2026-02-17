# 插件开发指南 (Developer Guide)

本文档旨在指导开发者如何为 Polyshift 微内核框架开发多语言插件。

## 核心协议 (Core Protocol)

插件与核心框架通过 gRPC 通信。协议定义在 `proto/plugin/plugin.proto`。

### 路由注册 (Route Registration)

插件在 `config.yaml` 中注册的路由支持以下模式：
- **静态路由**: `/api/hello`
- **参数路由**: `/api/users/:id` (通过 URL 路径参数传递)
- **通配符路由**: `/api/files/*filepath` (匹配后续所有路径)

核心框架使用 Radix Tree 进行高性能路由匹配，请求会被转发到插件的 `HandleRequest` 方法。

```protobuf
service PluginService {
    // 插件初始化
    rpc Init(InitRequest) returns (InitResponse);
    // 处理业务请求
    rpc HandleRequest(RequestContext) returns (ResponseContext);
    // 健康检查
    rpc HealthCheck(Empty) returns (HealthStatus);
    // 优雅停机
    rpc Shutdown(Empty) returns (Empty);
}
```

插件启动时，必须将 gRPC 服务监听地址通过标准输出 (stdout) 打印，格式为：
`|PLUGIN_ADDR|<address>|`
例如：`|PLUGIN_ADDR|127.0.0.1:50051|`

## 弹性与稳定性 (Resilience & Stability)

Polyshift 核心框架提供了多种机制来保障系统的稳定性，插件开发者应了解并配合这些机制。

### 1. 健康检查 (Health Check)
核心框架的 **Watchdog** 机制会定期调用插件的 `HealthCheck` 接口。
- **实现要求**: 插件应实现真实的健康检查逻辑（如检查数据库连接、内存使用等）。
- **返回值**: 如果返回 `SERVING`，核心认为插件正常；如果返回 `NOT_SERVING` 或调用超时，核心可能会重启插件进程。

### 2. 优雅停机 (Graceful Shutdown)
当 Watchdog 决定重启插件，或管理员手动卸载插件时，核心会调用 `Shutdown` 接口。
- **最佳实践**: 插件应在收到 `Shutdown` 请求后，停止接收新请求，完成当前处理中的请求，并释放资源（关闭文件、断开连接）。
- **超时**: 如果 `Shutdown` 超时（默认 5s），核心将强制杀掉插件进程 (`SIGKILL`)。

### 3. 熔断机制 (Circuit Breaker)
核心框架内置了熔断器。如果插件在短时间内频繁报错（如返回 500 或超时），熔断器会打开。
- **表现**: 熔断器打开期间，核心会直接拦截对该插件的请求，返回错误，而不会调用插件的 gRPC 接口。
- **恢复**: 经过冷却时间后，熔断器进入半开启状态，允许少量请求通过以探测插件是否恢复。

## 可观测性 (Observability)

核心框架已集成 OpenTelemetry 实现全链路追踪。

### 追踪上下文透传 (Trace Context Propagation)
核心框架在调用插件 gRPC 接口时，会将 Trace Context (`traceparent`) 注入到 gRPC Metadata 中。
- **Go 插件**: `sdk/go` 已自动集成了 `otelgrpc` 拦截器，开发者无需手动处理。如果插件内部需要调用其他微服务，建议从 `context` 中提取 Span。
- **其他语言插件**: 建议使用对应语言的 OpenTelemetry SDK，从 gRPC Metadata 中提取 `traceparent`，以保持链路的完整性。

## 开发指南 (Language Guides)

### Go 插件

使用 `sdk/go` 包简化开发。

```go
package main

import (
    "context"
    "fmt"
    "github.com/polyshift/microkernel/sdk/go/pkg/plugin"
    pb "github.com/polyshift/microkernel/proto/plugin"
)

func main() {
    server := plugin.NewServer()

    // 注册业务处理函数
    server.RegisterHandler(func(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
        // 获取配置
        greeting := server.GetConfig("greeting")
        if greeting == "" {
            greeting = "Hello"
        }

        return &pb.ResponseContext{
            StatusCode: 200,
            Body:       []byte(fmt.Sprintf("%s from Go!", greeting)),
            Headers:    map[string]string{"Content-Type": "text/plain"},
        }, nil
    })

    if err := server.Start(); err != nil {
        panic(err)
    }
}
```

### Python 插件

使用 `sdk/python` 包。

```python
from polyshift.plugin import Plugin, Server

class MyPlugin(Plugin):
    def handle_request(self, request):
        # 获取配置
        greeting = self.config.get("greeting", "Hello")
        
        return {
            "status_code": 200,
            "body": f"{greeting} from Python!".encode('utf-8'),
            "headers": {"Content-Type": "text/plain"}
        }

if __name__ == "__main__":
    Server(MyPlugin()).serve()
```

### Node.js 插件

使用 `sdk/js` 包。

```javascript
const { Plugin, Server } = require('./sdk/js');

class MyPlugin extends Plugin {
    async handleRequest(request) {
        // 获取配置
        const greeting = this.config['greeting'] || 'Hello';
        
        return {
            statusCode: 200,
            body: Buffer.from(`${greeting} from Node.js!`),
            headers: { "Content-Type": "text/plain" }
        };
    }
}

new Server(new MyPlugin()).start();
```

### Java 插件

使用 `sdk/java` 包。

```java
package com.example.hello;

import com.polyshift.plugin.Plugin;
import com.polyshift.plugin.PluginServer;
import com.polyshift.proto.PluginProto;
import java.util.Map;

public class HelloPlugin implements Plugin {
    private String greeting = "Hello";

    @Override
    public void init(Map<String, String> config) {
        if (config.containsKey("greeting")) {
            this.greeting = config.get("greeting");
        }
    }

    @Override
    public PluginProto.ResponseContext handleRequest(PluginProto.RequestContext request) {
        return PluginProto.ResponseContext.newBuilder()
                .setStatusCode(200)
                .putHeaders("Content-Type", "text/plain")
                .setBody(com.google.protobuf.ByteString.copyFromUtf8(greeting + " from Java!"))
                .build();
    }

    public static void main(String[] args) throws Exception {
        new PluginServer(new HelloPlugin()).start();
    }
}
```

### C++ 插件

使用 `sdk/cpp` 包。

```cpp
#include "polyshift/plugin.h"
#include <iostream>

using namespace polyshift;

class HelloPlugin : public Plugin {
public:
    void Init(const std::map<std::string, std::string>& config) override {
        if (config.find("greeting") != config.end()) {
            greeting_ = config.at("greeting");
        }
    }

    void HandleRequest(const ::plugin::RequestContext& request, ::plugin::ResponseContext* response) override {
        response->set_status_code(200);
        response->set_body(greeting_ + " from C++!");
        auto* map = response->mutable_headers();
        (*map)["Content-Type"] = "text/plain";
    }

private:
    std::string greeting_ = "Hello";
};

int main() {
    auto plugin = std::make_shared<HelloPlugin>();
    Server server(plugin);
    server.Start();
    return 0;
}
```
