#include <gtest/gtest.h>
#include "polyshift/plugin.h"
#include "src/plugin_service_impl.h"
#include "opentelemetry/sdk/trace/simple_processor_factory.h"
#include "opentelemetry/sdk/trace/tracer_provider_factory.h"
#include "opentelemetry/sdk/trace/span_data.h"
#include "opentelemetry/trace/provider.h"
#include "opentelemetry/exporters/memory/in_memory_span_exporter.h"
#include "opentelemetry/trace/noop.h"

namespace trace_api = opentelemetry::trace;
namespace trace_sdk = opentelemetry::sdk::trace;
namespace trace_exporter = opentelemetry::exporter::memory;

class MockPlugin : public polyshift::Plugin {
public:
    void Init(const std::map<std::string, std::string>& config) override {}
    void HandleRequest(const ::plugin::RequestContext& req, ::plugin::ResponseContext* resp) override {
        if (req.path() == "/error") {
            throw std::runtime_error("Mock error");
        }
        resp->set_status_code(200);
    }
};

class TracingTest : public ::testing::Test {
protected:
    void SetUp() override {
        std::unique_ptr<trace_exporter::InMemorySpanExporter> exporter = std::unique_ptr<trace_exporter::InMemorySpanExporter>(new trace_exporter::InMemorySpanExporter());
        exporter_raw_ = exporter.get();
        
        auto processor = trace_sdk::SimpleSpanProcessorFactory::Create(std::move(exporter));
        auto provider = trace_sdk::TracerProviderFactory::Create(std::move(processor));
        
        // Set global provider
        trace_api::Provider::SetTracerProvider(
            opentelemetry::nostd::shared_ptr<trace_api::TracerProvider>(provider.release())
        );
    }

    void TearDown() override {
        // Reset provider to noop or something safe
        // Note: OpenTelemetry C++ doesn't have a direct "Reset" but we can set a new Noop provider
        trace_api::Provider::SetTracerProvider(
            opentelemetry::nostd::shared_ptr<trace_api::TracerProvider>(new trace_api::NoopTracerProvider())
        );
    }

    trace_exporter::InMemorySpanExporter* exporter_raw_;
};

TEST_F(TracingTest, HandleRequestCreatesSpan) {
    auto plugin = std::make_shared<MockPlugin>();
    auto service = std::make_shared<PluginServiceImpl>(plugin);
    
    grpc::ServerContext context;
    ::plugin::RequestContext request;
    request.set_path("/test");
    ::plugin::ResponseContext response;
    
    service->HandleRequest(&context, &request, &response);
    
    auto data_container = exporter_raw_->GetData();
    auto spans = data_container->GetSpans();
    ASSERT_EQ(spans.size(), 1);
    
    auto span_data = spans[0].get();
    ASSERT_NE(span_data, nullptr);

    auto span_name = span_data->GetName();
    EXPECT_EQ(std::string(span_name.data(), span_name.size()), "plugin.PluginService/HandleRequest");
    EXPECT_EQ(span_data->GetStatus(), trace_api::StatusCode::kOk);
}

TEST_F(TracingTest, HandleRequestErrorSpan) {
    auto plugin = std::make_shared<MockPlugin>();
    auto service = std::make_shared<PluginServiceImpl>(plugin);
    
    grpc::ServerContext context;
    ::plugin::RequestContext request;
    request.set_path("/error");
    ::plugin::ResponseContext response;
    
    grpc::Status status = service->HandleRequest(&context, &request, &response);
    
    EXPECT_EQ(status.error_code(), grpc::StatusCode::INTERNAL);
    
    auto data_container = exporter_raw_->GetData();
    auto spans = data_container->GetSpans();
    ASSERT_EQ(spans.size(), 1);
    
    auto span_data = spans[0].get();
    ASSERT_NE(span_data, nullptr);

    auto span_name = span_data->GetName();
    EXPECT_EQ(std::string(span_name.data(), span_name.size()), "plugin.PluginService/HandleRequest");
    EXPECT_EQ(span_data->GetStatus(), trace_api::StatusCode::kError);
    auto desc = span_data->GetDescription();
    EXPECT_EQ(std::string(desc.data(), desc.size()), "Mock error");
}
