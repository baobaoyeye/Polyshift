# 需求规范 (Requirements): 微内核多语言服务框架

## 1. 用户故事 (User Stories)
| ID | As a (角色) | I want to (功能) | So that (价值) | Priority |
| :--- | :--- | :--- | :--- | :--- |
| US-001 | 平台管理员 | 在不重启核心服务的情况下加载、更新或卸载插件 | 保证服务的持续可用性，实现零停机更新 | P0 |
| US-002 | 业务开发者 | 使用 Java, Python, Go, Node.js 等不同语言开发业务插件 | 利用各语言生态优势，提高开发效率 | P0 |
| US-003 | 客户端 | 通过 HTTP 或 gRPC 协议访问服务 | 能够灵活接入不同的客户端应用 | P0 |
| US-004 | 运维人员 | 查看所有已加载插件的状态（健康/异常） | 及时发现并处理故障 | P1 |
| US-005 | 核心开发者 | 定义通用的业务上下文（如 RequestID, UserID） | 在插件间透传关键信息，实现全链路追踪 | P1 |

## 2. 功能性需求 (Functional Requirements)

### 2.1 核心微内核 (Microkernel Core)
*   **前置条件**: 配置文件已存在。
*   **输入**: 启动命令。
*   **处理逻辑**:
    1.  初始化日志、配置中心。
    2.  启动插件管理器（Plugin Manager）。
    3.  启动网关（Gateway）。
    4.  监听系统信号，优雅退出。
*   **输出**: 服务启动日志，监听端口。
*   **后置条件**: 服务处于 Running 状态。

### 2.2 插件管理 (Plugin Management)
*   **插件发现**: 能够扫描指定目录或通过 API 注册新插件。
*   **生命周期管理**:
    *   `Load`: 启动插件进程，建立通信连接。
    *   `Init`: 调用插件初始化接口。
    *   `Reload`: 热更新插件（新版本启动 -> 流量切换 -> 旧版本停止）。
    *   `Unload`: 停止插件进程，释放资源。
*   **健康检查**: 定期向插件发送 Heartbeat，超时则标记为异常并尝试重启。

### 2.3 多协议网关 (Multi-Protocol Gateway)
*   **HTTP 支持**: 监听 HTTP 端口，解析 URL Path，将请求路由到对应插件。
*   **gRPC 支持**: 监听 gRPC 端口，根据 Service/Method 名称路由请求。
*   **协议转换**: 支持将 HTTP 请求转换为内部协议（如 Protobuf）转发给插件。

### 2.4 通用业务处理 (Common Business Process)
*   **Context 管理**: 每个请求生成唯一的 TraceID，并携带 UserID, TenantID 等上下文信息。
*   **Middleware 链**: 支持在请求到达插件前/后执行通用逻辑（鉴权、限流、日志）。

## 3. 非功能性需求 (Non-Functional Requirements)
*   **性能 (Performance)**: 核心层转发延迟 < 5ms。
*   **隔离性 (Isolation)**: 单个插件崩溃不应影响核心服务和其他插件。
*   **可扩展性 (Extensibility)**: 支持未来添加新的语言运行时支持。
*   **兼容性 (Compatibility)**: 插件接口应保持向后兼容。

## 4. 验收标准 (Acceptance Criteria)
*   [ ] AC-01: 成功启动核心服务，并加载至少一个 Go 插件和一个 Python 插件。
*   [ ] AC-02: 通过 HTTP 请求访问 Go 插件提供的接口，返回正确结果。
*   [ ] AC-03: 更新 Python 插件代码后执行 Reload，服务不中断且新逻辑生效。
*   [ ] AC-04: 模拟插件进程崩溃，核心服务能检测到并自动拉起插件。
