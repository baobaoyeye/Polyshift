package com.polyshift.plugin;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.api.trace.propagation.W3CTraceContextPropagator;
import io.opentelemetry.context.propagation.ContextPropagators;
import io.opentelemetry.sdk.OpenTelemetrySdk;
import io.opentelemetry.sdk.resources.Resource;
import io.opentelemetry.sdk.testing.exporter.InMemorySpanExporter;
import io.opentelemetry.sdk.trace.SdkTracerProvider;
import io.opentelemetry.sdk.trace.data.SpanData;
import io.opentelemetry.sdk.trace.export.SimpleSpanProcessor;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import plugin.Plugin;
import plugin.PluginServiceGrpc;

import java.io.IOException;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class TracingTest {
    private PluginServer server;
    private InMemorySpanExporter spanExporter;
    private OpenTelemetrySdk openTelemetrySdk;
    private Thread serverThread;

    @BeforeEach
    void setup() {
        spanExporter = InMemorySpanExporter.create();
        SdkTracerProvider tracerProvider = SdkTracerProvider.builder()
                .addSpanProcessor(SimpleSpanProcessor.create(spanExporter))
                .setResource(Resource.create(Attributes.of(AttributeKey.stringKey("service.name"), "test-service")))
                .build();
        
        openTelemetrySdk = OpenTelemetrySdk.builder()
                .setTracerProvider(tracerProvider)
                .setPropagators(ContextPropagators.create(W3CTraceContextPropagator.getInstance()))
                .build();

        server = new PluginServer();
        server.setOpenTelemetry(openTelemetrySdk);
        
        serverThread = new Thread(() -> {
            try {
                server.start();
            } catch (Exception e) {
                // Ignore interruption
            }
        });
        serverThread.start();
        
        // Wait for server to start
        long start = System.currentTimeMillis();
        while (server.getPort() <= 0 && System.currentTimeMillis() - start < 5000) {
            try {
                Thread.sleep(100);
            } catch (InterruptedException e) {
                // Ignore
            }
        }
    }

    @AfterEach
    void cleanup() {
        if (server != null) {
            server.stop();
        }
        if (serverThread != null) {
            serverThread.interrupt();
        }
    }

    @Test
    void testHealthCheckCreatesSpan() {
        ManagedChannel channel = ManagedChannelBuilder.forAddress("localhost", server.getPort())
                .usePlaintext()
                .build();
        
        PluginServiceGrpc.PluginServiceBlockingStub stub = PluginServiceGrpc.newBlockingStub(channel);
        
        Plugin.HealthStatus status = stub.healthCheck(Plugin.Empty.newBuilder().build());
        assertEquals(Plugin.HealthStatus.Status.SERVING, status.getStatus());
        
        // Wait for spans
        long start = System.currentTimeMillis();
        while (spanExporter.getFinishedSpanItems().isEmpty() && System.currentTimeMillis() - start < 5000) {
            try {
                Thread.sleep(100);
            } catch (InterruptedException e) {
                // Ignore
            }
        }
        
        List<SpanData> spans = spanExporter.getFinishedSpanItems();
        assertTrue(spans.size() > 0, "Should have captured spans");
        
        boolean found = false;
        for (SpanData span : spans) {
            if (span.getName().contains("HealthCheck")) {
                found = true;
                break;
            }
        }
        assertTrue(found, "Should have HealthCheck span");
        
        channel.shutdown();
    }
}
