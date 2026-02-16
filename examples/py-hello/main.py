import sys
import os
import logging
import json
import grpc
from concurrent import futures

# Add SDK paths to sys.path
# 1. The root of the SDK package (sdk/python) so we can import polyshift.plugin
sdk_root = os.path.abspath(os.path.join(os.path.dirname(__file__), '../../sdk/python'))
sys.path.append(sdk_root)

# 2. The directory containing generated protobuf files so `import plugin_pb2` works
proto_dir = os.path.join(sdk_root, 'polyshift/plugin')
sys.path.append(proto_dir)

# Now we can import
try:
    import plugin_pb2
    import plugin_pb2_grpc
except ImportError as e:
    logging.error(f"Failed to import protobuf modules: {e}")
    sys.exit(1)

class PluginServer(plugin_pb2_grpc.PluginServiceServicer):
    def __init__(self):
        self.server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
        plugin_pb2_grpc.add_PluginServiceServicer_to_server(self, self.server)
        self.config = {}

    def Init(self, request, context):
        logging.info(f"Plugin initialized: {request.config}")
        self.config = request.config
        return plugin_pb2.InitResponse(success=True)

    def HandleRequest(self, request, context):
        logging.info(f"Handling request: {request.path}")
        if request.path == "/api/python":
            greeting = self.config.get("greeting", "Hello")
            msg = f"{greeting} from Python!"
        else:
            msg = f"Python Echo: {request.path}"
            
        return plugin_pb2.ResponseContext(
            status_code=200,
            headers={"Content-Type": "application/json"},
            body=json.dumps({"message": msg}).encode('utf-8')
        )

    def HealthCheck(self, request, context):
        return plugin_pb2.HealthStatus(status=plugin_pb2.HealthStatus.SERVING)

    def start(self):
        port = self.server.add_insecure_port('127.0.0.1:0')
        print(f"|PLUGIN_ADDR|127.0.0.1:{port}|", flush=True)
        self.server.start()
        self.server.wait_for_termination()

if __name__ == '__main__':
    logging.basicConfig(level=logging.INFO)
    server = PluginServer()
    server.start()
