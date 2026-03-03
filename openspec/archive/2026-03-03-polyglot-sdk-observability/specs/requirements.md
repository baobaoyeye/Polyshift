# 需求规范 (Requirements): 多语言 SDK 可观测性自动注入

## 1. 用户故事 (User Stories)
| ID | As a (角色) | I want to (功能) | So that (价值) | Priority |
| :--- | :--- | :--- | :--- | :--- |
| US-01 | 插件开发者 (Python) | 在编写 Python 插件时，不需要手动初始化 Tracer | 能够专注于业务逻辑，同时获得全链路追踪能力 | P0 |
| US-02 | 插件开发者 (Java) | 在编写 Java 插件时，不需要修改 `pom.xml` 或 `build.gradle` 即可获得 OTel 支持 | 简化依赖管理，降低接入成本 | P0 |
| US-03 | 插件开发者 (Node.js) | 在编写 Node.js 插件时，能够自动捕获 HTTP/gRPC 请求的 Trace Context | 实现前后端全链路打通 | P0 |
| US-04 | 插件开发者 (C++) | 在编写高性能 C++ 插件时，也能获得基本的 Trace 支持 | 消除性能敏感模块的可观测性盲区 | P1 |
| US-05 | 运维人员 | 通过环境变量配置所有插件的采样率和 Exporter 地址 | 统一管理整个集群的可观测性策略 | P1 |

## 2. 功能性需求 (Functional Requirements)

### 2.1 Python SDK 增强
*   **依赖集成**: 引入 `opentelemetry-api`, `opentelemetry-sdk`, `opentelemetry-instrumentation-grpc`。
*   **自动注入**: 在 `PluginServer` 启动时，自动添加 OTel Server Interceptor。
*   **上下文提取**: 从 gRPC Metadata 中提取 `traceparent` Header，并注入到 `context` 中。

### 2.2 Java SDK 增强
*   **依赖集成**: 引入 `io.opentelemetry:opentelemetry-api`, `io.opentelemetry:opentelemetry-sdk`, `io.opentelemetry.instrumentation:opentelemetry-grpc-1.6`。
*   **自动注入**: 在 `ServerBuilder` 构建过程中，添加 OTel Server Interceptor。
*   **BOM 管理**: 使用 `opentelemetry-bom` 统一管理版本，避免冲突。

### 2.3 Node.js SDK 增强
*   **依赖集成**: 引入 `@opentelemetry/api`, `@opentelemetry/sdk-node`, `@opentelemetry/instrumentation-grpc`。
*   **自动注入**: 在 `PluginServer` 构造函数中初始化 NodeSDK，并开启 gRPC 自动插桩。
*   **异步上下文**: 确保 Trace Context 在 `async/await` 调用链中正确传递。

### 2.4 C++ SDK 增强
*   **依赖集成**: 引入 `opentelemetry-cpp` 和 `opentelemetry-cpp-contrib` (gRPC interceptors)。
*   **构建支持**: 更新 `CMakeLists.txt`，自动查找并链接 OTel 库。
*   **自动注入**: 在 `ServerBuilder` 中添加 OTel Server Interceptor。

### 2.5 配置管理
*   **环境变量**: SDK 应读取以下环境变量来配置 OTel：
    *   `OTEL_TRACES_EXPORTER`: 默认为 `stdout` (开发环境) 或 `otlp` (生产环境)。
    *   `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP Collector 地址。
    *   `OTEL_SERVICE_NAME`: 服务名称，默认为插件名称。

## 3. 非功能性需求 (Non-Functional Requirements)
*   **性能 (Performance)**:
    *   引入 OTel 后，插件处理请求的平均延迟增加不超过 2ms。
    *   在采样率 (Sampling Rate) 为 0 (关闭) 时，性能损耗应接近于零。
*   **兼容性 (Compatibility)**:
    *   Python: 支持 Python 3.10+。
    *   Java: 支持 Java 11+。
    *   Node.js: 支持 Node.js 18+。
    *   C++: 支持 C++17 标准。
*   **稳定性 (Stability)**:
    *   OTel SDK 初始化失败 (如 Exporter 连接超时) 不应导致插件进程崩溃，应降级为无追踪模式。

## 4. 验收标准 (Acceptance Criteria)
*   [ ] AC-01: 运行 `examples/py-hello`，发起 HTTP 请求，Core 日志和 Python 插件日志中均包含相同的 Trace ID。
*   [ ] AC-02: 运行 `examples/java-hello`，发起请求，能够通过 OTLP Exporter 将 Trace 数据发送到 Jaeger/Zipkin。
*   [ ] AC-03: 运行 `examples/js-hello`，验证 Trace Context 在异步回调中未丢失。
*   [ ] AC-04: 运行 `examples/cpp-hello`，验证 C++ 插件能正确解析 Core 传递的 Trace Context。
*   [ ] AC-05: 设置环境变量 `OTEL_TRACES_SAMPLER=always_off`，验证插件不生成任何 Trace 数据。
