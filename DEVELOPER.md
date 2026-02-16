# 插件开发指南 (Developer Guide)

本文档旨在指导开发者如何为 Polyshift 微内核框架开发多语言插件。

## 核心协议 (Core Protocol)

插件与核心框架通过 gRPC 通信。协议定义在 `proto/plugin/plugin.proto`。

```protobuf
service PluginService {
    // 插件初始化
    rpc Init(InitRequest) returns (InitResponse);
    // 处理业务请求
    rpc HandleRequest(RequestContext) returns (ResponseContext);
    // 健康检查
    rpc HealthCheck(Empty) returns (HealthStatus);
    // 优雅停机
    rpc Shutdown(Empty) returns (Empty);
}
```

插件启动时，必须将 gRPC 服务监听地址通过标准输出 (stdout) 打印，格式为：
`|PLUGIN_ADDR|<address>|`
例如：`|PLUGIN_ADDR|127.0.0.1:50051|`

## 开发指南 (Language Guides)

### Go 插件

使用 `sdk/go` 包简化开发。

```go
package main

import (
    "context"
    "fmt"
    "github.com/polyshift/microkernel/sdk/go/pkg/plugin"
    pb "github.com/polyshift/microkernel/proto/plugin"
)

func main() {
    server := plugin.NewServer()

    // 注册业务处理函数
    server.RegisterHandler(func(ctx context.Context, req *pb.RequestContext) (*pb.ResponseContext, error) {
        // 获取配置
        greeting := server.GetConfig("greeting")
        if greeting == "" {
            greeting = "Hello"
        }

        return &pb.ResponseContext{
            StatusCode: 200,
            Body:       []byte(fmt.Sprintf("%s from Go!", greeting)),
            Headers:    map[string]string{"Content-Type": "text/plain"},
        }, nil
    })

    if err := server.Start(); err != nil {
        panic(err)
    }
}
```

### Python 插件

使用 `sdk/python` 包。

```python
from polyshift.plugin import Plugin, Server

class MyPlugin(Plugin):
    def handle_request(self, request):
        # 获取配置
        greeting = self.config.get("greeting", "Hello")
        
        return {
            "status_code": 200,
            "body": f"{greeting} from Python!".encode('utf-8'),
            "headers": {"Content-Type": "text/plain"}
        }

if __name__ == "__main__":
    Server(MyPlugin()).serve()
```

### Node.js 插件

使用 `sdk/js` 包。

```javascript
const { Plugin, Server } = require('./sdk/js');

class MyPlugin extends Plugin {
    async handleRequest(request) {
        // 获取配置
        const greeting = this.config['greeting'] || 'Hello';
        
        return {
            statusCode: 200,
            body: Buffer.from(`${greeting} from Node.js!`),
            headers: { "Content-Type": "text/plain" }
        };
    }
}

new Server(new MyPlugin()).start();
```

### Java 插件

使用 `sdk/java` 包。

```java
package com.example.hello;

import com.polyshift.plugin.Plugin;
import com.polyshift.plugin.PluginServer;
import com.polyshift.proto.PluginProto;
import java.util.Map;

public class HelloPlugin implements Plugin {
    private String greeting = "Hello";

    @Override
    public void init(Map<String, String> config) {
        if (config.containsKey("greeting")) {
            this.greeting = config.get("greeting");
        }
    }

    @Override
    public PluginProto.ResponseContext handleRequest(PluginProto.RequestContext request) {
        return PluginProto.ResponseContext.newBuilder()
                .setStatusCode(200)
                .putHeaders("Content-Type", "text/plain")
                .setBody(com.google.protobuf.ByteString.copyFromUtf8(greeting + " from Java!"))
                .build();
    }

    public static void main(String[] args) throws Exception {
        new PluginServer(new HelloPlugin()).start();
    }
}
```

### C++ 插件

使用 `sdk/cpp` 包。

```cpp
#include "polyshift/plugin.h"
#include <iostream>

using namespace polyshift;

class HelloPlugin : public Plugin {
public:
    void Init(const std::map<std::string, std::string>& config) override {
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
```
