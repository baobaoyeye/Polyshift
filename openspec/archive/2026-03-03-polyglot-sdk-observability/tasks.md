# 任务清单 (Tasks): 多语言 SDK 可观测性自动注入

## 阶段一：Python SDK 增强
- [x] **1.1 引入依赖**
    - [x] 更新 `sdk/python/requirements.txt`，添加 `opentelemetry-api`, `opentelemetry-sdk`, `opentelemetry-instrumentation-grpc`, `opentelemetry-exporter-otlp`。
- [x] **1.2 实现自动插桩**
    - [x] 修改 `sdk/python/polyshift/plugin/server.py`，在 `start()` 前初始化 OTel SDK。
    - [x] 使用 `GrpcInstrumentorServer` 自动拦截请求。
    - [x] 读取环境变量 `OTEL_TRACES_EXPORTER` 配置导出器。
- [x] **1.3 单元测试 (Unit Tests)**
    - [x] 创建 `sdk/python/tests/test_tracing.py`。
    - [x] 使用 `opentelemetry.sdk.trace.export.InMemorySpanExporter` 捕获 Span。
    - [x] 模拟 gRPC 请求，验证生成的 Span 包含正确的 Trace ID 和属性。
    - [x] 验证环境变量配置是否生效。
- [x] **1.4 验证 (Verification)**
    - [x] **Automated**: 编写脚本 `scripts/verify_python_tracing.sh`。
    - [x] 脚本应自动启动 `examples/py-hello`，发送请求，并正则匹配日志输出确保包含 Trace ID。

## 阶段二：Java SDK 增强
- [x] **2.1 引入依赖**
    - [x] 更新 `sdk/java/pom.xml`，引入 `opentelemetry-bom`, `opentelemetry-grpc-1.6` 和 `opentelemetry-sdk-testing` (test scope)。
- [x] **2.2 实现拦截器**
    - [x] 修改 `sdk/java/src/main/java/com/polyshift/plugin/PluginServer.java`。
    - [x] 初始化 `OpenTelemetry` 实例。
    - [x] 在 `ServerBuilder` 中添加 `GrpcTelemetry.create(otel).newServerInterceptor()`。
- [x] **2.3 单元测试 (Unit Tests)**
    - [x] 创建 `sdk/java/src/test/java/com/polyshift/plugin/TracingTest.java`。
    - [x] 使用 `OpenTelemetryRule` 或 `InMemorySpanExporter`。
    - [x] 启动 `InProcessServer`，发送请求，断言 Span 列表不为空且包含预期属性。
- [x] **2.4 验证 (Verification)**
    - [x] **Automated**: 编写脚本 `scripts/verify_java_tracing.sh`。
    - [x] 脚本应自动构建并启动 `examples/java-hello`，发送请求，并验证日志输出。

## 阶段三：Node.js SDK 增强
- [x] **3.1 引入依赖**
    - [x] 更新 `sdk/js/package.json`，添加 `@opentelemetry/api`, `@opentelemetry/sdk-node`, `@opentelemetry/instrumentation-grpc`。
    - [x] 添加开发依赖 `mocha` 或 `jest`。
- [x] **3.2 实现 SDK 初始化**
    - [x] 修改 `sdk/js/index.js`，在类构造函数或 `start()` 方法中初始化 `NodeSDK`。
    - [x] 确保 `GrpcInstrumentation` 被启用。
- [x] **3.3 单元测试 (Unit Tests)**
    - [x] 创建 `sdk/js/test/tracing.test.js`。
    - [x] 配置 `InMemorySpanExporter`。
    - [x] 启动 Mock Server，发起 gRPC 调用，验证 `exporter.getFinishedSpans()` 中包含正确的 Trace Context。
- [x] **3.4 验证 (Verification)**
    - [x] **Automated**: 编写脚本 `scripts/verify_js_tracing.sh`。
    - [x] 脚本应自动安装依赖并启动 `examples/js-hello`，验证日志中包含 Trace ID。

## 阶段四：C++ SDK 增强
- [x] **4.1 引入依赖**
    - [x] 更新 `sdk/cpp/CMakeLists.txt`，查找并链接 `opentelemetry-cpp`。
    - [x] (可选) 如果系统未安装 OTel，提供构建脚本或文档说明。
- [x] **4.2 实现手动提取**
    - [x] 修改 `sdk/cpp/src/server.cc`。
    - [x] 在 `HandleRequest` 中手动解析 `grpc::ServerContext` 的 Metadata。
    - [x] 使用 `opentelemetry::trace::Tracer` 创建 Span 并注入 Context。
- [x] **4.3 单元测试 (Unit Tests)**
    - [x] 创建 `sdk/cpp/test/tracing_test.cc`。
    - [x] 使用 `opentelemetry::exporter::memory::InMemorySpanExporter`。
    - [x] 构造 `grpc::ServerContext` 包含 `traceparent` Metadata。
    - [x] 调用 `HandleRequest`，验证内存 Exporter 中捕获到了 Child Span。
- [x] **4.4 验证 (Verification)**
    - [x] **Automated**: 编写脚本 `scripts/verify_cpp_tracing.sh`。
    - [x] 脚本应自动编译并启动 `examples/cpp-hello`，验证日志输出。

## 阶段五：文档与清理
- [x] **5.1 更新开发者文档**
    - [x] 更新 `DEVELOPER.md`，说明如何开启/关闭追踪，以及环境变量配置。
- [x] **5.2 清理**
    - [x] 移除测试代码和临时文件。
