# 技术设计 (Design): 微内核多语言服务框架

## 1. 架构概览 (Architecture Overview)
本方案采用 **微内核 (Microkernel) + 进程外插件 (Out-of-Process Plugins)** 架构。
核心层 (Core) 负责生命周期管理、配置加载、请求路由和通用中间件处理。
插件层 (Plugins) 运行在独立的进程中，通过 gRPC 与 Core 进行通信。

*   **Core (Golang)**:
    *   **Gateway**: HTTP/gRPC 入口，负责协议解析和转换。
    *   **PluginManager**: 管理插件进程的生命周期（Start, Stop, Reload）。
    *   **Router**: 维护 URL/Service 到插件实例的映射表。
    *   **MiddlewareChain**: 执行鉴权、限流、日志等通用逻辑。
*   **Plugins (Polyglot)**:
    *   **SDK**: 各语言提供 SDK，封装 gRPC Server 启动细节，暴露简单的 `Handle` 接口。
    *   **Business Logic**: 开发者编写的具体业务代码。
*   **Communication**:
    *   **Protocol**: gRPC (Protobuf)
    *   **Transport**: Unix Domain Socket (Linux/macOS) 或 TCP Loopback (Windows)。

### 架构图 (Architecture Diagram)
```mermaid
graph TD
    Client[Client (HTTP/gRPC)] --> Gateway
    subgraph Core [Microkernel Core]
        Gateway --> Middleware[Middleware Chain]
        Middleware --> Router
        Router --> PluginClient[gRPC Client Pool]
        PluginManager --> PluginProcess[Plugin Process Management]
    end
    subgraph Plugins [Plugin Processes]
        PluginClient -- gRPC/UDS --> P1[Java Plugin]
        PluginClient -- gRPC/UDS --> P2[Python Plugin]
        PluginClient -- gRPC/UDS --> P3[Go Plugin]
        PluginClient -- gRPC/UDS --> P4[Node.js Plugin]
    end
```

## 2. 数据模型 (Data Models)

### 2.1 插件定义 (Plugin Definition)
```yaml
# plugin.yaml
name: "user-service"
version: "1.0.0"
runtime: "python" # java, go, node, cpp
entrypoint: "main.py" # 或编译后的二进制文件路径
routes:
  - path: "/api/users"
    method: "GET"
    handler: "list_users"
  - path: "/api/users/:id"
    method: "GET"
    handler: "get_user"
```

### 2.2 内部请求上下文 (RequestContext)
```protobuf
message RequestContext {
    string request_id = 1;
    string method = 2;
    string path = 3;
    map<string, string> headers = 4;
    bytes body = 5;
    map<string, string> params = 6; // Path parameters
    map<string, string> query = 7;  // Query parameters
    // User info
    string user_id = 8;
    string tenant_id = 9;
}
```

## 3. 接口设计 (API Design)

### 3.1 核心插件协议 (Core-Plugin Protocol)
定义在 `proto/plugin.proto` 中：

```protobuf
syntax = "proto3";
package plugin;

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

message ResponseContext {
    int32 status_code = 1;
    map<string, string> headers = 2;
    bytes body = 3;
    string error_message = 4;
}
```

### 3.2 外部接口 (External API)
*   **HTTP**: `POST /api/v1/{plugin_name}/{action}` 或根据 `plugin.yaml` 中的路由规则自动映射。
*   **gRPC**: 通用的 Gateway Service，转发 payload。

## 4. 核心流程 (Core Flows)

### 4.1 插件加载流程
1.  Core 启动，扫描 `plugins/` 目录。
2.  读取每个插件的 `plugin.yaml`。
3.  PluginManager 根据 `runtime` 启动对应的子进程（例如 `python main.py`）。
4.  子进程启动 gRPC Server，监听随机端口或 UDS。
5.  子进程通过 stdout 输出监听地址（握手）。
6.  Core 读取地址，建立 gRPC 连接。
7.  Core 调用 `PluginService.Init`。
8.  Core 将插件路由注册到 Router。
9.  插件状态变为 `Ready`。

### 4.2 请求处理流程
1.  Client 发送 HTTP 请求 `GET /api/users/123`。
2.  Gateway 接收请求，生成 `RequestContext` (RequestID, Headers, etc.)。
3.  MiddlewareChain 执行（如鉴权）。
4.  Router 匹配路径 `/api/users/:id`，找到对应的插件 `user-service`。
5.  Core 通过 gRPC 调用插件的 `HandleRequest`，传入 `RequestContext`。
6.  插件处理业务逻辑，返回 `ResponseContext`。
7.  Gateway 将 `ResponseContext` 转换为 HTTP Response 返回给 Client。

## 5. 实现细节 (Implementation Details)

### 5.1 多语言 SDK
为了简化插件开发，需要为每种支持的语言提供 SDK：
*   **Java SDK**: 基于 gRPC-Java，封装 Server 启动和 protobuf 编解码。
*   **Python SDK**: 基于 gRPC-Python。
*   **Go SDK**: 基于 gRPC-Go。
*   **Node.js SDK**: 基于 @grpc/grpc-js。
*   **C++ SDK**: 基于 gRPC-C++。

### 5.2 热插拔 (Hot Reload)
1.  用户触发 Reload 指令（CLI 或 API）。
2.  Core 启动插件的新版本进程。
3.  等待新进程 `Ready`。
4.  Router 原子切换，将新请求导向新进程。
5.  旧进程进入 `Draining` 状态，等待当前处理中的请求完成（设置超时）。
6.  Core 发送 `Shutdown` 指令给旧进程，或直接 Kill。

## 6. 替代方案 (Alternatives Considered)
*   **WebAssembly (Wasm)**:
    *   *优点*: 隔离性好，启动快，轻量级。
    *   *缺点*: 多语言支持（尤其是带 GC 的语言如 Java/Python）目前尚不成熟，生态库支持有限，调试困难。
    *   *结论*: 暂不作为首选，未来可作为一种 Runtime 支持。
*   **Shared Library (JNI/CGO)**:
    *   *优点*: 调用性能最高。
    *   *缺点*: 只要一个插件崩溃就会导致整个 Core 崩溃（Segfault），隔离性差，多语言支持复杂。
    *   *结论*: 否决。
