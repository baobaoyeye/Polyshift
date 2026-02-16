# Polyshift Microkernel Service Framework

Polyshift 是一个基于 Go 语言开发的高性能微内核服务框架，通过 gRPC + Subprocess 实现多语言插件架构。它允许开发者使用 Go, Python, Node.js, Java 等多种语言编写业务插件，同时保持核心框架的高性能和稳定性。

## 特性 (Features)

- **多语言支持**: 原生支持 Go, Python, Node.js, Java 插件。
- **高性能核心**: 基于 Go 语言和 Gin 框架构建的高性能 HTTP/gRPC 网关。
- **进程隔离**: 插件运行在独立子进程中，核心框架与插件通过 gRPC 通信，确保隔离性。
- **热插拔 (Hot Swapping)**: 支持运行时动态加载、卸载、重载插件，无需重启核心服务。
- **动态路由**: 插件可动态注册 HTTP 路由。
- **中间件支持**: 内置 Request ID, Logger, API Key 鉴权, Rate Limit 等中间件。
- **配置管理**: 统一的 `config.yaml` 管理核心与插件配置。

## 快速开始 (Quick Start)

### 前置要求 (Prerequisites)

- Go 1.22+
- Python 3.8+ (可选)
- Node.js 16+ (可选)
- Java 8+ / Maven (可选)
- C++ 17+ / CMake / gRPC (可选)
- Protobuf Compiler (`protoc`)

### 运行 (Run)

1. **克隆项目**
   ```bash
   git clone https://github.com/polyshift/polyshift.git
   cd polyshift
   ```

2. **构建插件 SDK 和示例**
   ```bash
   # Go 插件
   go build -o examples/go-hello/go-hello examples/go-hello/main.go

   # Java 插件
   cd sdk/java && mvn clean install
   cd ../../examples/java-hello && mvn clean package
   cd ../../

   # C++ 插件
   cd examples/cpp-hello
   mkdir build && cd build
   cmake .. && make
   cd ../../../
   ```

3. **运行核心服务**
   ```bash
   go run cmd/core/main.go
   ```

4. **测试访问**
   ```bash
   curl http://localhost:8080/api/hello
   curl http://localhost:8080/api/python
   ```

## 配置 (Configuration)

修改 `config.yaml` 进行配置：

```yaml
server:
  port: 8080

auth:
  enabled: false
  api_key: "your-secret-key"

rate_limit:
  enabled: false
  qps: 100
  burst: 200

plugins:
  - name: "go-hello"
    version: "1.0.0"
    runtime: "go"
    entrypoint: "examples/go-hello/main.go" # 或编译后的二进制路径
    params:
      greeting: "Bonjour"
    routes:
      - path: "/api/hello"
        method: "GET"
        handler: "hello"
```

## 管理 API (Admin API)

- `GET /admin/plugins`: 列出所有插件
- `POST /admin/plugins`: 动态注册插件
- `PUT /admin/plugins/:name/reload`: 重载插件
- `DELETE /admin/plugins/:name`: 卸载插件
- `GET /admin/plugins/:name/health`: 检查插件健康状态
