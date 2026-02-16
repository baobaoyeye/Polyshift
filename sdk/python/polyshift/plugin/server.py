import grpc
import sys
import logging
import socket
from concurrent import futures
from . import plugin_pb2
from . import plugin_pb2_grpc

class PluginServer(plugin_pb2_grpc.PluginServiceServicer):
    def __init__(self):
        self._server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
        plugin_pb2_grpc.add_PluginServiceServicer_to_server(self, self._server)
        self._handler = None
        self._config = {}

    def register_handler(self, handler_func):
        """
        Register a handler function:
        def handler(context, request) -> ResponseContext
        """
        self._handler = handler_func
    
    def get_config(self, key):
        return self._config.get(key)

    def Init(self, request, context):
        logging.info(f"Plugin initialized with config: {request.config}")
        self._config = request.config
        return plugin_pb2.InitResponse(success=True)

    def HandleRequest(self, request, context):
        if not self._handler:
            return plugin_pb2.ResponseContext(
                status_code=500,
                error_message="Handler not registered"
            )
        try:
            return self._handler(context, request)
        except Exception as e:
            logging.error(f"Handler failed: {e}")
            return plugin_pb2.ResponseContext(
                status_code=500,
                error_message=str(e)
            )

    def HealthCheck(self, request, context):
        return plugin_pb2.HealthStatus(
            status=plugin_pb2.HealthStatus.SERVING,
            message="OK"
        )

    def Shutdown(self, request, context):
        def stop():
            self._server.stop(0)
        
        # Stop in a separate thread to allow response to return
        # But for simplicity here...
        self._server.stop(grace=1)
        return plugin_pb2.Empty()

    def start(self):
        # Bind to random port
        port = self._server.add_insecure_port('127.0.0.1:0')
        
        # Print address to stdout for Core to capture
        print(f"|PLUGIN_ADDR|127.0.0.1:{port}|", flush=True)
        
        logging.info(f"Plugin server listening on 127.0.0.1:{port}")
        self._server.start()
        self._server.wait_for_termination()
