# 需求：企业级可观测性

## 1. 用户故事
- **US-01**: 作为运维人员，我希望能够在 `config.yaml` 中配置日志级别和采样率，以便在不同环境（Dev/Prod）下灵活调整。
- **US-02**: 作为开发者，我希望每一条日志都包含 `trace_id`，以便我能够根据一个 ID 串联起整个请求的处理链路。
- **US-03**: 作为开发者，我希望能够看到 HTTP 请求经过 Core 到达 Plugin 再返回的完整调用链耗时，以便快速定位性能瓶颈。
- **US-04**: 作为 SRE，我希望通过 `/metrics` 接口获取 Prometheus 格式的监控数据，以便配置 Grafana 面板和报警。

## 2. 功能需求

### 2.1 配置管理
- 新增 `observability` 配置段：
  ```yaml
  observability:
    logging:
      level: "info" # debug, info, warn, error
      format: "json" # json, text
    tracing:
      enabled: true
      sampling_rate: 1.0 # 0.0 - 1.0
      exporter: "stdout" # stdout, otlp (未来支持)
    metrics:
      enabled: true
      port: 9090 # 独立的 metrics 端口
  ```

### 2.2 结构化日志
- 使用 `log/slog` 替换所有 `log.Printf`。
- 日志必须包含以下标准字段：
  - `time`: RFC3339 格式时间
  - `level`: 日志级别
  - `msg`: 消息内容
  - `trace_id`: (可选) 追踪 ID
  - `span_id`: (可选) 跨度 ID
  - `component`: (例如 "core", "plugin-manager", "gateway")

### 2.3 分布式追踪
- **Core Gateway**: 为每个入站 HTTP 请求生成 Root Span。
- **Plugin Call**: 在调用插件 gRPC 接口前创建 Child Span，并将 Context 注入 Metadata。
- **Propagation**: 支持 W3C Trace Context (`traceparent`) 标准。

### 2.4 指标监控
- 集成 OpenTelemetry Prometheus Exporter。
- 暴露标准指标：
  - `http_server_duration_milliseconds`: 直方图 (Histogram)
  - `plugin_request_duration_milliseconds`: 直方图 (Histogram)
  - `plugin_errors_total`: 计数器 (Counter)

## 3. 非功能需求
- **性能**: 开启追踪后的性能损耗 < 5% (在采样率 10% 下)。
- **可靠性**: 遥测组件的异常（如导出失败）不应影响主业务流程。
