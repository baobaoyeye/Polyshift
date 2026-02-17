import sys
import os
import logging
import json
from concurrent import futures

# Add SDK paths to sys.path
sdk_root = os.path.abspath(os.path.join(os.path.dirname(__file__), '../../sdk/python'))
sys.path.append(sdk_root)

# Add proto dir to path so generated code can import plugin_pb2
proto_dir = os.path.join(sdk_root, 'polyshift/plugin')
sys.path.append(proto_dir)

# Import SDK Server
try:
    from polyshift.plugin.server import PluginServer
    from polyshift.plugin import plugin_pb2
except ImportError as e:
    logging.error(f"Failed to import SDK modules: {e}")
    sys.exit(1)

def request_handler(context, request):
    logging.info(f"Handling request: {request.path}")
    
    # Access config via closure variable 'server_instance'
    # We need to ensure server_instance is available
    greeting = server_instance.get_config("greeting") or "Hello"
    
    if request.path == "/api/python":
        msg = f"{greeting} from Python!"
    else:
        msg = f"Python Echo: {request.path}"
        
    return plugin_pb2.ResponseContext(
        status_code=200,
        headers={"Content-Type": "application/json"},
        body=json.dumps({"message": msg}).encode('utf-8')
    )

if __name__ == '__main__':
    logging.basicConfig(level=logging.INFO)
    
    server_instance = PluginServer()
    server_instance.register_handler(request_handler)
    
    # Start server
    server_instance.start()
