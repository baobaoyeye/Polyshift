# 需求规范 (Requirements): 弹性与高可用性增强

## 1. 功能性需求 (Functional Requirements)

### 1.1 健康监控与自动重启 (Watchdog)
*   **主动探测**: Core 应每隔 `interval` (默认 5s) 向插件发送 gRPC `HealthCheck` 请求。
*   **被动感知**: Core 应监听子进程的退出信号 (Exit Code)。
*   **判定标准**:
    *   连续 `threshold` (默认 3次) 探测失败（超时或非 Serving 状态）。
    *   子进程意外退出。
*   **恢复动作**:
    *   立即重新启动插件进程。
    *   **指数退避**: 首次重启等待 0s，第二次 1s，第三次 2s，以此类推，最大等待 `max_backoff` (60s)。
    *   **重置机制**: 如果插件健康运行超过 `reset_interval` (10m)，则重置退避计数器。

### 1.2 熔断器 (Circuit Breaker)
*   **范围**: 针对每个插件实例（Plugin Instance）独立维护一个熔断器。
*   **状态机**: `Closed` (正常) -> `Open` (熔断) -> `Half-Open` (半开)。
*   **触发条件 (Open)**:
    *   在滑动窗口（如 10s）内，请求总数 >= `min_request` (10)。
    *   错误率 (Error Rate) >= `error_threshold` (50%)。
    *   或 连续失败次数 >= `consecutive_failures` (5)。
*   **熔断行为**:
    *   状态为 `Open` 时，直接拒绝所有请求，返回 `503 Service Unavailable`，不发起 gRPC 调用。
*   **恢复条件 (Half-Open)**:
    *   经过 `cooldown` (默认 30s) 后，进入 `Half-Open`。
    *   允许放行 1 个请求进行试探。
    *   若成功，转为 `Closed`；若失败，重新转为 `Open` 并重置冷却时间。

### 1.3 高性能动态路由 (High-Perf Dynamic Router)
*   **数据结构**: 使用 Radix Tree (基数树) 存储路由规则。
*   **匹配规则**:
    *   支持静态路径: `/api/users`
    *   支持参数路径: `/api/users/:id`
    *   支持通配符: `/api/files/*filepath`
    *   优先级: 静态 > 参数 > 通配符。
*   **原子更新**: 支持在运行时全量替换路由树，不影响正在处理的请求（Read-Copy-Update 或 Atomic Swap）。

## 2. 非功能性需求 (Non-Functional Requirements)
*   **性能**: 熔断器判断逻辑耗时 < 10μs。
*   **资源**: Watchdog 协程应复用，避免为每个插件创建过多 Goroutine（可使用 Worker Pool 或 Ticker 轮询）。

## 3. 接口变更 (API Changes)
*   **Config**: `plugin.yaml` 支持配置熔断和健康检查参数。
    ```yaml
    resilience:
      health_check_interval: 5s
      max_retries: 3
      circuit_breaker:
        enabled: true
        error_threshold: 0.5
    ```
