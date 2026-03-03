# 设计：企业级可观测性

## 1. 架构概览
本设计旨在引入统一的可观测性层 (Observability Layer)，负责初始化日志 (Logging)、追踪 (Tracing) 和指标 (Metrics) 组件，并将其注入到 Gateway 和 Plugin Manager 中。

### 1.1 组件
- **配置 (Config)**: 新增 `ObservabilityConfig` 结构体。
- **管理器 (Manager)**: `internal/core/observability` 包，负责初始化 OTel SDK 和全局 Logger。
- **中间件 (Middleware)**:
  - `TracingMiddleware`: 生成 HTTP Span，提取 Trace Context。
  - `LoggingMiddleware`: 记录请求日志，注入 TraceID 到 Logger Context。
  - `MetricsMiddleware`: 记录请求耗时和计数。
- **插件客户端 (Plugin Client)**: 使用 `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` 拦截器自动注入 Trace Context。

## 2. 数据模型

### 2.1 配置模式
```go
type ObservabilityConfig struct {
    Logging struct {
        Level  string `yaml:"level"`  // debug, info, warn, error
        Format string `yaml:"format"` // json, text
    } `yaml:"logging"`
    Tracing struct {
        Enabled      bool    `yaml:"enabled"`
        SamplingRate float64 `yaml:"sampling_rate"` // 0.0 - 1.0
        Exporter     string  `yaml:"exporter"`      // stdout, otlp
    } `yaml:"tracing"`
    Metrics struct {
        Enabled bool `yaml:"enabled"`
        Port    int  `yaml:"port"`
    } `yaml:"metrics"`
}
```

### 2.2 追踪上下文透传
Core 与 Plugin 之间通过 gRPC Metadata 传递 Trace Context。
- 键: `traceparent`
- 值: `00-<trace-id>-<span-id>-<flags>` (W3C 标准)

## 3. 实现细节

### 3.1 初始化流程
1. **加载配置**: 读取 `config.yaml`。
2. **初始化日志**: 根据配置初始化 `slog.Logger`，设置为 `slog.SetDefault()`。
3. **初始化追踪提供者**: 配置 OTel SDK，设置采样器 (Sampler) 和导出器 (Exporter)。
4. **初始化指标提供者**: 配置 Prometheus Exporter，启动 HTTP Server (`/metrics`)。
5. **启动网关**: 注入中间件。
6. **启动插件管理器**: 注入 gRPC 拦截器。

### 3.2 结构化日志集成
使用 `slog.With()` 创建带有 TraceID 的 Logger 实例，并存入 `context.Context` 中供后续使用。

```go
// 中间件逻辑示例
span := trace.SpanFromContext(ctx)
logger := slog.With("trace_id", span.SpanContext().TraceID().String())
ctx = context.WithValue(ctx, LoggerKey, logger)
```

### 3.3 插件埋点
在 `PluginInstance` 初始化 gRPC Client 时，添加 `otelgrpc.NewClientHandler()`。

## 4. 接口与 API

### 4.1 内部 API
- `observability.Init(cfg ObservabilityConfig) (shutdown func(), err error)`
- `middleware.Observability(tracer trace.Tracer, meter metric.Meter) gin.HandlerFunc`

### 4.2 外部 API
- `GET /metrics`: Prometheus Metrics Endpoint (独立端口或 Admin 端口)。
