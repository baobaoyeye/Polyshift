#include "polyshift/plugin.h"
#include <iostream>
#include <memory>
#include <thread>
#include <chrono>

// Dummy plugin implementation for testing
class DummyPlugin : public polyshift::Plugin {
public:
    void Init(const std::map<std::string, std::string>& config) override {
        std::cout << "DummyPlugin initialized" << std::endl;
    }

    void HandleRequest(const ::plugin::RequestContext& request, ::plugin::ResponseContext* response) override {
        std::cout << "DummyPlugin handling request: " << request.path() << std::endl;
        response->set_status_code(200);
        response->set_body("Hello from C++ Plugin");
    }
};

int main(int argc, char** argv) {
    auto plugin = std::make_shared<DummyPlugin>();
    polyshift::Server server(plugin);
    
    std::cout << "Starting C++ Plugin Server..." << std::endl;
    server.Start();
    
    return 0;
}
