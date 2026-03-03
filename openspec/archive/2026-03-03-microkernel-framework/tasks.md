# 任务清单 (Implementation Tasks): 微内核多语言服务框架

## 阶段一：核心与协议 (Core & Protocol)
- [x] **1.1 协议定义**
    - [x] 定义 `proto/plugin.proto` (Service, Messages)
    - [x] 生成 Go gRPC 代码 (其他语言将在 SDK 阶段生成)
- [x] **1.2 核心框架 (Microkernel Core)**
    - [x] 实现 `PluginManager`: 启动/停止子进程，管理 gRPC 连接池 (基础骨架)
    - [x] 实现 `Gateway`: HTTP Server (Gin)，请求路由分发 (基础骨架)
    - [x] 实现 `ConfigLoader`: 解析 `plugin.yaml`

## 阶段二：多语言 SDK (Polyglot SDKs)
- [x] **2.1 Go SDK**
    - [x] 封装 `PluginServer` 启动逻辑
    - [x] 提供 `Handle` 接口定义
- [x] **2.2 Python SDK**
    - [x] 封装 `grpc.server`
    - [x] 提供装饰器风格的路由注册
- [x] **2.3 Java SDK**
    - [x] 基于 Maven/Gradle 发布 SDK 包
    - [x] 提供注解支持 (`@PluginHandler` 或 Functional Interface)
- [x] **2.4 Node.js SDK**
    - [x] 基于 npm 发布 SDK 包
    - [x] 提供 Express/Koa 风格的中间件接口
- [x] **2.5 C++ SDK**
    - [x] 基于 CMake 构建
    - [x] 提供高性能处理接口

## 阶段三：高级特性 (Advanced Features)
- [x] **3.1 插件热插拔 (Hot Reload)**
    - [x] 实现平滑升级逻辑 (Drain old process -> Switch traffic -> Kill old process)
    - [x] CLI 工具支持 `reload` 命令 (通过 Admin API 实现)
- [x] **3.2 通用中间件**
    - [x] 实现 `AuthMiddleware` (API Key / JWT)
    - [x] 实现 `RateLimitMiddleware` (Token Bucket)
    - [x] 实现 `LoggingMiddleware` (Access Log, TraceID)

## 阶段四：验证与交付 (Verification & Delivery)
- [x] **4.1 示例插件**
    - [x] 编写 `examples/go-hello`
    - [x] 编写 `examples/py-hello` (替代 py-calc)
    - [x] 编写 `examples/js-hello`
    - [x] 编写 `examples/java-hello` (替代 java-user)
- [x] **4.2 集成测试**
    - [x] 测试多语言插件共存
    - [x] 测试热更新时的流量无损
- [x] **4.3 文档编写**
    - [ ] 编写 `README.md` (快速开始)
    - [ ] 编写 `DEVELOPER.md` (插件开发指南)
