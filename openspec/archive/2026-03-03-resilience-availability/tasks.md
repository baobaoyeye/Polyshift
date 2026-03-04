# 任务清单 (Tasks): 弹性与高可用性增强

## 阶段一：路由重构 (High-Performance Router)
- [x] **1.1 实现 Radix Tree 路由核心**
    - [x] 定义 `node` 结构体和 `insert` / `search` 算法。
    - [x] 实现参数解析 (`:id`) 和通配符 (`*filepath`) 支持。
    - [x] **UnitTest**: 覆盖各种路由匹配场景 (Static, Param, Wildcard, Priority)。
    - [x] **UnitTest**: 并发读写测试 (Concurrent Read/Write Safety)。
    - [x] **Benchmark**: 基准测试路由匹配性能 (vs Gin Default)。
- [x] **1.2 集成到 Gateway**
    - [x] 替换 `gateway/server.go` 中的 Gin 默认路由逻辑。
    - [x] 实现 `Router` 接口适配器。
    - [x] **IntegrationTest**: 验证 HTTP 请求能正确路由到对应插件。

## 阶段二：熔断机制 (Circuit Breaker)
- [x] **2.1 引入 Circuit Breaker 库**
    - [x] 添加 `sony/gobreaker` 依赖。
    - [x] 在 `config.go` 中添加 `ResilienceConfig`。
- [x] **2.2 集成到 PluginInstance**
    - [x] 在 `PluginInstance` 中初始化 CircuitBreaker。
    - [x] 修改 `HandleRequest`，使用 `cb.Execute` 包裹 gRPC 调用。
    - [x] **UnitTest**: 模拟插件连续报错，验证熔断器开启（状态 Open，拒绝请求）。
    - [x] **UnitTest**: 验证半开状态 (Half-Open) 下的试探逻辑。
    - [x] **UnitTest**: 验证正常请求计数重置逻辑。

## 阶段三：自动保活 (Watchdog)
- [x] **3.1 实现健康检查逻辑**
    - [x] 在 `PluginInstance` 中添加 `LastHeartbeat` 时间戳。
    - [x] 实现 `CheckHealth` 方法（调用 gRPC HealthCheck）。
- [x] **3.2 实现 Watchdog 循环**
    - [x] 在 `PluginManager` 启动时开启后台 Goroutine。
    - [x] 定期轮询所有插件状态。
- [x] **3.3 实现指数退避重启**
    - [x] 实现 `BackoffStrategy`。
    - [x] 当检测到 Dead 状态时，触发重启并应用退避延迟。
    - [x] **UnitTest**: 验证指数退避算法的时间间隔计算 (1s, 2s, 4s...)。
    - [x] **UnitTest**: 验证最大重试次数和重置逻辑。

## 阶段四：集成与验证 (Integration & E2E)
- [x] **4.1 配置支持**
    - [x] 更新 `config.yaml` 示例，添加 resiliency 配置段。
- [x] **4.2 自动化集成测试 (Automated Integration Tests)**
    - [x] **Test**: 编写 `test/integration/router_test.go`，模拟大量路由规则，验证匹配准确性。
    - [x] **Test**: 编写 `test/integration/resilience_test.go`，使用 Mock Plugin 模拟超时和错误，验证熔断器行为。
- [x] **4.3 端到端系统测试 (E2E System Tests)**
    - [x] **E2E**: 编写脚本 `test/e2e/chaos_kill.sh`：
        1. 启动 Core 和真实插件 (Go/Python)。
        2. 发送请求确认正常 (HTTP 200)。
        3. `kill -9 <plugin_pid>`。
        4. 立即发送请求，验证返回 503 (Service Unavailable)。
        5. 等待 5s (Watchdog 周期)，验证插件自动重启。
        6. 发送请求，验证服务恢复 (HTTP 200)。
    - [x] **E2E**: 编写脚本 `test/e2e/circuit_breaker.sh`：
        1. 配置熔断阈值 (3次错误)。
        2. 发送 3 次会导致插件报错的请求。
        3. 发送第 4 次请求，验证 Core 直接返回熔断错误 (无 gRPC 调用)。
        4. 等待冷却时间，验证服务恢复。
- [x] **4.4 性能基准测试 (Performance Benchmarks)**
    - [x] **Benchmark**: 在 1000+ 路由规则下，进行高并发 (1000 QPS) 压测，对比优化前后的延迟 (P99)。
    - [x] **Benchmark**: 熔断器开启/关闭状态下的 Core 吞吐量对比。
