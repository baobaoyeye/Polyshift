#pragma once

#include <string>
#include <memory>
#include <map>
#include <mutex>
#include <condition_variable>
#include <grpcpp/grpcpp.h>
#include "proto/plugin/plugin.grpc.pb.h"

namespace polyshift {

class Plugin {
public:
    virtual ~Plugin() = default;
    
    // 初始化插件
    virtual void Init(const std::map<std::string, std::string>& config) {}

    // 处理请求
    virtual void HandleRequest(const ::plugin::RequestContext& request, ::plugin::ResponseContext* response) = 0;
};

class Server {
public:
    explicit Server(std::shared_ptr<Plugin> plugin);
    void Start();

private:
    std::shared_ptr<Plugin> plugin_;
    int selected_port_;
    std::mutex mutex_;
    std::condition_variable cv_;
    bool shutdown_requested_ = false;
    std::unique_ptr<grpc::Server> grpc_server_;
};

} // namespace polyshift
