#include "polyshift/plugin.h"
#include <grpcpp/grpcpp.h>
#include <iostream>
#include <memory>
#include <string>
#include <vector>
#include <cstdlib>

// OpenTelemetry Includes
#include "opentelemetry/exporters/otlp/otlp_grpc_exporter_factory.h"
#include "opentelemetry/exporters/otlp/otlp_grpc_exporter_options.h"
#include "opentelemetry/exporters/ostream/span_exporter_factory.h"
#include "opentelemetry/sdk/trace/simple_processor_factory.h"
#include "opentelemetry/sdk/trace/tracer_provider_factory.h"
#include "opentelemetry/trace/provider.h"
#include "opentelemetry/context/propagation/global_propagator.h"
#include "opentelemetry/context/propagation/text_map_propagator.h"
#include "opentelemetry/trace/propagation/http_trace_context.h"
#include "opentelemetry/sdk/resource/resource.h"
#include "plugin_service_impl.h"

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::Status;
namespace trace_api = opentelemetry::trace;
namespace trace_sdk = opentelemetry::sdk::trace;
namespace otlp = opentelemetry::exporter::otlp;
namespace context = opentelemetry::context;
namespace resource = opentelemetry::sdk::resource;

namespace trace_exporter = opentelemetry::exporter::trace;

void InitTracer() {
    std::unique_ptr<trace_sdk::SpanExporter> exporter;
    const char* exporter_type = std::getenv("OTEL_TRACES_EXPORTER");
    
    if (exporter_type && std::string(exporter_type) == "console") {
        std::cout << "Initializing Console Exporter" << std::endl;
        exporter = trace_exporter::OStreamSpanExporterFactory::Create();
    } else if (exporter_type && std::string(exporter_type) == "none") {
        std::cout << "OTEL_TRACES_EXPORTER=none, tracing disabled." << std::endl;
        return;
    } else {
        std::cout << "Initializing OTLP Exporter" << std::endl;
        exporter = otlp::OtlpGrpcExporterFactory::Create();
    }
    
    auto processor = trace_sdk::SimpleSpanProcessorFactory::Create(std::move(exporter));
    
    // Create Resource
    resource::ResourceAttributes attributes = {
        {"service.name", "unknown-plugin"}
    };
    const char* service_name = std::getenv("OTEL_SERVICE_NAME");
    if (service_name) {
        attributes["service.name"] = service_name;
    }
    auto resource = resource::Resource::Create(attributes);
    
    auto provider = trace_sdk::TracerProviderFactory::Create(std::move(processor), resource);
    
    // Set global tracer provider
    trace_api::Provider::SetTracerProvider(
        opentelemetry::nostd::shared_ptr<trace_api::TracerProvider>(provider.release())
    );
    
    // Set global propagator
    opentelemetry::context::propagation::GlobalTextMapPropagator::SetGlobalPropagator(
        opentelemetry::nostd::shared_ptr<opentelemetry::context::propagation::TextMapPropagator>(
            new opentelemetry::trace::propagation::HttpTraceContext()
        )
    );
}

namespace polyshift {

Server::Server(std::shared_ptr<Plugin> plugin) : plugin_(plugin), selected_port_(0) {}

void Server::Start() {
    // Initialize OpenTelemetry
    ::InitTracer();

    std::string server_address("0.0.0.0:0");
    
    // Pass shutdown callback to service
    PluginServiceImpl service(plugin_, [this]() {
        std::lock_guard<std::mutex> lock(mutex_);
        shutdown_requested_ = true;
        cv_.notify_one();
    });

    ServerBuilder builder;
    builder.AddListeningPort(server_address, grpc::InsecureServerCredentials(), &selected_port_);
    builder.RegisterService(&service);
    grpc_server_ = builder.BuildAndStart();
    
    if (!grpc_server_) {
        std::cerr << "Failed to start server." << std::endl;
        {
            std::lock_guard<std::mutex> lock(mutex_);
            shutdown_requested_ = true;
        }
        cv_.notify_all();
    } else {
        // Print address to stdout for core to capture
        // Format: |PLUGIN_ADDR|<addr>|
        std::cout << "|PLUGIN_ADDR|127.0.0.1:" << selected_port_ << "|" << std::endl;
    }
    
    // Wait for shutdown signal instead of server->Wait()
    {
        std::unique_lock<std::mutex> lock(mutex_);
        cv_.wait(lock, [this]{ return shutdown_requested_; });
    }

    std::cout << "Shutting down server..." << std::endl;
    if (grpc_server_) {
        grpc_server_->Shutdown();
        // Wait ensures that all RPCs are finished
        grpc_server_->Wait();
    }
}

} // namespace polyshift
