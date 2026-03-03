#!/bin/bash
set -e

# Ensure we are in project root
cd "$(dirname "$0")/.."

cleanup() {
    echo "Stopping plugin..."
    if [ ! -z "$PLUGIN_PID" ]; then
        kill $PLUGIN_PID || true
    fi
    rm -f plugin_js.log
}
trap cleanup EXIT

# Ensure dependencies are installed
echo "Installing Node.js SDK dependencies..."
cd sdk/js
npm install
cd ../..

# Set environment variables for OTel
export OTEL_TRACES_EXPORTER=console
export OTEL_SERVICE_NAME=js-hello-service
export OTEL_DIAG_LEVEL=DEBUG

# Start the Node.js plugin
echo "Launching plugin..."
node examples/js-hello/index.js > plugin_js.log 2>&1 &
PLUGIN_PID=$!
echo "Plugin started with PID $PLUGIN_PID"

# Wait for plugin to start and extract port
echo "Waiting for plugin to initialize..."
MAX_RETRIES=30
PORT=""
for ((i=1; i<=MAX_RETRIES; i++)); do
    if grep -q "PLUGIN_ADDR" plugin_js.log; then
        # Format: |PLUGIN_ADDR|127.0.0.1:PORT|
        PORT=$(grep "PLUGIN_ADDR" plugin_js.log | awk -F'[:|]' '{print $4}')
        break
    fi
    sleep 1
done

if [ -z "$PORT" ]; then
    echo "Timeout waiting for plugin to start."
    cat plugin_js.log
    kill $PLUGIN_PID
    exit 1
fi

echo "Plugin listening on port $PORT"

# Send a gRPC request using the Python script (reusing the one we created)
# We need to make sure the Python environment is ready, or use grpcurl if available.
# Since we have the python script, let's use it.
echo "Sending HealthCheck request..."
# python3 scripts/send_grpc_request.py $PORT

# Alternatively, we can use a simple node script to send request if python is not reliable
# But let's try python first as it is already there.
if python3 scripts/send_grpc_request.py $PORT; then
    echo "Request sent successfully."
else
    echo "Failed to send request."
    kill $PLUGIN_PID
    exit 1
fi

# Allow some time for traces to be flushed
sleep 5

# Check logs for traces
echo "Checking logs for traces..."
if grep -q "traceId" plugin_js.log || grep -q "spanId" plugin_js.log; then
    echo "SUCCESS: Trace info found in logs."
    echo "Sample log output:"
    grep -m 1 "traceId" plugin_js.log
else
    echo "FAILURE: No trace info found in logs."
    cat plugin_js.log
    exit 1
fi
