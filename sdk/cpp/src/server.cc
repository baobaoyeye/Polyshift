#include "polyshift/plugin.h"
#include <grpcpp/grpcpp.h>
#include <iostream>

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::Status;

class PluginServiceImpl final : public ::plugin::PluginService::Service {
public:
    explicit PluginServiceImpl(std::shared_ptr<polyshift::Plugin> plugin) : plugin_(plugin) {}

    Status Init(ServerContext* context, const ::plugin::InitRequest* request, ::plugin::InitResponse* response) override {
        // Convert map to std::map
        std::map<std::string, std::string> config;
        for (const auto& pair : request->config()) {
            config[pair.first] = pair.second;
        }
        
        plugin_->Init(config);
        
        response->set_success(true);
        return Status::OK;
    }

    Status HandleRequest(ServerContext* context, const ::plugin::RequestContext* request, ::plugin::ResponseContext* response) override {
        plugin_->HandleRequest(*request, response);
        return Status::OK;
    }

    Status Shutdown(ServerContext* context, const ::plugin::Empty* request, ::plugin::Empty* response) override {
        // TODO: Graceful shutdown logic if needed
        return Status::OK;
    }

    Status HealthCheck(ServerContext* context, const ::plugin::Empty* request, ::plugin::HealthStatus* response) override {
        response->set_status(::plugin::HealthStatus::SERVING);
        return Status::OK;
    }

private:
    std::shared_ptr<polyshift::Plugin> plugin_;
};

namespace polyshift {

Server::Server(std::shared_ptr<Plugin> plugin) : plugin_(plugin), selected_port_(0) {}

void Server::Start() {
    std::string server_address("0.0.0.0:0");
    PluginServiceImpl service(plugin_);

    ServerBuilder builder;
    builder.AddListeningPort(server_address, grpc::InsecureServerCredentials(), &selected_port_);
    builder.RegisterService(&service);
    std::unique_ptr<grpc::Server> server(builder.BuildAndStart());
    
    // Print address to stdout for core to capture
    // Format: |PLUGIN_ADDR|<addr>|
    std::cout << "|PLUGIN_ADDR|127.0.0.1:" << selected_port_ << "|" << std::endl;
    
    server->Wait();
}

} // namespace polyshift
