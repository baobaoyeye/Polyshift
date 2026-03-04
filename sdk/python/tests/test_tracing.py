import unittest
import os
import grpc
import time
from concurrent import futures
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
from polyshift.plugin import server, plugin_pb2, plugin_pb2_grpc

class TestTracing(unittest.TestCase):
    def setUp(self):
        # Set env to avoid default OTLP exporter
        os.environ["OTEL_TRACES_EXPORTER"] = "none"
        os.environ["OTEL_SERVICE_NAME"] = "test-plugin"
        
        self.server = server.PluginServer()
        self.memory_exporter = InMemorySpanExporter()
        
        # Add memory exporter to the provider initialized by PluginServer
        provider = trace.get_tracer_provider()
        if isinstance(provider, TracerProvider):
            provider.add_span_processor(SimpleSpanProcessor(self.memory_exporter))
        
        # Start server in background
        self.server._server.add_insecure_port('localhost:50051')
        self.server._server.start()
        
        self.channel = grpc.insecure_channel('localhost:50051')
        self.stub = plugin_pb2_grpc.PluginServiceStub(self.channel)

    def tearDown(self):
        self.server.stop_server()
        self.channel.close()
        # Clean up env
        del os.environ["OTEL_TRACES_EXPORTER"]
        del os.environ["OTEL_SERVICE_NAME"]

    def test_health_check_creates_span(self):
        # Register a dummy handler to avoid errors (though HealthCheck doesn't use it)
        self.server.register_handler(lambda ctx, req: plugin_pb2.ResponseContext(status_code=200))
        
        response = self.stub.HealthCheck(plugin_pb2.Empty())
        self.assertEqual(response.status, plugin_pb2.HealthStatus.SERVING)
        
        # Wait for span to be processed
        time.sleep(0.5)
        
        spans = self.memory_exporter.get_finished_spans()
        self.assertTrue(len(spans) > 0, "No spans captured")
        
        span = spans[0]
        self.assertEqual(span.name, "/plugin.PluginService/HealthCheck")
        self.assertEqual(span.attributes.get("rpc.method"), "HealthCheck")
        self.assertEqual(span.attributes.get("rpc.service"), "plugin.PluginService")

    def test_context_propagation(self):
        # Create a remote span context
        trace_id = 0x12345678123456781234567812345678
        span_id = 0x1234567812345678
        
        # Inject context into metadata
        metadata = [
            ("traceparent", f"00-{trace_id:032x}-{span_id:016x}-01")
        ]
        
        # Send request with metadata
        self.server.register_handler(lambda ctx, req: plugin_pb2.ResponseContext(status_code=200))
        self.stub.HealthCheck(plugin_pb2.Empty(), metadata=metadata)
        
        time.sleep(0.5)
        
        spans = self.memory_exporter.get_finished_spans()
        self.assertTrue(len(spans) > 0)
        
        span = spans[0]
        self.assertEqual(span.context.trace_id, trace_id)

    def test_init_tracing(self):
        # Test that Init call is traced
        request = plugin_pb2.InitRequest(config={"key": "value"})
        response = self.stub.Init(request)
        self.assertTrue(response.success)
        
        time.sleep(0.5)
        
        spans = self.memory_exporter.get_finished_spans()
        self.assertTrue(len(spans) > 0)
        
        span = spans[0]
        self.assertEqual(span.name, "/plugin.PluginService/Init")
        self.assertEqual(span.attributes.get("rpc.method"), "Init")

    def test_handler_error_tracing(self):
        # Test that handler error is traced correctly
        def faulty_handler(ctx, req):
            raise RuntimeError("Something went wrong")
            
        self.server.register_handler(faulty_handler)
        
        # Expecting a valid response with error message, not an RpcError
        response = self.stub.HandleRequest(plugin_pb2.RequestContext(method="GET", path="/"))
        self.assertEqual(response.status_code, 500)
        self.assertIn("Something went wrong", response.error_message)
        
        time.sleep(0.5)
        
        spans = self.memory_exporter.get_finished_spans()
        self.assertTrue(len(spans) > 0)
        
        span = spans[0]
        self.assertEqual(span.name, "/plugin.PluginService/HandleRequest")

if __name__ == '__main__':
    unittest.main()
