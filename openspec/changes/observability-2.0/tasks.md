# 任务：企业级可观测性

## 第一阶段：配置与日志（基础）
- [x] **1.1 配置模式更新**
  - [x] 更新 `internal/core/config/config.go`，增加 `ObservabilityConfig`。
  - [x] 添加默认值（Info 级别，JSON 格式）。
  - [x] 更新 `config.yaml` 示例。
- [x] **1.2 结构化日志初始化**
  - [x] 创建 `internal/core/observability` 包。
  - [x] 实现 `InitLogger(cfg)`，使用 `log/slog`。
  - [x] 支持 JSON/Text 格式和级别控制。
  - [x] 将全局 `log.Printf` 替换为 `slog.Info/Error`。

## 第二阶段：分布式追踪（核心）
- [x] **2.1 OTel SDK 集成**
  - [x] 添加 OTel 依赖 (`go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`)。
  - [x] 在 `observability` 包中实现 `InitTracer(cfg)`。
  - [x] 配置 `stdout` 导出器用于开发环境。
- [x] **2.2 Gin 中间件**
  - [x] 创建 `internal/core/middleware/tracing.go`。
  - [x] 实现 `TracingMiddleware`：为每个请求启动 Root Span。
  - [x] 将 TraceID 注入响应头 (`X-Trace-ID`)。

## 第三阶段：插件上下文透传
- [x] **3.1 gRPC 客户端拦截器**
  - [x] 添加 `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`。
  - [x] 更新 `internal/core/plugin/instance.go` 使用 `otelgrpc.NewClientHandler`。
- [x] **3.2 插件服务端拦截器**
  - [x] 更新 `sdk/go/pkg/plugin/server.go` 使用 `otelgrpc.NewServerHandler`。
  - [x] 验证日志中的上下文透传（TraceID 关联）。

## 第四阶段：指标与验证
- [x] **4.1 Prometheus 导出器**
  - [x] 添加 `go.opentelemetry.io/otel/exporters/prometheus`。
  - [x] 实现 `InitMeter(cfg)` 并暴露 `/metrics` 端点。
  - [x] 埋点 HTTP 请求耗时直方图。
- [x] **4.2 验证**
  - [x] **集成测试**：验证日志和 Header 中存在 TraceID。
  - [x] **E2E 测试**：验证 `/metrics` 端点返回有效数据。
  - [x] **手动验证**：运行 `go-hello` 插件并检查完整追踪链路。
