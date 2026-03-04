package com.polyshift.plugin;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import plugin.Plugin;
import plugin.PluginServiceGrpc;
import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.api.trace.propagation.W3CTraceContextPropagator;
import io.opentelemetry.context.propagation.ContextPropagators;
import io.opentelemetry.exporter.logging.LoggingSpanExporter;
import io.opentelemetry.exporter.otlp.trace.OtlpGrpcSpanExporter;
import io.opentelemetry.instrumentation.grpc.v1_6.GrpcTelemetry;
import io.opentelemetry.sdk.OpenTelemetrySdk;
import io.opentelemetry.sdk.resources.Resource;
import io.opentelemetry.sdk.trace.SdkTracerProvider;
import io.opentelemetry.sdk.trace.export.BatchSpanProcessor;
import io.opentelemetry.sdk.trace.export.SimpleSpanProcessor;
import io.opentelemetry.sdk.trace.export.SpanExporter;

import java.io.IOException;
import java.util.Map;
import java.util.logging.Logger;

public class PluginServer {
    private static final Logger logger = Logger.getLogger(PluginServer.class.getName());
    private Server server;
    private RequestHandler handler;
    private Map<String, String> config;
    private final ObjectMapper jsonMapper = new ObjectMapper();
    private OpenTelemetrySdk openTelemetry;

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

    private void initTracing() {
        String serviceName = System.getenv().getOrDefault("OTEL_SERVICE_NAME", "unknown-plugin");
        Resource resource = Resource.getDefault().merge(
                Resource.create(Attributes.of(AttributeKey.stringKey("service.name"), serviceName)));

        SpanExporter exporter = null;
        String exporterType = System.getenv().getOrDefault("OTEL_TRACES_EXPORTER", "otlp");
        boolean useSimpleProcessor = false;

        if ("console".equalsIgnoreCase(exporterType)) {
            exporter = LoggingSpanExporter.create();
            useSimpleProcessor = true;
        } else if ("none".equalsIgnoreCase(exporterType)) {
            return;
        } else {
            String endpoint = System.getenv().getOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317");
            exporter = OtlpGrpcSpanExporter.builder().setEndpoint(endpoint).build();
        }

        SdkTracerProvider sdkTracerProvider = SdkTracerProvider.builder()
                .addSpanProcessor(useSimpleProcessor ? SimpleSpanProcessor.create(exporter) : BatchSpanProcessor.builder(exporter).build())
                .setResource(resource)
                .build();

        openTelemetry = OpenTelemetrySdk.builder()
                .setTracerProvider(sdkTracerProvider)
                .setPropagators(ContextPropagators.create(W3CTraceContextPropagator.getInstance()))
                .buildAndRegisterGlobal();
    }

    public void setOpenTelemetry(OpenTelemetrySdk sdk) {
        this.openTelemetry = sdk;
    }

    public int getPort() {
        return server != null ? server.getPort() : -1;
    }

    public void stop() {
        if (server != null) {
            server.shutdown();
        }
    }

    public void start() throws IOException, InterruptedException {
        if (openTelemetry == null) {
            initTracing();
        }

        ServerBuilder<?> serverBuilder = ServerBuilder.forPort(0);
        
        if (openTelemetry != null) {
            GrpcTelemetry grpcTelemetry = GrpcTelemetry.create(openTelemetry);
            serverBuilder.intercept(grpcTelemetry.newServerInterceptor());
        }

        server = serverBuilder
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
    }
}
