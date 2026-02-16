#include "polyshift/plugin.h"
#include <iostream>

using namespace polyshift;

class HelloPlugin : public Plugin {
public:
    void Init(const std::map<std::string, std::string>& config) override {
        // Optional: read config
        if (config.find("greeting") != config.end()) {
            greeting_ = config.at("greeting");
        }
    }

    void HandleRequest(const ::plugin::RequestContext& request, ::plugin::ResponseContext* response) override {
        response->set_status_code(200);
        response->set_body(greeting_ + " from C++!");
        auto* map = response->mutable_headers();
        (*map)["Content-Type"] = "text/plain";
    }

private:
    std::string greeting_ = "Hello";
};

int main() {
    auto plugin = std::make_shared<HelloPlugin>();
    Server server(plugin);
    server.Start();
    return 0;
}
