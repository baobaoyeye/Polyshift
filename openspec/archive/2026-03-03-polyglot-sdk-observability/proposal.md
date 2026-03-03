# 提案 (Proposal): 多语言 SDK 可观测性自动注入 (Polyglot SDK Observability Auto-Instrumentation)

## 1. 背景与问题 (Context & Problem)
*   **背景**: Polyshift 框架旨在支持多语言微服务架构。目前，核心层 (Core) 和 Go 语言 SDK 已经集成了 OpenTelemetry，能够生成和传播 Trace Context。
*   **问题**: 
    *   **链路断裂**: Python, Java, Node.js, C++ 等其他语言的 SDK 尚未集成 OpenTelemetry。当请求从 Core 转发到这些语言编写的插件时，Trace ID 无法透传，导致全链路追踪中断。
    *   **开发成本高**: 插件开发者如果想要实现追踪，目前需要手动引入 OTel 库并编写大量样板代码来解析 gRPC Metadata，这违背了 SDK "开箱即用" 的设计初衷。
    *   **体验不一致**: Go 开发者享有自动化的可观测性支持，而其他语言开发者则没有，导致开发体验割裂。

## 2. 目标 (Goals)
*   [ ] **全语言覆盖**: 在 Python, Java, Node.js, C++ 四种语言的 SDK 中，默认集成 OpenTelemetry gRPC Server Interceptor/Handler。
*   [ ] **自动注入**: SDK 初始化时自动配置 Tracer Provider 和 Exporter (默认 stdout 或 OTLP)，无需用户干预。
*   [ ] **上下文透传**: 确保 `traceparent` Header 能够从 gRPC Metadata 中正确提取，并注入到当前请求的 Context 中。
*   [ ] **零代码侵入**: 插件开发者升级 SDK 版本后，即可自动获得追踪能力，无需修改业务代码。

## 3. 非目标 (Non-Goals)
*   [ ] **业务指标监控 (Metrics)**: 本次迭代主要关注 **Tracing (追踪)**，Metrics (如 CPU、内存、自定义计数器) 暂不作为强制集成项，后续迭代再考虑。
*   [ ] **日志集成 (Logging)**: 虽然 Trace ID 需要注入到日志中，但本次不强制要求所有语言 SDK 都实现结构化日志库的替换，仅确保 Trace Context 可用。

## 4. 核心价值 (Value Proposition)
*   **端到端可见性**: 实现从 HTTP 入口 -> Core -> 任意语言 Plugin 的完整调用链追踪，彻底消除可观测性盲区。
*   **降低认知负担**: 开发者无需学习复杂的 OpenTelemetry API，只需关注业务逻辑，SDK 默默处理所有遥测数据。
*   **标准化**: 统一所有语言的遥测数据格式 (OTLP/W3C)，便于对接统一的后端分析平台 (如 Jaeger, Tempo)。

## 5. 风险与缓解 (Risks & Mitigation)
*   **风险 1**: **依赖冲突 (Dependency Hell)**。引入 OTel SDK 可能与用户业务代码中的依赖产生版本冲突（尤其是 Java 和 Python）。
    *   **缓解**: 
        *   **Java**: 使用 `Shaded Jar` 或 `Bill of Materials (BOM)` 管理依赖。
        *   **Python**: 明确声明依赖版本范围，或提供仅包含 API 的轻量级包。
*   **风险 2**: **性能开销**。自动注入可能带来额外的延迟和内存消耗。
    *   **缓解**: 
        *   默认开启 **采样 (Sampling)**，如默认采样率 1.0 (全采) 但支持配置降低。
        *   提供环境变量开关 `POLYSHIFT_OTEL_ENABLED=false` 以便在极端性能敏感场景下关闭。
*   **风险 3**: **C++ 构建复杂性**。引入 `opentelemetry-cpp` 可能导致编译时间显著增加或链接错误。
    *   **缓解**: 使用 `vcpkg` 或 `conan` 管理依赖，或者在 SDK 中提供预编译的静态库。

## 6. 成功指标 (Success Metrics)
*   [ ] 运行 `examples/py-hello`，在控制台或 Jaeger 中能看到完整的 Trace 树 (Core -> Python Plugin)。
*   [ ] 运行 `examples/cpp-hello`，同样能看到完整的 Trace。
*   [ ] 引入 OTel 后，插件启动时间增加不超过 500ms。
