package com.example.hello;

import com.google.protobuf.ByteString;
import com.polyshift.plugin.PluginServer;
import plugin.Plugin;

import java.nio.charset.StandardCharsets;

public class HelloPlugin {
    public static void main(String[] args) throws Exception {
        final PluginServer server = new PluginServer();
        
        server.registerHandler(request -> {
            String path = request.getPath();
            String responseBody;
            
            if ("/api/java".equals(path)) {
                String greeting = server.getConfig("greeting");
                if (greeting == null || greeting.isEmpty()) {
                    greeting = "Hello";
                }
                responseBody = "{\"message\": \"" + greeting + " from Java Plugin!\"}";
            } else {
                responseBody = "{\"message\": \"Java Echo: " + path + "\"}";
            }
            
            return Plugin.ResponseContext.newBuilder()
                    .setStatusCode(200)
                    .setBody(ByteString.copyFrom(responseBody, StandardCharsets.UTF_8))
                    .putHeaders("Content-Type", "application/json")
                    .build();
        });
        
        server.start();
    }
}
