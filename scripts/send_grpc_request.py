import sys
import grpc
import os
import time

# Add SDK paths
sdk_root = os.path.abspath(os.path.join(os.path.dirname(__file__), '../sdk/python'))
sys.path.append(sdk_root)

try:
    from polyshift.plugin import plugin_pb2
    from polyshift.plugin import plugin_pb2_grpc
except ImportError:
    # Fallback if protobufs not generated/found
    print("Error: Could not import protobufs")
    sys.exit(1)

if len(sys.argv) < 2:
    print("Usage: python3 send_grpc_request.py <port>")
    sys.exit(1)

port = sys.argv[1]
channel = grpc.insecure_channel(f'127.0.0.1:{port}')
stub = plugin_pb2_grpc.PluginServiceStub(channel)

try:
    # Retry a few times if server is not ready
    for i in range(5):
        try:
            response = stub.HealthCheck(plugin_pb2.Empty())
            print(f"Health Status: {response.status}")
            break
        except grpc.RpcError as e:
            time.sleep(0.5)
            if i == 4:
                raise

    # Send a HandleRequest to trigger manual instrumentation
    print("Sending HandleRequest...")
    req = plugin_pb2.RequestContext(
        method="GET",
        path="/test",
        request_id="test-req-1"
    )
    resp = stub.HandleRequest(req)
    print(f"HandleRequest Status: {resp.status_code}")

except Exception as e:
    print(f"RPC Failed: {e}")
    sys.exit(1)
