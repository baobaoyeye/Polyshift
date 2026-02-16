package com.polyshift.plugin;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import plugin.Plugin;
import plugin.PluginServiceGrpc;

import java.io.IOException;
import java.util.Map;
import java.util.logging.Logger;

public class PluginServer {
    private static final Logger logger = Logger.getLogger(PluginServer.class.getName());
    private Server server;
    private RequestHandler handler;
    private Map<String, String> config;
    private final ObjectMapper jsonMapper = new ObjectMapper();

    public interface RequestHandler {
        Plugin.ResponseContext handle(Plugin.RequestContext request) throws Exception;
    }

    public void registerHandler(RequestHandler handler) {
        this.handler = handler;
    }
    
    public String getConfig(String key) {
        if (config == null) return null;
        return config.get(key);
    }

    public void start() throws IOException, InterruptedException {
        // Bind to random port
        server = ServerBuilder.forPort(0)
                .addService(new PluginServiceImpl())
                .build()
                .start();

        int port = server.getPort();
        
        // Output address for Core to capture
        System.out.println("|PLUGIN_ADDR|127.0.0.1:" + port + "|");
        System.out.flush();
        
        logger.info("Plugin server listening on 127.0.0.1:" + port);
        
        server.awaitTermination();
    }

    private class PluginServiceImpl extends PluginServiceGrpc.PluginServiceImplBase {
        @Override
        public void init(Plugin.InitRequest request, StreamObserver<Plugin.InitResponse> responseObserver) {
            config = request.getConfigMap();
            logger.info("Plugin initialized with config: " + config);
            
            Plugin.InitResponse response = Plugin.InitResponse.newBuilder()
                    .setSuccess(true)
                    .build();
            
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        }

        @Override
        public void healthCheck(Plugin.Empty request, StreamObserver<Plugin.HealthStatus> responseObserver) {
            Plugin.HealthStatus response = Plugin.HealthStatus.newBuilder()
                    .setStatus(Plugin.HealthStatus.Status.SERVING)
                    .setMessage("OK")
                    .build();
            
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        }

        @Override
        public void handleRequest(Plugin.RequestContext request, StreamObserver<Plugin.ResponseContext> responseObserver) {
            if (handler == null) {
                responseObserver.onNext(Plugin.ResponseContext.newBuilder()
                        .setStatusCode(500)
                        .setErrorMessage("Handler not registered")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            try {
                Plugin.ResponseContext response = handler.handle(request);
                responseObserver.onNext(response);
            } catch (Exception e) {
                logger.severe("Handler failed: " + e.getMessage());
                responseObserver.onNext(Plugin.ResponseContext.newBuilder()
                        .setStatusCode(500)
                        .setErrorMessage(e.toString())
                        .build());
            }
            responseObserver.onCompleted();
        }

        @Override
        public void shutdown(Plugin.Empty request, StreamObserver<Plugin.Empty> responseObserver) {
            logger.info("Shutting down plugin...");
            responseObserver.onNext(Plugin.Empty.newBuilder().build());
            responseObserver.onCompleted();
            
            new Thread(() -> {
                try {
                    // Wait a bit for response to be sent
                    Thread.sleep(100);
                } catch (InterruptedException e) {
                    // ignore
                }
                if (server != null) {
                    server.shutdown();
                }
                System.exit(0);
            }).start();
        }
    }
}
