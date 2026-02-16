#pragma once

#include <string>
#include <memory>
#include <map>
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
};

} // namespace polyshift
