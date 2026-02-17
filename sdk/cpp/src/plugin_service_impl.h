#pragma once

#include "polyshift/plugin.h"
#include <grpcpp/grpcpp.h>
#include "opentelemetry/trace/provider.h"
#include "opentelemetry/context/propagation/text_map_propagator.h"
#include "opentelemetry/context/propagation/global_propagator.h"
#include "opentelemetry/trace/propagation/http_trace_context.h"
#include "opentelemetry/context/runtime_context.h"

namespace trace_api = opentelemetry::trace;
namespace context = opentelemetry::context;

// GrpcServerCarrier for extracting context from metadata
class GrpcServerCarrier : public context::propagation::TextMapCarrier {
public:
    GrpcServerCarrier(const std::multimap<grpc::string_ref, grpc::string_ref>& metadata)
        : metadata_(metadata) {}

    opentelemetry::nostd::string_view Get(opentelemetry::nostd::string_view key) const noexcept override {
        auto it = metadata_.find(grpc::string_ref(key.data(), key.size()));
        if (it != metadata_.end()) {
            return opentelemetry::nostd::string_view(it->second.data(), it->second.size());
        }
        return "";
    }

    void Set(opentelemetry::nostd::string_view key, opentelemetry::nostd::string_view value) noexcept override {
        // Not used for extraction
    }

private:
    const std::multimap<grpc::string_ref, grpc::string_ref>& metadata_;
};

class PluginServiceImpl final : public ::plugin::PluginService::Service {
public:
    explicit PluginServiceImpl(std::shared_ptr<polyshift::Plugin> plugin) : plugin_(plugin) {
        // Get tracer
        tracer_ = trace_api::Provider::GetTracerProvider()->GetTracer("polyshift-cpp-plugin");
    }

    grpc::Status Init(grpc::ServerContext* context, const ::plugin::InitRequest* request, ::plugin::InitResponse* response) override {
        // Convert map to std::map
        std::map<std::string, std::string> config;
        for (const auto& pair : request->config()) {
            config[pair.first] = pair.second;
        }
        
        plugin_->Init(config);
        
        response->set_success(true);
        return grpc::Status::OK;
    }

    grpc::Status HandleRequest(grpc::ServerContext* context, const ::plugin::RequestContext* request, ::plugin::ResponseContext* response) override {
        // Extract context from metadata
        const auto& metadata = context->client_metadata();
        GrpcServerCarrier carrier(metadata);
        auto prop = context::propagation::GlobalTextMapPropagator::GetGlobalPropagator();
        auto current_ctx = context::RuntimeContext::GetCurrent();
        auto new_ctx = prop->Extract(carrier, current_ctx);

        // Start span
        trace_api::StartSpanOptions options;
        options.kind = trace_api::SpanKind::kServer;
        options.parent = new_ctx; // Set parent context from extraction
        
        auto span = tracer_->StartSpan("plugin.PluginService/HandleRequest", 
                                       {{"rpc.system", "grpc"},
                                        {"rpc.service", "plugin.PluginService"},
                                        {"rpc.method", "HandleRequest"}},
                                       options);
        
        // Scope the span
        auto scope = tracer_->WithActiveSpan(span);

        // Call plugin logic
        try {
            // Debug: Print Trace ID
            char trace_id_hex[32];
            span->GetContext().trace_id().ToLowerBase16(trace_id_hex);
            std::cout << "Handling request with Trace ID: " << std::string(trace_id_hex, 32) << std::endl;

            plugin_->HandleRequest(*request, response);
            span->SetStatus(trace_api::StatusCode::kOk);
        } catch (const std::exception& e) {
            span->SetStatus(trace_api::StatusCode::kError, e.what());
            span->End();
            return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
        }

        span->End();
        return grpc::Status::OK;
    }

    grpc::Status Shutdown(grpc::ServerContext* context, const ::plugin::Empty* request, ::plugin::Empty* response) override {
        // TODO: Graceful shutdown logic if needed
        return grpc::Status::OK;
    }

    grpc::Status HealthCheck(grpc::ServerContext* context, const ::plugin::Empty* request, ::plugin::HealthStatus* response) override {
        response->set_status(::plugin::HealthStatus::SERVING);
        return grpc::Status::OK;
    }

private:
    std::shared_ptr<polyshift::Plugin> plugin_;
    opentelemetry::nostd::shared_ptr<opentelemetry::trace::Tracer> tracer_;
};
